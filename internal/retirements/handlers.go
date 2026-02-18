package retirements

import (
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handlers struct {
	Service *Service
}

// GET /api/v1/retirements/view-org
func (h *Handlers) ViewOrg(c *fiber.Ctx) error {
	user := middleware.GetUser(c)
	if user == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	m, ok := user.(map[string]interface{})
	if !ok {
		return response.Error(c, "Authorization error", 500, nil)
	}
	orgIDStr, _ := m["org_id"].(string)

	result, err := h.Service.ViewOrgRetirements(c.Context(), orgIDStr)
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	if result.Error != "" {
		return response.Error(c, result.Error, result.Code, nil)
	}
	return response.Success(c, "Organization retirements fetched successfully", result.Data, nil)
}

// POST /api/v1/retirements/view-one
func (h *Handlers) ViewOne(c *fiber.Ctx) error {
	var body struct {
		CertificateID string `json:"certificate_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.CertificateID == "" {
		return response.Error(c, "certificate_id is required", 400, nil)
	}
	certID, err := uuid.Parse(body.CertificateID)
	if err != nil {
		return response.Error(c, "Invalid certificate_id format", 400, nil)
	}

	result, err := h.Service.ViewOneRetirement(c.Context(), certID)
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	if result.Error != "" {
		return response.Error(c, result.Error, result.Code, nil)
	}
	return response.Success(c, "Retirement certificate fetched successfully", result.Data, nil)
}
