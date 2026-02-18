package uploads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	lastBucket string
	lastPath   string
	err        error
}

func (f *fakeClient) CreateSignedUploadURL(ctx context.Context, bucket, path string) (string, error) {
	f.lastBucket = bucket
	f.lastPath = path
	if f.err != nil {
		return "", f.err
	}
	return "https://example.com/upload", nil
}

func setupUploadTest(t *testing.T) (*Handlers, *fakeClient) {
	client := &fakeClient{}
	svc := &Service{
		Client:      client,
		SupabaseURL: "https://example.supabase.co",
	}
	h := &Handlers{Service: svc}
	return h, client
}

func TestUploadOrgLogo_MissingFileName(t *testing.T) {
	h, _ := setupUploadTest(t)
	app := fiber.New()
	app.Post("/api/v1/uploads/org-logo", h.UploadOrgLogo)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/api/v1/uploads/org-logo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestUploadOrgLogo_Success(t *testing.T) {
	h, client := setupUploadTest(t)
	app := fiber.New()
	app.Post("/api/v1/uploads/org-logo", h.UploadOrgLogo)

	body, _ := json.Marshal(map[string]string{"file_name": "logo.png"})
	req := httptest.NewRequest("POST", "/api/v1/uploads/org-logo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "org-logos", client.lastBucket)
}

func TestUploadOrgDoc_Success(t *testing.T) {
	h, client := setupUploadTest(t)
	app := fiber.New()
	app.Post("/api/v1/uploads/org-doc", h.UploadOrgDoc)

	body, _ := json.Marshal(map[string]string{"file_name": "doc.pdf"})
	req := httptest.NewRequest("POST", "/api/v1/uploads/org-doc", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "org-docs", client.lastBucket)
}

