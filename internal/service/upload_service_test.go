package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/storage"
)

// uploadTestNow is the fixed clock every upload test runs on.
var uploadTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

const (
	uploadTestPrefix  = "uploads"
	uploadTestMaxImage  = 5 << 20
	uploadTestMaxResume = 10 << 20
)

// fakePresigner records the calls it receives and returns a fixed signed
// request, so the upload rules can be checked without AWS.
type fakePresigner struct {
	putErr    error
	getErr    error
	putKeys   []string
	putTypes  []string
	putTTLs   []time.Duration
	getKeys   []string
}

func (p *fakePresigner) PresignPutURL(_ context.Context, key, contentType string, ttl time.Duration) (*storage.PresignedRequest, error) {
	if p.putErr != nil {
		return nil, p.putErr
	}
	p.putKeys = append(p.putKeys, key)
	p.putTypes = append(p.putTypes, contentType)
	p.putTTLs = append(p.putTTLs, ttl)
	return &storage.PresignedRequest{
		URL:       "https://bucket.s3.ap-south-1.amazonaws.com/" + key + "?X-Amz-Signature=fake",
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: uploadTestNow.Add(ttl),
	}, nil
}

func (p *fakePresigner) PresignGetURL(_ context.Context, key string, ttl time.Duration) (*storage.PresignedRequest, error) {
	if p.getErr != nil {
		return nil, p.getErr
	}
	p.getKeys = append(p.getKeys, key)
	return &storage.PresignedRequest{
		URL:       "https://bucket.s3.ap-south-1.amazonaws.com/" + key + "?X-Amz-Signature=fake",
		Method:    "GET",
		Headers:   map[string]string{},
		ExpiresAt: uploadTestNow.Add(ttl),
	}, nil
}

func newUploadTestService(presigner storage.Presigner) *UploadService {
	return NewUploadService(UploadDeps{
		Presigner:      presigner,
		Bucket:         "portfoliov2-289210138514-ap-south-1-an",
		UploadPrefix:   uploadTestPrefix,
		PutPresignTTL:  15 * time.Minute,
		GetPresignTTL:  time.Hour,
		MaxImageBytes:  uploadTestMaxImage,
		MaxResumeBytes: uploadTestMaxResume,
		Now:            func() time.Time { return uploadTestNow },
	})
}

func uploadErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

func validUploadRequest() UploadRequest {
	return UploadRequest{
		Kind:        UploadKindAvatar,
		Filename:    "profile.png",
		ContentType: "image/png",
		SizeBytes:   120_000,
	}
}

func TestPresignUploadBuildsAGeneratedKey(t *testing.T) {
	presigner := &fakePresigner{}
	svc := newUploadTestService(presigner)

	got, err := svc.PresignUpload(context.Background(), validUploadRequest())
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}

	// The key must be prefix/kind/<uuid><ext> and must not contain the filename.
	if !strings.HasPrefix(got.Key, "uploads/avatar/") {
		t.Errorf("Key = %q, want it under uploads/avatar/", got.Key)
	}
	if !strings.HasSuffix(got.Key, ".png") {
		t.Errorf("Key = %q, want a .png extension", got.Key)
	}
	if strings.Contains(got.Key, "profile") {
		t.Errorf("Key = %q, must not embed the uploaded filename", got.Key)
	}
	if got.Method != "PUT" {
		t.Errorf("Method = %q, want PUT", got.Method)
	}
	if got.Headers["Content-Type"] != "image/png" {
		t.Errorf("Headers = %+v, want Content-Type image/png", got.Headers)
	}
	if !got.ExpiresAt.Equal(uploadTestNow.Add(15 * time.Minute)) {
		t.Errorf("ExpiresAt = %v, want the fixed clock plus the PUT TTL", got.ExpiresAt)
	}
	if presigner.putTypes[0] != "image/png" {
		t.Errorf("presigner got content type %q, want image/png", presigner.putTypes[0])
	}
}

func TestPresignUploadGeneratesADistinctKeyEveryTime(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{})

	first, err := svc.PresignUpload(context.Background(), validUploadRequest())
	if err != nil {
		t.Fatalf("first PresignUpload: %v", err)
	}
	second, err := svc.PresignUpload(context.Background(), validUploadRequest())
	if err != nil {
		t.Fatalf("second PresignUpload: %v", err)
	}
	if first.Key == second.Key {
		t.Errorf("both uploads got key %q; a second upload would overwrite the first", first.Key)
	}
}

func TestPresignUploadAcceptsEveryKind(t *testing.T) {
	cases := []struct {
		kind        string
		contentType string
		extension   string
	}{
		{kind: UploadKindAvatar, contentType: "image/jpeg", extension: ".jpg"},
		{kind: UploadKindProjectImage, contentType: "image/webp", extension: ".webp"},
		{kind: UploadKindCompanyLogo, contentType: "image/png", extension: ".png"},
		{kind: UploadKindResume, contentType: "application/pdf", extension: ".pdf"},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			svc := newUploadTestService(&fakePresigner{})

			got, err := svc.PresignUpload(context.Background(), UploadRequest{
				Kind:        tc.kind,
				ContentType: tc.contentType,
				SizeBytes:   1024,
			})
			if err != nil {
				t.Fatalf("PresignUpload: %v", err)
			}
			if !strings.HasPrefix(got.Key, "uploads/"+tc.kind+"/") {
				t.Errorf("Key = %q, want it under uploads/%s/", got.Key, tc.kind)
			}
			if !strings.HasSuffix(got.Key, tc.extension) {
				t.Errorf("Key = %q, want extension %q", got.Key, tc.extension)
			}
		})
	}
}

func TestPresignUploadRejectsAnUnknownKind(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{})

	_, err := svc.PresignUpload(context.Background(), UploadRequest{
		Kind:        "video",
		ContentType: "image/png",
		SizeBytes:   1024,
	})
	if code := uploadErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; !strings.Contains(msg, "kind must be one of") {
		t.Errorf("error message = %q", msg)
	}
}

func TestPresignUploadRejectsSVG(t *testing.T) {
	presigner := &fakePresigner{}
	svc := newUploadTestService(presigner)

	// SVG can carry script, so it is never an accepted image type even though a
	// browser would happily render it.
	_, err := svc.PresignUpload(context.Background(), UploadRequest{
		Kind:        UploadKindProjectImage,
		ContentType: "image/svg+xml",
		SizeBytes:   1024,
	})
	if code := uploadErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if len(presigner.putKeys) != 0 {
		t.Error("a rejected request must not reach the presigner")
	}
}

func TestPresignUploadRejectsAContentTypeOutsideTheKind(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{})

	// A PDF is valid for the resume kind only.
	_, err := svc.PresignUpload(context.Background(), UploadRequest{
		Kind:        UploadKindAvatar,
		ContentType: "application/pdf",
		SizeBytes:   1024,
	})
	if code := uploadErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; !strings.Contains(msg, `for kind "avatar"`) {
		t.Errorf("error message = %q, want it to name the kind", msg)
	}
}

func TestPresignUploadEnforcesSizeLimits(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		ctype     string
		sizeBytes int64
		wantErr   string
	}{
		{
			name:      "zero size",
			kind:      UploadKindAvatar,
			ctype:     "image/png",
			sizeBytes: 0,
			wantErr:   "size_bytes must be greater than zero.",
		},
		{
			name:      "negative size",
			kind:      UploadKindAvatar,
			ctype:     "image/png",
			sizeBytes: -1,
			wantErr:   "size_bytes must be greater than zero.",
		},
		{
			name:      "image over the image ceiling",
			kind:      UploadKindAvatar,
			ctype:     "image/png",
			sizeBytes: uploadTestMaxImage + 1,
			wantErr:   `size_bytes must be at most 5242880 for kind "avatar".`,
		},
		{
			name:      "image at the image ceiling",
			kind:      UploadKindAvatar,
			ctype:     "image/png",
			sizeBytes: uploadTestMaxImage,
			wantErr:   "",
		},
		{
			name:      "resume gets its own larger ceiling",
			kind:      UploadKindResume,
			ctype:     "application/pdf",
			sizeBytes: uploadTestMaxResume,
			wantErr:   "",
		},
		{
			name:      "resume over the resume ceiling",
			kind:      UploadKindResume,
			ctype:     "application/pdf",
			sizeBytes: uploadTestMaxResume + 1,
			wantErr:   `size_bytes must be at most 10485760 for kind "resume".`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newUploadTestService(&fakePresigner{})

			_, err := svc.PresignUpload(context.Background(), UploadRequest{
				Kind:        tc.kind,
				ContentType: tc.ctype,
				SizeBytes:   tc.sizeBytes,
			})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if code := uploadErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestPresignUploadRejectsAFilenameExtensionMismatch(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{})

	in := validUploadRequest()
	in.Filename = "payload.exe"

	_, err := svc.PresignUpload(context.Background(), in)
	if code := uploadErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; !strings.Contains(msg, "does not match content_type") {
		t.Errorf("error message = %q", msg)
	}
}

func TestPresignUploadIgnoresANonConflictingFilename(t *testing.T) {
	cases := []struct {
		name     string
		filename string
	}{
		{name: "matching extension in a different case", filename: "PROFILE.PNG"},
		{name: "no extension at all", filename: "profile"},
		{name: "empty filename", filename: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newUploadTestService(&fakePresigner{})

			in := validUploadRequest()
			in.Filename = tc.filename

			if _, err := svc.PresignUpload(context.Background(), in); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPresignUploadIgnoresACraftedFilenameForTheKey(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{})

	// A traversal attempt in the filename must not reach the key: the key is
	// built from a generated UUID, not from the name.
	in := validUploadRequest()
	in.Filename = "../../../../etc/passwd.png"

	got, err := svc.PresignUpload(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got.Key, "..") || strings.Contains(got.Key, "passwd") {
		t.Errorf("Key = %q, must be a generated UUID path", got.Key)
	}
	if !strings.HasPrefix(got.Key, "uploads/avatar/") {
		t.Errorf("Key = %q, want it under uploads/avatar/", got.Key)
	}
}

func TestPresignUploadSurfacesAPresignerFailure(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{putErr: errUploadValidation("boom")})

	_, err := svc.PresignUpload(context.Background(), validUploadRequest())
	if code := uploadErrCode(t, err); code != "upload_unavailable" {
		t.Fatalf("got %v (code %q), want upload_unavailable", err, code)
	}
}

func TestPresignDownloadSignsAKeyInsideThePrefix(t *testing.T) {
	presigner := &fakePresigner{}
	svc := newUploadTestService(presigner)

	got, err := svc.PresignDownload(context.Background(), "uploads/avatar/abc.png")
	if err != nil {
		t.Fatalf("PresignDownload: %v", err)
	}
	if got.Key != "uploads/avatar/abc.png" {
		t.Errorf("Key = %q, want the key echoed back", got.Key)
	}
	if got.Method != "GET" {
		t.Errorf("Method = %q, want GET", got.Method)
	}
	if !got.ExpiresAt.Equal(uploadTestNow.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want the fixed clock plus the GET TTL", got.ExpiresAt)
	}
	if presigner.getKeys[0] != "uploads/avatar/abc.png" {
		t.Errorf("presigner got key %q", presigner.getKeys[0])
	}
}

func TestPresignDownloadRejectsKeysOutsideThePrefix(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{name: "empty key", key: ""},
		{name: "bucket root", key: "secrets.txt"},
		{name: "sibling prefix", key: "private/avatar/abc.png"},
		{name: "prefix without separator", key: "uploads"},
		{name: "leading slash outside the prefix", key: "/etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presigner := &fakePresigner{}
			svc := newUploadTestService(presigner)

			_, err := svc.PresignDownload(context.Background(), tc.key)
			if code := uploadErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if len(presigner.getKeys) != 0 {
				t.Error("a rejected key must not reach the presigner")
			}
		})
	}
}

func TestPresignDownloadRejectsTraversalAndUncleanKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{name: "dot dot escaping the prefix", key: "uploads/../secrets.txt"},
		{name: "deep dot dot", key: "uploads/avatar/../../../root/key"},
		{name: "internal dot dot", key: "uploads/avatar/./abc.png"},
		{name: "double slash", key: "uploads//avatar/abc.png"},
		{name: "embedded whitespace", key: "uploads/avatar/ab c.png"},
		{name: "trailing newline", key: "uploads/avatar/abc.png\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presigner := &fakePresigner{}
			svc := newUploadTestService(presigner)

			_, err := svc.PresignDownload(context.Background(), tc.key)
			if code := uploadErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if len(presigner.getKeys) != 0 {
				t.Errorf("key %q must never be signed", tc.key)
			}
		})
	}
}

func TestPresignDownloadHonoursACustomPrefix(t *testing.T) {
	presigner := &fakePresigner{}
	svc := NewUploadService(UploadDeps{
		Presigner:      presigner,
		Bucket:         "portfoliov2-289210138514-ap-south-1-an",
		UploadPrefix:   "media/assets",
		PutPresignTTL:  15 * time.Minute,
		GetPresignTTL:  time.Hour,
		MaxImageBytes:  uploadTestMaxImage,
		MaxResumeBytes: uploadTestMaxResume,
		Now:            func() time.Time { return uploadTestNow },
	})

	if _, err := svc.PresignDownload(context.Background(), "media/assets/avatar/abc.png"); err != nil {
		t.Fatalf("a key inside the custom prefix should be accepted: %v", err)
	}
	// The old default must no longer be trusted once the prefix is overridden.
	if _, err := svc.PresignDownload(context.Background(), "uploads/avatar/abc.png"); err == nil {
		t.Error("the default prefix must not be accepted after the prefix is overridden")
	}
}

func TestPresignUploadReports503WhenUploadsAreDisabled(t *testing.T) {
	// A nil presigner is how the router wires the service when no bucket is
	// configured; the rest of the API must keep working.
	svc := NewUploadService(UploadDeps{
		UploadPrefix: uploadTestPrefix,
	})

	if _, err := svc.PresignUpload(context.Background(), validUploadRequest()); err == nil {
		t.Fatal("expected an error, got nil")
	} else if code := uploadErrCode(t, err); code != "uploads_disabled" {
		t.Errorf("got code %q, want uploads_disabled", code)
	}

	if _, err := svc.PresignDownload(context.Background(), "uploads/avatar/abc.png"); err == nil {
		t.Fatal("expected an error, got nil")
	} else if code := uploadErrCode(t, err); code != "uploads_disabled" {
		t.Errorf("got code %q, want uploads_disabled", code)
	}
}

func TestPresignUploadDefaultsThePrefixWhenUnset(t *testing.T) {
	presigner := &fakePresigner{}
	svc := NewUploadService(UploadDeps{
		Presigner:      presigner,
		Bucket:         "portfoliov2-289210138514-ap-south-1-an",
		UploadPrefix:   "",
		PutPresignTTL:  15 * time.Minute,
		GetPresignTTL:  time.Hour,
		MaxImageBytes:  uploadTestMaxImage,
		MaxResumeBytes: uploadTestMaxResume,
		Now:            func() time.Time { return uploadTestNow },
	})

	got, err := svc.PresignUpload(context.Background(), validUploadRequest())
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if !strings.HasPrefix(got.Key, "uploads/") {
		t.Errorf("Key = %q, want the default uploads/ prefix", got.Key)
	}
}

func TestPresignDownloadSurfacesAPresignerFailure(t *testing.T) {
	svc := newUploadTestService(&fakePresigner{getErr: errUploadValidation("boom")})

	_, err := svc.PresignDownload(context.Background(), "uploads/avatar/abc.png")
	if code := uploadErrCode(t, err); code != "upload_unavailable" {
		t.Fatalf("got %v (code %q), want upload_unavailable", err, code)
	}
}