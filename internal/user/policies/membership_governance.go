package policies

import (
	"troo-backend/internal/constants"
	"troo-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func sameOrgMembership(orgIDStr *string, orgIDUUID *uuid.UUID) bool {
	if orgIDStr == nil && orgIDUUID == nil {
		return true
	}
	if orgIDStr == nil || orgIDUUID == nil {
		return false
	}
	return *orgIDStr == orgIDUUID.String()
}

// ValidateOrgMembershipChange replicates express user/policies/membershipGovernance.js.
// Returns the target user on success; returns an error with the exact Express message on failure.
func ValidateOrgMembershipChange(db *gorm.DB, params ValidateOrgMembershipChangeParams) (*models.User, error) {
	if params.ActorUserID == params.TargetUserID {
		return nil, ErrYouCannotRemoveYourselfFromOrg
	}
	var target models.User
	if err := db.Where("user_id = ?", params.TargetUserID).First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if !sameOrgMembership(params.OrgID, target.OrgID) {
		return nil, ErrUserDoesNotBelongToYourOrg
	}
	// Admin cannot remove admin/superadmin
	if params.ActorRole == constants.Admin &&
		(target.Role == constants.Admin || target.Role == constants.Superadmin) {
		return nil, ErrAdminsCannotRemoveAdminsOrSuperadmins
	}
	// Prevent last superadmin removal
	if target.Role == constants.Superadmin {
		var count int64
		if target.OrgID == nil {
			db.Model(&models.User{}).Where("org_id IS NULL AND role = ?", constants.Superadmin).Count(&count)
		} else {
			db.Model(&models.User{}).Where("org_id = ? AND role = ?", target.OrgID, constants.Superadmin).Count(&count)
		}
		if count <= 1 {
			return nil, ErrOrgMustHaveAtLeastOneSuperadmin
		}
	}
	return &target, nil
}

type ValidateOrgMembershipChangeParams struct {
	ActorUserID  string
	ActorRole    string
	TargetUserID string
	OrgID        *string
}
