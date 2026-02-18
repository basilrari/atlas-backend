package policies

import "errors"

var (
	ErrOnlySuperadminsCanAssignAdminOrSuperadmin = errors.New("Only superadmins can assign admin or superadmin roles")
	ErrTargetUserNotFound                        = errors.New("Target user not found")
	ErrCannotModifyUsersOutsideYourOrg          = errors.New("Cannot modify users outside your organization")
	ErrUsersCannotModifyTheirOwnRole             = errors.New("Users cannot modify their own role")
	ErrOrgMustHaveAtLeastOneSuperadmin           = errors.New("Organization must have at least one superadmin")

	ErrYouCannotRemoveYourselfFromOrg     = errors.New("You cannot remove yourself from the organization")
	ErrUserNotFound                       = errors.New("User not found")
	ErrUserDoesNotBelongToYourOrg          = errors.New("User does not belong to your organization")
	ErrAdminsCannotRemoveAdminsOrSuperadmins = errors.New("Admins cannot remove admins or superadmins")
)
