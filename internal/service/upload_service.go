package service

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/storage"
)

// Upload kinds. Each kind gets its own folder under the upload prefix and its
// own size ceiling, so an avatar can never consume the resume allowance.
const (
	UploadKindAvatar        = "avatar"
	UploadKindProjectImage  = "project_image"
	UploadKindCompanyLogo   = "company_logo"
	UploadKindResume        = "resume"
)

// imageContentTypes are the raster formats accepted for image kinds. SVG is
// deliberately absent: an SVG can carry script, so serving one from the bucket
// would be a stored-XSS vector against the public site.
var imageContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// resumeContentTypes are the formats accepted for the resume kind.
var resumeContentTypes = map[string]string{
	"application/pdf": ".pdf",
}

// UploadDeps wires the upload service. Presigner is the only collaborator, so
// the validation rules can be tested without AWS.
type UploadDeps struct {
	Presigner      storage.Presigner
	Bucket         string
	UploadPrefix   string
	PutPresignTTL  time.Duration
	GetPresignTTL  time.Duration
	MaxImageBytes  int64
	MaxResumeBytes int64
	// Now is injectable so ExpiresAt stays deterministic in tests.
	Now func() time.Time
}

// UploadService validates upload requests and mints the presigned URLs.
type UploadService struct {
	presigner      storage.Presigner
	bucket         string
	uploadPrefix   string
	putPresignTTL  time.Duration
	getPresignTTL  time.Duration
	maxImageBytes  int64
	maxResumeBytes int64
	now            func() time.Time
}

// NewUploadService builds the service.
func NewUploadService(deps UploadDeps) *UploadService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	prefix := strings.Trim(strings.TrimSpace(deps.UploadPrefix), "/")
	if prefix == "" {
		prefix = "uploads"
	}
	return &UploadService{
		presigner:      deps.Presigner,
		bucket:         deps.Bucket,
		uploadPrefix:   prefix,
		putPresignTTL:  deps.PutPresignTTL,
		getPresignTTL:  deps.GetPresignTTL,
		maxImageBytes:  deps.MaxImageBytes,
		maxResumeBytes: deps.MaxResumeBytes,
		now:            now,
	}
}

// UploadRequest is the admin payload asking for an upload URL.
type UploadRequest struct {
	Kind        string
	Filename    string
	ContentType string
	SizeBytes   int64
}

// PresignedUploadView is what the admin panel needs to perform the upload and
// to reference the object afterwards.
type PresignedUploadView struct {
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Key       string            `json:"key"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// PresignedDownloadView is the read counterpart for a private bucket.
type PresignedDownloadView struct {
	DownloadURL string            `json:"download_url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Key         string            `json:"key"`
	ExpiresAt   time.Time         `json:"expires_at"`
}

// PresignUpload validates the request, mints a key and signs an upload URL.
func (s *UploadService) PresignUpload(ctx context.Context, in UploadRequest) (*PresignedUploadView, error) {
	if s.presigner == nil {
		return nil, errUploadsDisabled()
	}

	kind, contentType, extension, err := s.validateUpload(in)
	if err != nil {
		return nil, err
	}

	// The uploaded filename is deliberately discarded. Keys are generated from a
	// UUID plus an extension derived from the validated content type, so a
	// crafted filename can neither traverse out of the prefix nor overwrite an
	// existing object.
	key := fmt.Sprintf("%s/%s/%s%s", s.uploadPrefix, kind, ids.New(), extension)

	signed, err := s.presigner.PresignPutURL(ctx, key, contentType, s.putPresignTTL)
	if err != nil {
		return nil, wrapErr(err, 502, "upload_unavailable", "Could not prepare the upload URL. Please try again.")
	}
	return &PresignedUploadView{
		UploadURL: signed.URL,
		Method:    signed.Method,
		Headers:   signed.Headers,
		Key:       key,
		ExpiresAt: signed.ExpiresAt,
	}, nil
}

// PresignDownload signs a read URL for an object already in the bucket. The key
// must sit inside the upload prefix: without that check the endpoint would mint
// signed URLs for any object in the bucket.
func (s *UploadService) PresignDownload(ctx context.Context, key string) (*PresignedDownloadView, error) {
	if s.presigner == nil {
		return nil, errUploadsDisabled()
	}

	cleaned, err := s.validateStoredKey(key)
	if err != nil {
		return nil, err
	}

	signed, err := s.presigner.PresignGetURL(ctx, cleaned, s.getPresignTTL)
	if err != nil {
		return nil, wrapErr(err, 502, "upload_unavailable", "Could not prepare the download URL. Please try again.")
	}
	return &PresignedDownloadView{
		DownloadURL: signed.URL,
		Method:      signed.Method,
		Headers:     signed.Headers,
		Key:         cleaned,
		ExpiresAt:   signed.ExpiresAt,
	}, nil
}

// validateUpload applies the per-kind rules and returns the normalised kind, the
// content type and the extension to append to the generated key.
func (s *UploadService) validateUpload(in UploadRequest) (kind, contentType, extension string, err error) {
	kind = strings.ToLower(strings.TrimSpace(in.Kind))
	if !isUploadKind(kind) {
		return "", "", "", errUploadValidation(
			fmt.Sprintf("kind must be one of %s.", strings.Join(uploadKinds(), ", ")))
	}

	contentType = strings.ToLower(strings.TrimSpace(in.ContentType))
	allowed, maxBytes := s.limitsFor(kind)
	extension, ok := allowed[contentType]
	if !ok {
		return "", "", "", errUploadValidation(
			fmt.Sprintf("content_type must be one of %s for kind %q.", sortedContentTypes(allowed), kind))
	}

	if in.SizeBytes <= 0 {
		return "", "", "", errUploadValidation("size_bytes must be greater than zero.")
	}
	if in.SizeBytes > maxBytes {
		return "", "", "", errUploadValidation(
			fmt.Sprintf("size_bytes must be at most %d for kind %q.", maxBytes, kind))
	}

	// The filename is never used to build the key, but a mismatching extension
	// means the admin panel is about to tell the user one thing and upload
	// another, so it is surfaced instead of being silently ignored.
	if given := path.Ext(strings.TrimSpace(in.Filename)); given != "" && !strings.EqualFold(given, extension) {
		return "", "", "", errUploadValidation(
			fmt.Sprintf("filename extension %s does not match content_type %q.", given, contentType))
	}

	return kind, contentType, extension, nil
}

// limitsFor returns the accepted content types and the size ceiling of a kind.
func (s *UploadService) limitsFor(kind string) (map[string]string, int64) {
	if kind == UploadKindResume {
		return resumeContentTypes, s.maxResumeBytes
	}
	return imageContentTypes, s.maxImageBytes
}

// validateStoredKey keeps a download request inside the upload prefix.
//
// Nothing is normalised here on purpose. A key that arrives with a dot-dot
// segment, a doubled slash or stray whitespace is a caller bug, and signing a
// "cleaned up" version would mean the key we authorise is not the key the caller
// asked for. Rejecting instead keeps that invariant obvious.
func (s *UploadService) validateStoredKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errUploadRequired("key")
	}
	// Checked before any trimming so a trailing newline cannot be normalised away.
	if strings.ContainsAny(key, " \t\r\n") {
		return "", errUploadValidation("key must not contain whitespace.")
	}

	// path.Clean collapses ".." and duplicate slashes; any difference means the
	// caller sent an unclean key, so it is rejected rather than resolved.
	if cleaned := strings.TrimPrefix(path.Clean(key), "/"); cleaned != key {
		return "", errUploadValidation("key must be a clean object key.")
	}
	if !strings.HasPrefix(key, s.uploadPrefix+"/") {
		return "", errUploadValidation(fmt.Sprintf("key must start with %q.", s.uploadPrefix+"/"))
	}
	return key, nil
}

func uploadKinds() []string {
	return []string{
		UploadKindAvatar,
		UploadKindProjectImage,
		UploadKindCompanyLogo,
		UploadKindResume,
	}
}

func isUploadKind(kind string) bool {
	for _, candidate := range uploadKinds() {
		if kind == candidate {
			return true
		}
	}
	return false
}

func sortedContentTypes(allowed map[string]string) string {
	values := make([]string, 0, len(allowed))
	for contentType := range allowed {
		values = append(values, contentType)
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}
