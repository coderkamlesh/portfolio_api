package storage

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "github.com/coderkamlesh/portfolio_api/internal/config"
)

// NewS3PresignerFromConfig builds a presigner from the application config.
//
// Credentials are resolved in the same order as the SES sender:
//  1. AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN (local dev)
//  2. the Lambda execution role (production — leave the keys empty)
//  3. the standard AWS chain (shared config file, ECS/EC2 metadata)
func NewS3PresignerFromConfig(ctx context.Context, cfg *appconfig.Config) (*S3Presigner, error) {
	if !cfg.UploadsEnabled() {
		return nil, fmt.Errorf("storage: S3_BUCKET is not set")
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
	}
	if cfg.AWSAccessKeyID != "" && cfg.AWSSecretAccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.AWSSessionToken,
			),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("storage: load AWS config: %w", err)
	}
	return NewS3Presigner(s3.NewFromConfig(awsCfg), cfg.S3Bucket), nil
}

type S3Presigner struct {
	presign *s3.PresignClient
	bucket  string
	// now is injectable so the returned expiry stays deterministic in tests.
	now func() time.Time
}

// NewS3Presigner builds a presigner for bucket. The caller supplies an already
// configured *s3.Client so credential resolution (env vars locally, the Lambda
// execution role in production) stays the caller's concern.
func NewS3Presigner(client *s3.Client, bucket string) *S3Presigner {
	return &S3Presigner{
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// PresignPutURL signs an upload. contentType is signed into the request, so the
// browser must send the identical header or S3 rejects the upload.
func (p *S3Presigner) PresignPutURL(ctx context.Context, key, contentType string, ttl time.Duration) (*PresignedRequest, error) {
	req, err := p.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return nil, fmt.Errorf("storage: presign put %s: %w", key, err)
	}
	return p.toPresigned(req, ttl), nil
}

// PresignGetURL signs a download.
func (p *S3Presigner) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (*PresignedRequest, error) {
	req, err := p.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return nil, fmt.Errorf("storage: presign get %s: %w", key, err)
	}
	return p.toPresigned(req, ttl), nil
}

// toPresigned flattens the SDK response into the transport-neutral shape the
// handler returns. Only content-type is forwarded: it is the single header the
// browser must echo back, and forwarding anything else would invite the admin
// panel to send a header the signature does not cover.
func (p *S3Presigner) toPresigned(req *v4.PresignedHTTPRequest, ttl time.Duration) *PresignedRequest {
	headers := make(map[string]string)
	if contentType := req.SignedHeader.Get("Content-Type"); contentType != "" {
		headers["Content-Type"] = contentType
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	return &PresignedRequest{
		URL:       req.URL,
		Method:    strings.ToUpper(method),
		Headers:   headers,
		ExpiresAt: p.now().Add(ttl),
	}
}
