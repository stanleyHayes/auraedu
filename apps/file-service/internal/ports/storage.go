package ports

import (
	"context"
	"io"
)

// Storage persists and retrieves file bytes. Implementations MUST scope every object by tenantID.
type Storage interface {
	// Backend returns the domain storage backend key for this adapter.
	Backend() string
	// Save writes the contents of r for the given tenant and file ID. It returns the
	// storage-specific path that can later be passed to Open.
	Save(ctx context.Context, tenantID, fileID string, r io.Reader) (path string, err error)
	// Open returns a reader for the stored object at path, scoped to tenantID.
	Open(ctx context.Context, tenantID, path string) (io.ReadCloser, error)
	// Delete removes the stored object at path, scoped to tenantID.
	Delete(ctx context.Context, tenantID, path string) error
}

// SignedUploadProvider generates backend-signed upload parameters for direct
// client uploads (e.g. Cloudinary signed uploads).
type SignedUploadProvider interface {
	SignUpload(ctx context.Context, tenantID, fileID, folder, resourceType string) (SignedUpload, error)
}

// DeliveryURLProvider returns a CDN/transform URL for a stored object.
type DeliveryURLProvider interface {
	DeliveryURL(tenantID, path, resourceType, transform string) (string, error)
}

// WebhookVerifier validates incoming backend webhooks (e.g. Cloudinary notifications).
type WebhookVerifier interface {
	VerifyWebhook(timestamp int64, signature string, body []byte) bool
}

// SignedUpload contains the parameters a client needs to perform a direct
// backend-signed upload.
type SignedUpload struct {
	FileID       string
	PublicID     string
	Folder       string
	ResourceType string
	Signature    string
	Timestamp    int64
	APIKey       string
	CloudName    string
	UploadURL    string
}
