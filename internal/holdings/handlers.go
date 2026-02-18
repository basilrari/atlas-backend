package holdings

import (
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handlers bundles holdings handlers.
type Handlers struct {
	Service *Service
}

// ViewHoldings GET /api/v1/holdings/view-holdings
func (h *Handlers) ViewHoldings(c *fiber.Ctx) error {
	user := middleware.GetUser(c)
	if user == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	m, ok := user.(map[string]interface{})
	if !ok {
		return response.Error(c, "Authorization error", 500, nil)
	}
	orgIDStr, _ := m["org_id"].(string)
	if orgIDStr == "" {
		return response.Error(c, "Invalid org ID format (must be a valid UUID)", 400, nil)
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return response.Error(c, "Invalid org ID format (must be a valid UUID)", 400, nil)
	}

	data, err := h.Service.ViewHoldings(c.Context(), orgID)
	if err != nil {
		switch err.Error() {
		case "org_id is required":
			return response.Error(c, err.Error(), 400, nil)
		case "Organization not found":
			return response.Error(c, err.Error(), 404, nil)
		default:
			return response.Error(c, "Internal Server Error", 500, nil)
		}
	}

	return response.Success(c, "Holdings fetched successfully", data, nil)
}

type viewProjectRequest struct {
	HoldingID string `json:"holding_id"`
}

// ViewProject POST /api/v1/holdings/view-project
func (h *Handlers) ViewProject(c *fiber.Ctx) error {
	var req viewProjectRequest
	if err := c.BodyParser(&req); err != nil || req.HoldingID == "" {
		return response.Error(c, "Invalid Holding ID format (must be a valid UUID)", 400, nil)
	}
	holdingID, err := uuid.Parse(req.HoldingID)
	if err != nil {
		return response.Error(c, "Invalid Holding ID format (must be a valid UUID)", 400, nil)
	}

	user := middleware.GetUser(c)
	if user == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	m, ok := user.(map[string]interface{})
	if !ok {
		return response.Error(c, "Authorization error", 500, nil)
	}
	orgIDStr, _ := m["org_id"].(string)
	if orgIDStr == "" {
		return response.Error(c, "Invalid org ID format (must be a valid UUID)", 400, nil)
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return response.Error(c, "Invalid org ID format (must be a valid UUID)", 400, nil)
	}

	data, err := h.Service.ViewProjectByHoldingID(c.Context(), holdingID, orgID)
	if err != nil {
		switch err.Error() {
		case "holding_id is required":
			return response.Error(c, "holding_id is required", 400, nil)
		case "Holding not found":
			return response.Error(c, err.Error(), 404, nil)
		case "Unauthorized access to holding":
			return response.Error(c, err.Error(), 403, nil)
		case "Project not found":
			return response.Error(c, err.Error(), 404, nil)
		default:
			return response.Error(c, "Internal Server Error", 500, nil)
		}
	}

	return response.Success(c, "Project fetched successfully", data, nil)
}

