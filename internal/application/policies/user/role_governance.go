package policies

import (
	"troo-backend/internal/domain"
	"troo-backend/internal/pkg/constants"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func sameOrg(orgIDStr *string, orgIDUUID *uuid.UUID) bool {
	if orgIDStr == nil && orgIDUUID == nil {
		return true
	}
	if orgIDStr == nil || orgIDUUID == nil {
		return false
	}
	return *orgIDStr == orgIDUUID.String()
}

// ValidateRoleAssignment replicates express user/policies/roleGovernance.js.
// Returns nil on success; returns an error with the exact Express message on failure.
func ValidateRoleAssignment(db *gorm.DB, params ValidateRoleAssignmentParams) error {
	// Only superadmin can assign admin/superadmin
	if (params.TargetRole == constants.Admin || params.TargetRole == constants.Superadmin) &&
		params.ActorRole != constants.Superadmin {
		return ErrOnlySuperadminsCanAssignAdminOrSuperadmin
	}
	if params.TargetUserID == "" {
		return nil // invitations stop here
	}
	var target domain.User
	if err := db.Where("user_id = ?", params.TargetUserID).First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrTargetUserNotFound
		}
		return err
	}
	if !sameOrg(params.OrgID, target.OrgID) {
		return ErrCannotModifyUsersOutsideYourOrg
	}
	// Prevent self-role modification
	if params.ActorUserID == params.TargetUserID && params.ActorRole != constants.Superadmin {
		return ErrUsersCannotModifyTheirOwnRole
	}
	// Prevent last superadmin downgrade
	if target.Role == constants.Superadmin && params.TargetRole != constants.Superadmin {
		var count int64
		if params.OrgID == nil {
			db.Model(&domain.User{}).Where("org_id IS NULL AND role = ?", constants.Superadmin).Count(&count)
		} else {
			db.Model(&domain.User{}).Where("org_id = ? AND role = ?", params.OrgID, constants.Superadmin).Count(&count)
		}
		if count <= 1 {
			return ErrOrgMustHaveAtLeastOneSuperadmin
		}
	}
	return nil
}

type ValidateRoleAssignmentParams struct {
	ActorRole    string
	TargetRole   string
	ActorUserID  string
	TargetUserID string
	OrgID        *string
}
