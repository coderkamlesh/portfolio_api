package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/coderkamlesh/portfolio_api/internal/config"
)

// SESSender delivers mail through Amazon SES v2 (SendEmail).
//
// Credentials are resolved in this order, matching how the AWS SDK works:
//  1. AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN (all optional)
//  2. the Lambda execution role (recommended in production — leave the keys empty)
//  3. the standard AWS chain (shared config file, ECS/EC2 metadata)
type SESSender struct {
	client           *sesv2.Client
	from             string
	replyTo          []string
	configurationSet string
}

// NewSESSender builds a SES-backed sender. It loads the AWS default config,
// which fails fast if the region or credentials are unusable.
func NewSESSender(ctx context.Context, cfg *config.Config) (*SESSender, error) {
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
		return nil, fmt.Errorf("email: load AWS config: %w", err)
	}

	var clientOpts []func(*sesv2.Options)
	if cfg.SESEndpointURL != "" {
		// LocalStack / VPC endpoint override.
		clientOpts = append(clientOpts, func(o *sesv2.Options) {
			o.BaseEndpoint = aws.String(cfg.SESEndpointURL)
		})
	}

	sender := &SESSender{
		client:           sesv2.NewFromConfig(awsCfg, clientOpts...),
		from:             cfg.MailFrom(),
		configurationSet: cfg.SESConfigurationSet,
	}
	if cfg.SESReplyTo != "" {
		sender.replyTo = []string{cfg.SESReplyTo}
	}
	return sender, nil
}

// Send delivers msg through SES.
func (s *SESSender) Send(ctx context.Context, msg Message) error {
	body := &sestypes.Body{
		Text: &sestypes.Content{Data: aws.String(msg.TextBody), Charset: aws.String("UTF-8")},
	}
	if msg.HTMLBody != "" {
		body.Html = &sestypes.Content{Data: aws.String(msg.HTMLBody), Charset: aws.String("UTF-8")}
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.from),
		Destination:      &sestypes.Destination{ToAddresses: []string{msg.To}},
		ReplyToAddresses: s.replyTo,
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: aws.String(msg.Subject), Charset: aws.String("UTF-8")},
				Body:    body,
			},
		},
	}
	if s.configurationSet != "" {
		input.ConfigurationSetName = aws.String(s.configurationSet)
	}

	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return fmt.Errorf("email: ses send to %s: %w", msg.To, err)
	}
	return nil
}
