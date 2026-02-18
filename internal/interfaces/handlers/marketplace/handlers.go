package marketplace

import (
	mktsvc "troo-backend/internal/application/marketplace"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handlers bundles marketplace handlers.
type Handlers struct {
	Service *mktsvc.Service
}

// GetAllProjects GET /api/v1/marketplace/projects
func (h *Handlers) GetAllProjects(c *fiber.Ctx) error {
	status := c.Query("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	data, err := h.Service.GetAllProjects(c.Context(), statusPtr)
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	// Special marketplace shape: { success: true, data }
	return c.JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

// GetProjectByID GET /api/v1/marketplace/projects/:id
func (h *Handlers) GetProjectByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.JSON(fiber.Map{
			"success": false,
		})
	}
	project, err := h.Service.GetProjectByID(c.Context(), id)
	if err != nil {
		if err.Error() == "Project not found" {
			// Express passes through 404 by default error handler; here we standardize:
			return response.Error(c, err.Error(), 404, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    project,
	})
}

// AdminSync POST /api/v1/marketplace/admin-sync
func (h *Handlers) AdminSync(c *fiber.Ctx) error {
	data, err := h.Service.SyncIcrProjects(c.Context())
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	// Express: res.json(data); data already includes success/count fields.
	return c.JSON(data)
}
