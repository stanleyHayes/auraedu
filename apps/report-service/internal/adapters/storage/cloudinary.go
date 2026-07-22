package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/report-service/internal/ports"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Cloudinary stores PDFs as raw tenant-scoped objects. Downloads still pass
// through report-service authorization rather than exposing storage paths in UI.
type Cloudinary struct {
	client     *cloudinary.Cloudinary
	httpClient *http.Client
}

var _ ports.ReportStorage = (*Cloudinary)(nil)

func NewCloudinary(rawURL string) (*Cloudinary, error) {
	if !strings.HasPrefix(rawURL, "cloudinary://") {
		return nil, fmt.Errorf("CLOUDINARY_URL must use cloudinary://")
	}
	client, err := cloudinary.NewFromURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("initialize Cloudinary: %w", err)
	}
	return &Cloudinary{client: client, httpClient: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (s *Cloudinary) Save(ctx context.Context, tenantID, objectKey string, content []byte) (string, error) {
	publicID, err := cloudinaryKey(tenantID, objectKey)
	if err != nil {
		return "", err
	}
	result, err := s.client.Upload.Upload(ctx, bytes.NewReader(content), uploader.UploadParams{
		PublicID: publicID, ResourceType: "raw", Type: api.Authenticated, Overwrite: boolPointer(true),
	})
	if err != nil {
		return "", fmt.Errorf("upload report PDF: %w", err)
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("upload report PDF: %s", result.Error.Message)
	}
	if result.PublicID != "" {
		return result.PublicID, nil
	}
	return publicID, nil
}

func (s *Cloudinary) Open(ctx context.Context, tenantID, storagePath string) (io.ReadCloser, error) {
	publicID, err := cloudinaryKey(tenantID, storagePath)
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(time.Minute)
	url, err := s.client.Upload.PrivateDownloadURL(uploader.PrivateDownloadURLParams{
		PublicID: publicID, ResourceType: api.File, DeliveryType: string(api.Authenticated), ExpiresAt: &expires,
	})
	if err != nil {
		return nil, fmt.Errorf("sign private report download: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build report download request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download report PDF: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		statusErr := fmt.Errorf("download report PDF: %s", resp.Status)
		return nil, errors.Join(statusErr, resp.Body.Close())
	}
	return resp.Body, nil
}

func cloudinaryKey(tenantID, objectKey string) (string, error) {
	tenantID = strings.Trim(strings.TrimSpace(tenantID), "/")
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	if tenantID == "" || objectKey == "" || strings.Contains(objectKey, "..") || strings.Contains(objectKey, "\\") {
		return "", fmt.Errorf("invalid report storage key")
	}
	prefix := "reports/" + tenantID + "/"
	if strings.HasPrefix(objectKey, prefix) {
		return objectKey, nil
	}
	if strings.Contains(objectKey, "/") {
		return "", fmt.Errorf("report storage key escapes tenant scope")
	}
	return prefix + objectKey, nil
}

func boolPointer(value bool) *bool { return &value }
