package constants

// PermissionRoles maps each permission to roles allowed to perform it (Express PERMISSION_ROLES).
var PermissionRoles = map[string][]string{
	ViewData:        {Viewer, Manager, Admin, Superadmin},
	BuyCredits:      {Manager, Admin, Superadmin},
	SellCredits:     {Manager, Admin, Superadmin},
	RetireCredits:   {Manager, Admin, Superadmin},
	TransferCredits: {Admin, Superadmin},
	CreateListing:   {Manager, Admin, Superadmin},
	EditListing:     {Manager, Admin, Superadmin},
	CancelListing:   {Manager, Admin, Superadmin},
	InviteUser:      {Admin, Superadmin},
	RemoveUser:      {Admin, Superadmin},
	AssignRole:      {Admin, Superadmin},
	ManageAdmins:    {Superadmin},
	UpdateOrg:       {Admin, Superadmin},
}

// AllowedRole returns true if role is in the list of allowed roles for the permission.
func AllowedRole(permission, role string) bool {
	roles, ok := PermissionRoles[permission]
	if !ok {
		return false
	}
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
