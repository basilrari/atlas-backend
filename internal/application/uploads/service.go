package uploads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SupabaseClient defines what we need from Supabase storage.
type SupabaseClient interface {
	CreateSignedUploadURL(ctx context.Context, bucket, path string) (string, error)
}

// HTTPClient is a SupabaseClient backed by the HTTP API.
type HTTPClient struct {
	BaseURL   string
	SecretKey string
	Client    *http.Client
}

type supabaseSignedUploadResponse struct {
	SignedURL string `json:"signedUrl"`
	Path      string `json:"path"`
}

func (c *HTTPClient) CreateSignedUploadURL(ctx context.Context, bucket, path string) (string, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 10 * time.Second}
	}
	base := strings.TrimRight(c.BaseURL, "/")
	url := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", base, bucket, path)

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"expiresIn": 3600, // 1 hour; Express uses supabase-js default
		"upsert":    false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}

	var data supabaseSignedUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.SignedURL == "" {
		return "", fmt.Errorf("supabase returned empty signedUrl")
	}
	return data.SignedURL, nil
}

// Service encapsulates upload logic.
type Service struct {
	Client       SupabaseClient
	SupabaseURL  string
}

// UploadResult matches Express uploadService return shape.
type UploadResult struct {
	UploadURL string `json:"uploadUrl"`
	PublicURL string `json:"publicUrl"`
	Path      string `json:"path"`
}

// GetSignedUploadURL generates a signed upload URL (Express getSignedUploadUrl).
func (s *Service) GetSignedUploadURL(ctx context.Context, bucket, fileName string) (*UploadResult, error) {
	path := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), fileName)

	signedURL, err := s.Client.CreateSignedUploadURL(ctx, bucket, path)
	if err != nil {
		return nil, err
	}

	publicBase := strings.TrimRight(s.SupabaseURL, "/")
	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", publicBase, bucket, path)

	return &UploadResult{
		UploadURL: signedURL,
		PublicURL: publicURL,
		Path:      path,
	}, nil
}
