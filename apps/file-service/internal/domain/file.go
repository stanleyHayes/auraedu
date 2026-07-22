// Package domain holds the file-service aggregate roots and value objects.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FileStatus enumerates the lifecycle states of a file upload.
type FileStatus string

const (
	StatusPending  FileStatus = "pending"
	StatusActive   FileStatus = "active"
	StatusArchived FileStatus = "archived"
	StatusDeleted  FileStatus = "deleted"
)

// StorageBackend enumerates supported storage backends.
type StorageBackend string

const (
	BackendLocal      StorageBackend = "local"
	BackendCloudinary StorageBackend = "cloudinary"
)

// FileUpload is the aggregate root of the file service. Every record is tenant-scoped.
type FileUpload struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	OriginalFilename string         `json:"original_filename"`
	StoragePath      string         `json:"storage_path"`
	StorageBackend   string         `json:"storage_backend"`
	ContentType      string         `json:"content_type"`
	SizeBytes        int64          `json:"size_bytes"`
	Checksum         string         `json:"checksum"`
	OwnerID          string         `json:"owner_id"`
	Purpose          string         `json:"purpose"`
	Status           string         `json:"status"`
	SecureURL        string         `json:"secure_url,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// NewFileUpload constructs a FileUpload, enforcing invariants.
func NewFileUpload(tenantID, originalFilename, contentType, ownerID, purpose string, sizeBytes int64, checksum string) (*FileUpload, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	originalFilename, err := SanitizeFilename(originalFilename)
	if err != nil {
		return nil, ErrValidation
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("file: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &FileUpload{
		ID:               id.String(),
		TenantID:         tenantID,
		OriginalFilename: originalFilename,
		StorageBackend:   string(BackendLocal),
		ContentType:      normalizeContentType(contentType),
		SizeBytes:        sizeBytes,
		Checksum:         checksum,
		OwnerID:          ownerID,
		Purpose:          strings.TrimSpace(purpose),
		Status:           string(StatusPending),
		Metadata:         make(map[string]any),
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// Validate checks that the file upload aggregate is well-formed.
func (f FileUpload) Validate() error {
	if f.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(f.OriginalFilename) == "" {
		return ErrValidation
	}
	cleaned, err := SanitizeFilename(f.OriginalFilename)
	if err != nil || cleaned != f.OriginalFilename {
		return ErrValidation
	}
	if f.SizeBytes < 0 {
		return ErrValidation
	}
	if !isValidStatus(f.Status) {
		return ErrValidation
	}
	if !isValidBackend(f.StorageBackend) {
		return ErrValidation
	}
	return nil
}

// ApplyUpdate mutates the file upload with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (f *FileUpload) ApplyUpdate(originalFilename, contentType, purpose, status *string, metadata map[string]any) ([]string, error) {
	var changed []string
	if originalFilename != nil {
		cleaned, err := SanitizeFilename(*originalFilename)
		if err != nil {
			return nil, ErrValidation
		}
		f.OriginalFilename = cleaned
		changed = append(changed, "original_filename")
	}
	if contentType != nil {
		f.ContentType = normalizeContentType(*contentType)
		changed = append(changed, "content_type")
	}
	if purpose != nil {
		f.Purpose = strings.TrimSpace(*purpose)
		changed = append(changed, "purpose")
	}
	if status != nil {
		if !isValidStatus(*status) {
			return nil, ErrValidation
		}
		f.Status = *status
		changed = append(changed, "status")
	}
	if len(metadata) > 0 {
		if f.Metadata == nil {
			f.Metadata = make(map[string]any)
		}
		for k, v := range metadata {
			f.Metadata[k] = v
		}
		changed = append(changed, "metadata")
	}
	if len(changed) > 0 {
		f.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// SanitizeFilename removes client-supplied path components and rejects control
// characters that could escape Content-Disposition or corrupt audit output.
func SanitizeFilename(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = filepath.Base(value)
	if value == "" || value == "." || value == ".." || len(value) > 255 {
		return "", ErrValidation
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", ErrValidation
		}
	}
	return value, nil
}

// ComputeChecksum returns the SHA-256 hex digest of the supplied bytes.
func ComputeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// LocalStoragePath returns a tenant-scoped local storage path for the file.
func (f FileUpload) LocalStoragePath(storageDir string) string {
	return filepath.Join(storageDir, f.TenantID, f.ID)
}

func isValidStatus(v string) bool {
	switch FileStatus(v) {
	case StatusPending, StatusActive, StatusArchived, StatusDeleted:
		return true
	}
	return false
}

func isValidBackend(v string) bool {
	switch StorageBackend(v) {
	case BackendLocal, BackendCloudinary:
		return true
	}
	return false
}

func normalizeContentType(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "application/octet-stream"
	}
	return v
}
