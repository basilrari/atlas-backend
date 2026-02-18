package uploads

import (
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handlers bundles upload handlers with the service.
type Handlers struct {
	Service *Service
}

type uploadRequest struct {
	FileName string `json:"file_name"`
}

// UploadOrgLogo POST /api/v1/uploads/org-logo
func (h *Handlers) UploadOrgLogo(c *fiber.Ctx) error {
	var req uploadRequest
	if err := c.BodyParser(&req); err != nil || req.FileName == "" {
		return response.Error(c, "file_name is required", 400, nil)
	}

	res, err := h.Service.GetSignedUploadURL(c.Context(), "org-logos", req.FileName)
	if err != nil {
		return response.Error(c, "Failed to generate upload URL", 500, nil)
	}
	return response.Success(c, "Upload URL generated", res, nil)
}

// UploadOrgDoc POST /api/v1/uploads/org-doc
func (h *Handlers) UploadOrgDoc(c *fiber.Ctx) error {
	var req uploadRequest
	if err := c.BodyParser(&req); err != nil || req.FileName == "" {
		return response.Error(c, "file_name is required", 400, nil)
	}

	res, err := h.Service.GetSignedUploadURL(c.Context(), "org-docs", req.FileName)
	if err != nil {
		return response.Error(c, "Failed to generate upload URL", 500, nil)
	}
	return response.Success(c, "Upload URL generated", res, nil)
}

