package listingevents

import (
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handlers struct {
	Service *Service
}

// GET /api/v1/listing-events/get-org-listing-events
func (h *Handlers) GetOrgListingEvents(c *fiber.Ctx) error {
	user := middleware.GetUser(c)
	if user == nil {
		return response.Error(c, "User is not associated with any organization", 401, nil)
	}
	m, ok := user.(map[string]interface{})
	if !ok {
		return response.Error(c, "User is not associated with any organization", 401, nil)
	}
	orgIDStr, _ := m["org_id"].(string)
	if orgIDStr == "" {
		return response.Error(c, "User is not associated with any organization", 401, nil)
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return response.Error(c, "User is not associated with any organization", 401, nil)
	}

	events, err := h.Service.GetOrgListingEvents(c.Context(), orgID)
	if err != nil {
		switch err.Error() {
		case "Organization ID is required":
			return response.Error(c, err.Error(), 400, nil)
		case "Organization not found":
			return response.Error(c, err.Error(), 404, nil)
		default:
			return response.Error(c, err.Error(), 500, nil)
		}
	}

	return response.Success(c, "Listing events fetched successfully", fiber.Map{"events": events}, nil)
}
