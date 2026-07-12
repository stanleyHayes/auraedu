// Package storage implements file-service storage backends.
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // SHA-1 is required for Cloudinary legacy webhook signatures
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// CloudinaryStorage persists file bytes in Cloudinary, scoped by tenantID.
type CloudinaryStorage struct {
	cld             *cloudinary.Cloudinary
	resourceType    string
	httpClient      *http.Client
	deliveryBaseURL string
}

var _ ports.Storage = (*CloudinaryStorage)(nil)

// CloudinaryOption customizes the Cloudinary adapter.
type CloudinaryOption func(*CloudinaryStorage)

// WithResourceType sets the Cloudinary resource type (e.g. "raw", "image", "auto").
// Defaults to "raw" for arbitrary file storage.
func WithResourceType(rt string) CloudinaryOption {
	return func(s *CloudinaryStorage) { s.resourceType = rt }
}

// WithHTTPClient overrides the HTTP client used for Open downloads.
func WithHTTPClient(c *http.Client) CloudinaryOption {
	return func(s *CloudinaryStorage) { s.httpClient = c }
}

// WithDeliveryBaseURL overrides the delivery (download) base URL. Used for tests.
func WithDeliveryBaseURL(base string) CloudinaryOption {
	return func(s *CloudinaryStorage) { s.deliveryBaseURL = base }
}

// NewCloudinaryStorage creates a Cloudinary-backed storage adapter from a
// Cloudinary URL (cloudinary://api_key:api_secret@cloud_name).
func NewCloudinaryStorage(cloudURL string, opts ...CloudinaryOption) (*CloudinaryStorage, error) {
	if cloudURL == "" {
		return nil, fmt.Errorf("cloudinary URL is required")
	}
	if err := validateCloudinaryURL(cloudURL); err != nil {
		return nil, err
	}
	cld, err := cloudinary.NewFromURL(cloudURL)
	if err != nil {
		return nil, fmt.Errorf("invalid cloudinary URL: %w", err)
	}
	s := &CloudinaryStorage{
		cld:             cld,
		resourceType:    "raw",
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		deliveryBaseURL: "",
	}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Backend returns the domain backend key for Cloudinary storage.
func (s *CloudinaryStorage) Backend() string { return string(domain.BackendCloudinary) }

var _ ports.SignedUploadProvider = (*CloudinaryStorage)(nil)
var _ ports.DeliveryURLProvider = (*CloudinaryStorage)(nil)
var _ ports.WebhookVerifier = (*CloudinaryStorage)(nil)

// SignUpload returns the parameters required for a direct Cloudinary signed upload.
// The caller supplies fileID; public_id in the upload parameters is set to fileID
// and the Cloudinary asset public_id will be folder/fileID.
func (s *CloudinaryStorage) SignUpload(ctx context.Context, tenantID, fileID, folder, resourceType string) (ports.SignedUpload, error) {
	_ = ctx
	if tenantID == "" || fileID == "" {
		return ports.SignedUpload{}, fmt.Errorf("tenant_id and file_id are required")
	}
	if resourceType == "" {
		resourceType = s.resourceType
	}

	timestamp := time.Now().Unix()
	params := url.Values{}
	params.Set("folder", folder)
	params.Set("public_id", fileID)
	params.Set("timestamp", strconv.FormatInt(timestamp, 10))

	signature, err := api.SignParameters(params, s.cld.Config.Cloud.APISecret)
	if err != nil {
		return ports.SignedUpload{}, fmt.Errorf("sign upload parameters: %w", err)
	}

	publicID := fileID
	if folder != "" {
		publicID = folder + "/" + fileID
	}

	uploadURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/%s/upload",
		s.cld.Config.Cloud.CloudName, resourceType)

	return ports.SignedUpload{
		FileID:       fileID,
		PublicID:     publicID,
		Folder:       folder,
		ResourceType: resourceType,
		Signature:    signature,
		Timestamp:    timestamp,
		APIKey:       s.cld.Config.Cloud.APIKey,
		CloudName:    s.cld.Config.Cloud.CloudName,
		UploadURL:    uploadURL,
	}, nil
}

// DeliveryURL returns a Cloudinary delivery URL, optionally with a transformation
// chain (e.g. "w_300,h_300,c_fit"). If resourceType is empty it defaults to the
// adapter's configured resource type.
func (s *CloudinaryStorage) DeliveryURL(_, path, resourceType, transform string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if resourceType == "" {
		resourceType = s.resourceType
	}
	base := fmt.Sprintf("https://res.cloudinary.com/%s/%s", s.cld.Config.Cloud.CloudName, resourceType)
	if transform != "" {
		return fmt.Sprintf("%s/%s/upload/%s", base, transform, path), nil
	}
	return fmt.Sprintf("%s/upload/%s", base, path), nil
}

// VerifyWebhook validates a Cloudinary notification signature.
// It accepts both SHA-1 (legacy) and SHA-256 signatures and rejects stale timestamps.
func (s *CloudinaryStorage) VerifyWebhook(timestamp int64, signature string, body []byte) bool {
	if signature == "" || timestamp == 0 {
		return false
	}
	now := time.Now().Unix()
	if timestamp < now-600 || timestamp > now+60 {
		return false
	}
	payload := strconv.FormatInt(timestamp, 10) + string(body)
	secret := s.cld.Config.Cloud.APISecret
	for _, fn := range []func() hash.Hash{sha256.New, sha1.New} {
		mac := hmac.New(fn, []byte(secret))
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(signature))) {
			return true
		}
	}
	return false
}

// Save uploads the contents of r to Cloudinary under a tenant-scoped public_id.
// It returns the public_id (used as the storage path for Open/Delete).
func (s *CloudinaryStorage) Save(ctx context.Context, tenantID, fileID string, r io.Reader) (string, error) {
	if tenantID == "" || fileID == "" {
		return "", fmt.Errorf("tenant_id and file_id are required")
	}
	publicID := fmt.Sprintf("%s/%s", tenantID, fileID)
	resp, err := s.cld.Upload.Upload(ctx, r, uploader.UploadParams{
		PublicID:     publicID,
		ResourceType: s.resourceType,
		Overwrite:    boolPtr(true),
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary upload failed: %w", err)
	}
	if resp.Error.Message != "" {
		return "", fmt.Errorf("cloudinary upload error: %s", resp.Error.Message)
	}
	if resp.PublicID == "" {
		return publicID, nil
	}
	return resp.PublicID, nil
}

// Open returns a reader for the stored Cloudinary object at publicID path.
func (s *CloudinaryStorage) Open(ctx context.Context, tenantID, path string) (io.ReadCloser, error) {
	_ = ctx
	_ = tenantID
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	url := s.deliveryURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudinary download failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return nil, fmt.Errorf("cloudinary download failed: %s (close: %w)", resp.Status, err)
		}
		return nil, fmt.Errorf("cloudinary download failed: %s", resp.Status)
	}
	return resp.Body, nil
}

// Delete removes the stored Cloudinary object at publicID path.
func (s *CloudinaryStorage) Delete(ctx context.Context, tenantID, path string) error {
	_ = tenantID
	if path == "" {
		return nil
	}
	_, err := s.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     path,
		ResourceType: s.resourceType,
	})
	if err != nil {
		return fmt.Errorf("cloudinary destroy failed: %w", err)
	}
	return nil
}

func (s *CloudinaryStorage) deliveryURL(path string) string {
	if s.deliveryBaseURL != "" {
		return s.deliveryBaseURL + "/" + path
	}
	return fmt.Sprintf("https://res.cloudinary.com/%s/%s/upload/%s",
		s.cld.Config.Cloud.CloudName, s.resourceType, path)
}

func boolPtr(v bool) *bool { return &v }

func validateCloudinaryURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid cloudinary URL: %w", err)
	}
	if u.Scheme != "cloudinary" {
		return fmt.Errorf("invalid cloudinary URL: scheme must be cloudinary")
	}
	if u.Host == "" {
		return fmt.Errorf("invalid cloudinary URL: cloud_name is required")
	}
	if u.User == nil || u.User.Username() == "" {
		return fmt.Errorf("invalid cloudinary URL: api_key is required")
	}
	if _, ok := u.User.Password(); !ok {
		return fmt.Errorf("invalid cloudinary URL: api_secret is required")
	}
	return nil
}
