// Package storage turns API requests into S3 presigned URLs. It owns every AWS
// call so the service layer stays free of SDK types and stays testable: the
// presigner is an interface, so the upload rules can be exercised without a
// network or credentials.
package storage

import (
	"context"
	"time"
)

// PresignedRequest describes a presigned request handed to the browser.
type PresignedRequest struct {
	// URL is the signed URL including the signature query parameters.
	URL string
	// Method is the HTTP method the browser must use, normally PUT or GET.
	Method string
	// Headers must be sent by the browser exactly as returned. Signing a
	// Content-Type binds it to the signature, so a mismatch is rejected by S3.
	Headers map[string]string
	// ExpiresAt is when the URL stops working. Clients should refresh before it.
	ExpiresAt time.Time
}

// Presigner mints presigned URLs for object writes and reads.
type Presigner interface {
	// PresignPutURL signs an upload of contentType to key.
	PresignPutURL(ctx context.Context, key, contentType string, ttl time.Duration) (*PresignedRequest, error)
	// PresignGetURL signs a download of key.
	PresignGetURL(ctx context.Context, key string, ttl time.Duration) (*PresignedRequest, error)
}
