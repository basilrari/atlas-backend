package uploads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SupabaseClient defines what we need from Supabase storage.
type SupabaseClient interface {
	CreateSignedUploadURL(ctx context.Context, bucket, path string) (string, error)
}

// HTTPClient is a SupabaseClient backed by the HTTP API.
// BaseURL should be SUPABASE_URL (e.g. https://xwsiuytkbefejvoqpjyg.supabase.co) so sign and public URLs use the same origin (CORS).
type HTTPClient struct {
	BaseURL   string
	SecretKey string
	Client    *http.Client
}

type supabaseSignedUploadResponse struct {
	SignedURL string `json:"signedUrl"`
	SignedURLSnake string `json:"signed_url"`
	URL       string `json:"url"` // relative path returned by upload/sign API
	Path      string `json:"path"`
}

func (c *HTTPClient) CreateSignedUploadURL(ctx context.Context, bucket, path string) (string, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 10 * time.Second}
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		return "", fmt.Errorf("supabase: SUPABASE_URL is not set")
	}
	if c.SecretKey == "" {
		return "", fmt.Errorf("supabase: SUPABASE_SECRET_KEY is not set")
	}
	// Sign URL: https://{project}.supabase.co/storage/v1/object/upload/sign/{bucket}/{path}
	url := fmt.Sprintf("%s/storage/v1/object/upload/sign/%s/%s", base, bucket, path)

	reqBody := map[string]interface{}{
		"path":   path,
		"upsert": false,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	// Same as Express/supabase-js: apikey + Bearer (same key = service_role)
	req.Header.Set("apikey", c.SecretKey)
	req.Header.Set("Authorization", "Bearer "+c.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("supabase request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyStr := string(respBody)
		if resp.StatusCode == 400 || resp.StatusCode == 403 {
			if strings.Contains(bodyStr, "Invalid Compact JWS") || strings.Contains(bodyStr, "Unauthorized") {
				return "", fmt.Errorf("supabase storage requires the service_role key (secret), not the anon key: set SUPABASE_SECRET_KEY to your project's service_role key from Supabase Dashboard → Project Settings → API (raw body: %s)", bodyStr)
			}
		}
		return "", fmt.Errorf("supabase error: status %d body: %s", resp.StatusCode, bodyStr)
	}

	var data supabaseSignedUploadResponse
	if err := json.Unmarshal(respBody, &data); err != nil {
		return "", fmt.Errorf("supabase response decode: %w", err)
	}

	var signedURL string
	if data.SignedURL != "" {
		signedURL = data.SignedURL
	} else if data.SignedURLSnake != "" {
		signedURL = data.SignedURLSnake
	} else if data.URL != "" {
		signedURL = data.URL
	} else {
		return "", fmt.Errorf("supabase returned no signed URL, body: %s", string(respBody))
	}

	// Fix: Supabase returns relative "/object/upload/sign/..." → we must insert /storage/v1/
	if !strings.HasPrefix(signedURL, "http") {
		if strings.HasPrefix(signedURL, "/object/") {
			signedURL = base + "/storage/v1" + signedURL
		} else if strings.HasPrefix(signedURL, "/storage/") {
			signedURL = base + signedURL
		} else {
			signedURL = base + "/storage/v1" + signedURL
		}
	}

	return signedURL, nil
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
