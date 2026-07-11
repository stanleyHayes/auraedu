// Package application implements the file-service use cases.
package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys (must match contracts/permissions/permissions.yaml).
const (
	PermRead   = "files.read"
	PermCreate = "files.upload"
	PermUpdate = "files.update"
	PermDelete = "files.delete"
)

// FeatureFileManagement is the feature flag key for file management.
const FeatureFileManagement = "file_management"

// Service holds the file use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo             ports.Repository
	storage          ports.Storage
	signer           ports.SignedUploadProvider
	deliveryProvider ports.DeliveryURLProvider
	webhookVerifier  ports.WebhookVerifier
	pub              ports.EventPublisher
	gates            flags.Gate
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithSignedUploadProvider sets the provider used to sign direct uploads.
func WithSignedUploadProvider(p ports.SignedUploadProvider) Option {
	return func(s *Service) { s.signer = p }
}

// WithDeliveryURLProvider sets the provider used to generate CDN/transform URLs.
func WithDeliveryURLProvider(p ports.DeliveryURLProvider) Option {
	return func(s *Service) { s.deliveryProvider = p }
}

// WithWebhookVerifier sets the provider used to verify incoming backend webhooks.
func WithWebhookVerifier(v ports.WebhookVerifier) Option {
	return func(s *Service) { s.webhookVerifier = v }
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, string, *domain.FileUpload, map[string]any) error {
	return nil
}

// NewService constructs the application service.
func NewService(repo ports.Repository, storage ports.Storage, opts ...Option) *Service {
	s := &Service{repo: repo, storage: storage, pub: noopPublisher{}, gates: flags.NewStaticSnapshot()}
	for _, o := range opts {
		o(s)
	}
	return s
}

// CreateFileRequest is the input for uploading a file.
type CreateFileRequest struct {
	OriginalFilename string
	ContentType      string
	OwnerID          string
	Purpose          string
	Data             []byte
	Metadata         map[string]any
}

// UpdateFileRequest is the input for patching a file record.
type UpdateFileRequest struct {
	OriginalFilename *string
	ContentType      *string
	Purpose          *string
	Status           *string
	Metadata         map[string]any
}

// SignedUploadRequest is the input for requesting a signed direct upload.
type SignedUploadRequest struct {
	Folder       string
	FileName     string
	ResourceType string
}

// SignedUploadResponse is the payload returned to the client for a signed upload.
type SignedUploadResponse struct {
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
	APIKey    string `json:"api_key"`
	Folder    string `json:"folder"`
	CloudName string `json:"cloud_name"`
	UploadURL string `json:"upload_url"`
	PublicID  string `json:"public_id"`
	FileID    string `json:"file_id"`
}

// CompleteSignedUploadRequest is the input for finalizing a signed upload.
type CompleteSignedUploadRequest struct {
	SecureURL   string
	PublicID    string
	SizeBytes   int64
	ContentType string
}

// Create validates, stores, and persists a new FileUpload for the actor's tenant.
func (s *Service) Create(ctx context.Context, actor auth.Actor, req CreateFileRequest) (*domain.FileUpload, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	ownerID := req.OwnerID
	if ownerID == "" {
		ownerID = actor.UserID
	}
	checksum := domain.ComputeChecksum(req.Data)
	file, err := domain.NewFileUpload(tenantID, req.OriginalFilename, req.ContentType, ownerID, req.Purpose, int64(len(req.Data)), checksum)
	if err != nil {
		return nil, err
	}
	if len(req.Metadata) > 0 {
		file.Metadata = req.Metadata
	}
	path, err := s.storage.Save(ctx, tenantID, file.ID, bytes.NewReader(req.Data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrStorage, err)
	}
	file.StoragePath = path
	file.StorageBackend = s.storage.Backend()
	file.Status = string(domain.StatusActive)
	if err := file.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, file); err != nil {
		return nil, err
	}
	if err := s.repo.RecordStorage(ctx, tenantID, int64(len(req.Data))); err != nil {
		slog.Default().ErrorContext(ctx, "failed to record storage", "tenant_id", tenantID, "bytes", len(req.Data), "err", err)
	}
	if err := s.pub.Publish(ctx, "file.uploaded.v1", file, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish event", "event_type", "file.uploaded.v1", "err", err)
	}
	return file, nil
}

// List returns a tenant-scoped page of file uploads.
func (s *Service) List(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.FileUpload, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.repo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// Get returns a single file upload if the actor may read the tenant's data.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*domain.FileUpload, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// Update patches a file record.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, req UpdateFileRequest) (*domain.FileUpload, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	file, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := file.ApplyUpdate(req.OriginalFilename, req.ContentType, req.Purpose, req.Status, req.Metadata)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return file, nil
	}
	if err := file.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tenantID, file); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "file.updated.v1", file, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish event", "event_type", "file.updated.v1", "err", err)
	}
	return file, nil
}

// Delete removes a file record and its stored bytes.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermDelete)
	if err != nil {
		return err
	}
	file, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.storage.Delete(ctx, tenantID, file.StoragePath); err != nil {
		return fmt.Errorf("%w: %w", domain.ErrStorage, err)
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "file.deleted.v1", file, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish event", "event_type", "file.deleted.v1", "err", err)
	}
	return nil
}

// RequestSignedUpload creates a pending file record and returns signed upload parameters.
func (s *Service) RequestSignedUpload(ctx context.Context, actor auth.Actor, req SignedUploadRequest) (*SignedUploadResponse, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Folder) == "" || strings.TrimSpace(req.FileName) == "" {
		return nil, domain.ErrValidation
	}
	if req.ResourceType == "" {
		req.ResourceType = "raw"
	}

	file, err := domain.NewFileUpload(tenantID, req.FileName, "", actor.UserID, "", 0, "")
	if err != nil {
		return nil, err
	}
	file.StoragePath = req.Folder + "/" + file.ID
	file.StorageBackend = string(domain.BackendCloudinary)
	file.Status = string(domain.StatusPending)

	if s.signer == nil {
		return nil, fmt.Errorf("%w: signed upload provider not configured", domain.ErrStorage)
	}
	signed, err := s.signer.SignUpload(ctx, tenantID, file.ID, req.Folder, req.ResourceType)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrStorage, err)
	}

	if err := s.repo.Create(ctx, tenantID, file); err != nil {
		return nil, err
	}

	return &SignedUploadResponse{
		Signature: signed.Signature,
		Timestamp: signed.Timestamp,
		APIKey:    signed.APIKey,
		Folder:    signed.Folder,
		CloudName: signed.CloudName,
		UploadURL: signed.UploadURL,
		PublicID:  signed.PublicID,
		FileID:    signed.FileID,
	}, nil
}

// CompleteSignedUpload finalizes a pending signed upload after the client has
// uploaded the asset directly to the storage backend.
func (s *Service) CompleteSignedUpload(ctx context.Context, actor auth.Actor, fileID string, req CompleteSignedUploadRequest) (*domain.FileUpload, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	if fileID == "" || strings.TrimSpace(req.PublicID) == "" || strings.TrimSpace(req.SecureURL) == "" || req.SizeBytes < 0 {
		return nil, domain.ErrValidation
	}

	file, err := s.repo.GetByID(ctx, tenantID, fileID)
	if err != nil {
		return nil, err
	}
	if file.Status != string(domain.StatusPending) || file.StorageBackend != string(domain.BackendCloudinary) {
		return nil, domain.ErrNotFound
	}
	if file.StoragePath != req.PublicID {
		return nil, domain.ErrValidation
	}

	file.Status = string(domain.StatusActive)
	file.SizeBytes = req.SizeBytes
	if req.ContentType != "" {
		file.ContentType = req.ContentType
	}
	file.SecureURL = req.SecureURL
	if file.Metadata == nil {
		file.Metadata = make(map[string]any)
	}
	file.Metadata["secure_url"] = req.SecureURL
	file.UpdatedAt = time.Now().UTC()

	if err := file.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tenantID, file); err != nil {
		return nil, err
	}
	if err := s.repo.RecordStorage(ctx, tenantID, req.SizeBytes); err != nil {
		slog.Default().ErrorContext(ctx, "failed to record storage", "tenant_id", tenantID, "bytes", req.SizeBytes, "err", err)
	}
	if err := s.pub.Publish(ctx, "file.uploaded.v1", file, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish event", "event_type", "file.uploaded.v1", "err", err)
	}
	return file, nil
}

// ProcessCloudinaryWebhook handles a verified Cloudinary upload/moderation notification.
func (s *Service) ProcessCloudinaryWebhook(ctx context.Context, timestamp int64, signature string, body []byte) error {
	if s.webhookVerifier == nil {
		return fmt.Errorf("%w: webhook verifier not configured", domain.ErrStorage)
	}
	if !s.webhookVerifier.VerifyWebhook(timestamp, signature, body) {
		return domain.ErrForbidden
	}

	payload, err := parseWebhookPayload(body)
	if err != nil {
		return err
	}

	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{
		TenantID:  payload.TenantID,
		RequestID: tenancy.RequestID(ctx),
	})

	file, err := s.repo.GetByID(ctx, payload.TenantID, payload.FileID)
	if err != nil {
		return err
	}

	changed := applyWebhookUpdate(file, payload)
	if !changed {
		return nil
	}
	file.UpdatedAt = time.Now().UTC()
	if err := file.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, payload.TenantID, file); err != nil {
		return err
	}
	if err := s.repo.RecordStorage(ctx, payload.TenantID, payload.Bytes); err != nil {
		slog.Default().ErrorContext(ctx, "failed to record storage", "tenant_id", payload.TenantID, "bytes", payload.Bytes, "err", err)
	}
	if err := s.pub.Publish(ctx, "file.uploaded.v1", file, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish event", "event_type", "file.uploaded.v1", "err", err)
	}
	return nil
}

type webhookPayload struct {
	NotificationType string `json:"notification_type"`
	PublicID         string `json:"public_id"`
	SecureURL        string `json:"secure_url"`
	Bytes            int64  `json:"bytes"`
	ResourceType     string `json:"resource_type"`
	ModerationStatus string `json:"moderation_status"`
	TenantID         string
	FileID           string
}

func parseWebhookPayload(body []byte) (webhookPayload, error) {
	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return webhookPayload{}, domain.ErrValidation
	}
	if payload.PublicID == "" {
		return webhookPayload{}, domain.ErrValidation
	}
	parts := strings.Split(payload.PublicID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return webhookPayload{}, domain.ErrValidation
	}
	payload.TenantID = parts[0]
	payload.FileID = parts[1]
	return payload, nil
}

func applyWebhookUpdate(file *domain.FileUpload, payload webhookPayload) bool {
	changed := false
	if payload.SecureURL != "" && file.SecureURL != payload.SecureURL {
		file.SecureURL = payload.SecureURL
		changed = true
	}
	if payload.Bytes > 0 && file.SizeBytes != payload.Bytes {
		file.SizeBytes = payload.Bytes
		changed = true
	}
	if payload.ResourceType != "" {
		if file.Metadata == nil {
			file.Metadata = make(map[string]any)
		}
		if file.Metadata["resource_type"] != payload.ResourceType {
			file.Metadata["resource_type"] = payload.ResourceType
			changed = true
		}
	}
	if payload.ModerationStatus != "" {
		if file.Metadata == nil {
			file.Metadata = make(map[string]any)
		}
		if file.Metadata["moderation_status"] != payload.ModerationStatus {
			file.Metadata["moderation_status"] = payload.ModerationStatus
			changed = true
		}
	}
	if file.Status != string(domain.StatusActive) {
		file.Status = string(domain.StatusActive)
		changed = true
	}
	return changed
}

// GetDeliveryURL returns a CDN/transform URL for a Cloudinary-backed file.
func (s *Service) GetDeliveryURL(ctx context.Context, actor auth.Actor, fileID, preset, transform string) (string, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return "", "", err
	}
	if fileID == "" {
		return "", "", domain.ErrValidation
	}
	file, err := s.repo.GetByID(ctx, tenantID, fileID)
	if err != nil {
		return "", "", err
	}
	if file.StorageBackend != string(domain.BackendCloudinary) {
		return "", "", fmt.Errorf("%w: delivery URLs are only available for Cloudinary-backed files", domain.ErrStorage)
	}
	if s.deliveryProvider == nil {
		return "", "", fmt.Errorf("%w: delivery URL provider not configured", domain.ErrStorage)
	}
	if transform == "" {
		transform = transformForPreset(preset)
	}
	url, err := s.deliveryProvider.DeliveryURL(tenantID, file.StoragePath, "", transform)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", domain.ErrStorage, err)
	}
	if err := s.repo.RecordDelivery(ctx, tenantID, file.SizeBytes); err != nil {
		slog.Default().ErrorContext(ctx, "failed to record delivery", "tenant_id", tenantID, "bytes", file.SizeBytes, "err", err)
	}
	return url, transform, nil
}

func transformForPreset(preset string) string {
	switch preset {
	case "thumbnail":
		return "w_150,h_150,c_fill"
	case "avatar":
		return "w_200,h_200,c_fill,g_face"
	case "banner":
		return "w_1200,h_400,c_fit"
	case "", "original":
		return ""
	default:
		return ""
	}
}

// Download returns a reader for the stored file bytes.
func (s *Service) Download(ctx context.Context, actor auth.Actor, id string) (*domain.FileUpload, io.ReadCloser, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, nil, err
	}
	file, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	rc, err := s.storage.Open(ctx, tenantID, file.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", domain.ErrStorage, err)
	}
	if err := s.repo.RecordDelivery(ctx, tenantID, file.SizeBytes); err != nil {
		slog.Default().ErrorContext(ctx, "failed to record delivery", "tenant_id", tenantID, "bytes", file.SizeBytes, "err", err)
	}
	return file, rc, nil
}

// GetUsage returns per-day usage records for the actor's tenant.
func (s *Service) GetUsage(ctx context.Context, actor auth.Actor, limit int) ([]*ports.UsageRecord, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetUsage(ctx, tenantID, normalizeLimit(limit))
}

func (s *Service) requireAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	if !actor.Authenticated() {
		return "", domain.ErrForbidden
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if !actor.CanAccessTenant(tenantID) {
		return "", domain.ErrForbidden
	}
	if !actor.Has(perm) {
		return "", domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureFileManagement) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureFileManagement)
	}
	return tenantID, nil
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
