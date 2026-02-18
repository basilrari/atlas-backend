package middleware

import (
	"troo-backend/internal/constants"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AuthorizePermission returns a handler that checks the session user's role against PERMISSION_ROLES (Express parity).
// Unconfigured permission -> 500 "Permission configuration error"; role not allowed -> 403 "User is Forbidden from performing this action".
func AuthorizePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := GetUser(c)
		if user == nil {
			return response.Unauthorized(c, "Unauthorized")
		}
		role := getRoleFromUser(user)
		if role == "" {
			return response.Error(c, "Authorization error", 500, nil)
		}
		roles, ok := constants.PermissionRoles[permission]
		if !ok || len(roles) == 0 {
			return response.Error(c, "Permission configuration error", 500, nil)
		}
		if !constants.AllowedRole(permission, role) {
			return response.Error(c, "User is Forbidden from performing this action", 403, nil)
		}
		return c.Next()
	}
}

func getRoleFromUser(user interface{}) string {
	m, ok := user.(map[string]interface{})
	if !ok {
		return ""
	}
	r, _ := m["role"].(string)
	return r
}
