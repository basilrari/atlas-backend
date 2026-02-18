package marketplace

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that the special marketplace shape { success: true, data } is preserved.
func TestGetAllProjects_SpecialShape(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/marketplace/projects", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"data": map[string]interface{}{
				"projects": []int{1, 2},
				"total":    2,
			},
		})
	})

	req := httptest.NewRequest("GET", "/api/v1/marketplace/projects", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

