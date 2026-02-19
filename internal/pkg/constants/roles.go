package constants

const (
	Superadmin = "superadmin"
	Admin      = "admin"
	Manager    = "manager"
	Viewer     = "viewer"
)

// ValidRoles is the set of allowed DB enum values for user role (must match enum_Users_role).
var ValidRoles = []string{Viewer, Manager, Admin, Superadmin}

// IsValidRole returns true if role is one of the allowed enum values.
func IsValidRole(role string) bool {
	for _, r := range ValidRoles {
		if r == role {
			return true
		}
	}
	return false
}
