package policies

import (
	"errors"
	"strings"
	"time"

	"troo-backend/internal/models"

	"gorm.io/gorm"
)

// ValidateInviteCreation replicates invitations/policies/invitePolicy.js validateInviteCreation.
func ValidateInviteCreation(db *gorm.DB, email string, orgID string, actorEmail string) error {
	normalized := strings.ToLower(email)

	if normalized == strings.ToLower(actorEmail) {
		return errors.New("You cannot invite yourself")
	}

	var user models.User
	if err := db.Where("email = ?", normalized).First(&user).Error; err == nil {
		if user.OrgID != nil && user.OrgID.String() == orgID {
			return errors.New("User already belongs to this organization")
		}
	}

	var invite models.Invitation
	if err := db.Where("org_id = ? AND email = ? AND status = ?", orgID, normalized, "pending").
		First(&invite).Error; err == nil {
		return errors.New("A pending invitation already exists for this email")
	}

	return nil
}

// ValidateInviteAcceptance replicates invitations/policies/invitePolicy.js validateInviteAcceptance.
func ValidateInviteAcceptance(invitation *models.Invitation, userEmail string) error {
	if strings.ToLower(invitation.Email) != strings.ToLower(userEmail) {
		return errors.New("Invitation email does not match logged-in user")
	}

	if invitation.Status != "pending" {
		return errors.New("Invitation is no longer valid")
	}

	if invitation.ExpiresAt.Before(time.Now()) {
		return errors.New("Invitation has expired")
	}

	return nil
}
