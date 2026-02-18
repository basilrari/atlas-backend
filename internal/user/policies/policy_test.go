package policies

import (
	"testing"

	"troo-backend/internal/constants"
	"troo-backend/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPolicyDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	return db
}

func TestValidateRoleAssignment_OnlySuperadminCanAssignAdmin(t *testing.T) {
	db := setupPolicyDB(t)
	orgID := uuid.New()
	params := ValidateRoleAssignmentParams{
		ActorRole: constants.Admin, TargetRole: constants.Superadmin,
		ActorUserID: "a", TargetUserID: "b", OrgID: strPtr(orgID.String()),
	}
	err := ValidateRoleAssignment(db, params)
	require.Error(t, err)
	assert.Equal(t, ErrOnlySuperadminsCanAssignAdminOrSuperadmin, err)
}

func TestValidateRoleAssignment_TargetUserNotFound(t *testing.T) {
	db := setupPolicyDB(t)
	params := ValidateRoleAssignmentParams{
		ActorRole: constants.Superadmin, TargetRole: constants.Admin,
		ActorUserID: "a", TargetUserID: uuid.New().String(), OrgID: nil,
	}
	err := ValidateRoleAssignment(db, params)
	require.Error(t, err)
	assert.Equal(t, ErrTargetUserNotFound, err)
}

func TestValidateRoleAssignment_UsersCannotModifyTheirOwnRole(t *testing.T) {
	db := setupPolicyDB(t)
	uid := uuid.New()
	orgID := uuid.New()
	require.NoError(t, db.Create(&models.User{
		UserID: uid, UserName: "u", Email: "u@x.com", PasswordHash: "x", Fullname: "U", Role: constants.Admin, OrgID: &orgID,
	}).Error)
	params := ValidateRoleAssignmentParams{
		ActorRole: constants.Admin, TargetRole: constants.Manager,
		ActorUserID: uid.String(), TargetUserID: uid.String(), OrgID: strPtr(orgID.String()),
	}
	err := ValidateRoleAssignment(db, params)
	require.Error(t, err)
	assert.Equal(t, ErrUsersCannotModifyTheirOwnRole, err)
}

func TestValidateOrgMembershipChange_YouCannotRemoveYourself(t *testing.T) {
	db := setupPolicyDB(t)
	params := ValidateOrgMembershipChangeParams{
		ActorUserID: "same", TargetUserID: "same", ActorRole: constants.Admin, OrgID: nil,
	}
	_, err := ValidateOrgMembershipChange(db, params)
	require.Error(t, err)
	assert.Equal(t, ErrYouCannotRemoveYourselfFromOrg, err)
}

func TestValidateOrgMembershipChange_UserNotFound(t *testing.T) {
	db := setupPolicyDB(t)
	params := ValidateOrgMembershipChangeParams{
		ActorUserID: "a", TargetUserID: uuid.New().String(), ActorRole: constants.Admin, OrgID: nil,
	}
	_, err := ValidateOrgMembershipChange(db, params)
	require.Error(t, err)
	assert.Equal(t, ErrUserNotFound, err)
}

func strPtr(s string) *string { return &s }
