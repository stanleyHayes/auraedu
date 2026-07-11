package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/auraedu/file-service/internal/application"
	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// maxUploadSize caps multipart uploads at 32 MiB for this minimal implementation.
const maxUploadSize = 32 << 20

// Handler adapts HTTP to the file use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files", h.list)
	mux.HandleFunc("GET /api/v1/files/usage", h.usage)
	mux.HandleFunc("POST /api/v1/files", h.create)
	mux.HandleFunc("POST /api/v1/uploads/signed", h.requestSignedUpload)
	mux.HandleFunc("POST /api/v1/files/webhook", h.cloudinaryWebhook)
	mux.HandleFunc("GET /api/v1/files/{file_id}", h.get)
	mux.HandleFunc("PATCH /api/v1/files/{file_id}", h.update)
	mux.HandleFunc("DELETE /api/v1/files/{file_id}", h.delete)
	mux.HandleFunc("GET /api/v1/files/{file_id}/download", h.download)
	mux.HandleFunc("GET /api/v1/files/{file_id}/url", h.deliveryURL)
	mux.HandleFunc("POST /api/v1/files/{file_id}/complete", h.completeSignedUpload)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	files, nextCursor, err := h.svc.List(ctx, actor, limit, cursor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": files, "next_cursor": nullIfEmpty(nextCursor)})
}

func (h *Handler) usage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	records, err := h.svc.GetUsage(ctx, actor, limit)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		if errors.Is(err, http.ErrNotMultipart) {
			httpx.ValidationError(w, r, map[string]any{"body": "multipart/form-data required"})
			return
		}
		httpx.ValidationError(w, r, map[string]any{"body": "failed to parse multipart form"})
		return
	}
	defer r.MultipartForm.RemoveAll()

	fileHeader, fileInfo, err := r.FormFile("file")
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"file": "file part is required"})
		return
	}
	defer fileHeader.Close()

	data, err := io.ReadAll(fileHeader)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}

	metadata, _ := parseMetadata(r.FormValue("metadata"))

	uploaded, err := h.svc.Create(ctx, actor, application.CreateFileRequest{
		OriginalFilename: fileInfo.Filename,
		ContentType:      fileInfo.Header.Get("Content-Type"),
		OwnerID:          r.FormValue("owner_id"),
		Purpose:          r.FormValue("purpose"),
		Data:             data,
		Metadata:         metadata,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, uploaded)
}

type signedUploadRequestBody struct {
	Folder       string `json:"folder"`
	FileName     string `json:"file_name"`
	ResourceType string `json:"resource_type"`
}

func (h *Handler) requestSignedUpload(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body signedUploadRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	resp, err := h.svc.RequestSignedUpload(ctx, actor, application.SignedUploadRequest{
		Folder:       body.Folder,
		FileName:     body.FileName,
		ResourceType: body.ResourceType,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, resp)
}

func (h *Handler) cloudinaryWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.RespondError(w, r, httpx.Error{Code: httpx.ErrInternal, Message: "failed to read body"})
		return
	}
	timestamp, _ := strconv.ParseInt(r.Header.Get("X-Cld-Timestamp"), 10, 64)
	signature := r.Header.Get("X-Cld-Signature")

	ctx, _, _ := h.context(r)
	if err := h.svc.ProcessCloudinaryWebhook(ctx, timestamp, signature, body); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	file, err := h.svc.Get(ctx, actor, r.PathValue("file_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, file)
}

type updateBody struct {
	OriginalFilename *string        `json:"original_filename"`
	ContentType      *string        `json:"content_type"`
	Purpose          *string        `json:"purpose"`
	Status           *string        `json:"status"`
	Metadata         map[string]any `json:"metadata"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	file, err := h.svc.Update(ctx, actor, r.PathValue("file_id"), application.UpdateFileRequest{
		OriginalFilename: body.OriginalFilename,
		ContentType:      body.ContentType,
		Purpose:          body.Purpose,
		Status:           body.Status,
		Metadata:         body.Metadata,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, file)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx, actor, r.PathValue("file_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type completeSignedUploadBody struct {
	SecureURL   string `json:"secure_url"`
	PublicID    string `json:"public_id"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

func (h *Handler) completeSignedUpload(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body completeSignedUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	file, err := h.svc.CompleteSignedUpload(ctx, actor, r.PathValue("file_id"), application.CompleteSignedUploadRequest{
		SecureURL:   body.SecureURL,
		PublicID:    body.PublicID,
		SizeBytes:   body.SizeBytes,
		ContentType: body.ContentType,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, file)
}

func (h *Handler) deliveryURL(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	url, transforms, err := h.svc.GetDeliveryURL(ctx, actor, r.PathValue("file_id"), r.URL.Query().Get("preset"), r.URL.Query().Get("transform"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{
		"url":             url,
		"transformations": transforms,
	})
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	file, rc, err := h.svc.Download(ctx, actor, r.PathValue("file_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.OriginalFilename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func (h *Handler) context(r *http.Request) (context.Context, auth.Actor, bool) {
	actor := auth.FromHeaders(r.Header)
	tenantID := r.Header.Get(tenancy.HeaderTenantID)
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-Code")
	}
	ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{
		TenantID:  tenantID,
		RequestID: r.Header.Get(tenancy.HeaderRequestID),
		ActorID:   actor.UserID,
		ActorRole: actor.Role,
	})
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "file")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureFileManagement)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, domain.ErrStorage):
		httpx.RespondError(w, r, httpx.Error{Code: httpx.ErrInternal, Message: "storage operation failed"})
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func parseMetadata(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return m, nil
}
