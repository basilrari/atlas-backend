package invitations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"troo-backend/internal/invitations/policies"
	"troo-backend/internal/models"
	userPolicies "troo-backend/internal/user/policies"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const dayMS = 24 * time.Hour
const inviteExpiry = 7 * dayMS

type Service struct {
	DB *gorm.DB
}

type SendInviteInput struct {
	ActorUserID string
	ActorRole   string
	ActorEmail  string
	OrgID       string
	Email       string
	Role        string
}

func (s *Service) SendInvite(ctx context.Context, in SendInviteInput) (*models.Invitation, error) {
	if err := userPolicies.ValidateRoleAssignment(s.DB, userPolicies.ValidateRoleAssignmentParams{
		ActorRole:    in.ActorRole,
		TargetRole:   in.Role,
		ActorUserID:  in.ActorUserID,
		TargetUserID: "",
		OrgID:        &in.OrgID,
	}); err != nil {
		return nil, err
	}

	if err := policies.ValidateInviteCreation(s.DB, in.Email, in.OrgID, in.ActorEmail); err != nil {
		return nil, err
	}

	normalized := strings.ToLower(in.Email)
	token := randomHex(32)
	expiresAt := time.Now().Add(inviteExpiry)

	var existing models.Invitation
	err := s.DB.WithContext(ctx).Where("org_id = ? AND email = ?", in.OrgID, normalized).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		orgUUID, _ := uuid.Parse(in.OrgID)
		inv := &models.Invitation{
			OrgID:       orgUUID,
			Email:       normalized,
			Role:        in.Role,
			InviteToken: token,
			ExpiresAt:   expiresAt,
			CreatedBy:   in.ActorUserID,
			Status:      "pending",
		}
		if err := s.DB.WithContext(ctx).Create(inv).Error; err != nil {
			return nil, err
		}
		return inv, nil
	} else if err != nil {
		return nil, err
	}

	existing.InviteToken = token
	existing.Role = in.Role
	existing.Status = "pending"
	existing.ExpiresAt = expiresAt
	if err := s.DB.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

type ResendInviteInput struct {
	Email string
	OrgID string
}

func (s *Service) ResendInvite(ctx context.Context, in ResendInviteInput) (*models.Invitation, error) {
	normalized := strings.ToLower(in.Email)

	var inv models.Invitation
	if err := s.DB.WithContext(ctx).Where("email = ? AND org_id = ?", normalized, in.OrgID).First(&inv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Invitation not found")
		}
		return nil, err
	}

	if time.Since(inv.UpdatedAt) < dayMS {
		return nil, errors.New("Invite can only be resent once per day")
	}

	inv.InviteToken = randomHex(32)
	inv.Status = "pending"
	inv.ExpiresAt = time.Now().Add(inviteExpiry)
	if err := s.DB.WithContext(ctx).Save(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

type AcceptInviteInput struct {
	Token  string
	UserID string
}

type AcceptInviteResult struct {
	OrgID   string `json:"org_id"`
	Role    string `json:"role"`
	OrgName string `json:"org_name"`
}

func (s *Service) AcceptInvite(ctx context.Context, in AcceptInviteInput) (*AcceptInviteResult, error) {
	var inv models.Invitation
	if err := s.DB.WithContext(ctx).Where("invite_token = ?", in.Token).First(&inv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Invalid invitation token")
		}
		return nil, err
	}

	if inv.Status != "pending" {
		return nil, errors.New("Invitation is no longer valid")
	}

	if inv.ExpiresAt.Before(time.Now()) {
		inv.Status = "expired"
		s.DB.WithContext(ctx).Save(&inv)
		return nil, errors.New("Invitation has expired")
	}

	var user models.User
	if err := s.DB.WithContext(ctx).Where("user_id = ?", in.UserID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("User not found")
		}
		return nil, err
	}

	if strings.ToLower(user.Email) != strings.ToLower(inv.Email) {
		return nil, errors.New("Invitation email does not match logged-in user")
	}

	user.OrgID = &inv.OrgID
	user.Role = inv.Role
	if err := s.DB.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, err
	}

	inv.Status = "accepted"
	s.DB.WithContext(ctx).Save(&inv)

	var org models.Org
	orgName := ""
	if err := s.DB.WithContext(ctx).Where("org_id = ?", inv.OrgID).First(&org).Error; err == nil {
		orgName = org.OrgName
	}

	return &AcceptInviteResult{
		OrgID:   inv.OrgID.String(),
		Role:    inv.Role,
		OrgName: orgName,
	}, nil
}

type RevokeInviteInput struct {
	Email string
	OrgID string
}

func (s *Service) RevokeInvite(ctx context.Context, in RevokeInviteInput) (*models.Invitation, error) {
	normalized := strings.ToLower(in.Email)

	var inv models.Invitation
	if err := s.DB.WithContext(ctx).Where("email = ? AND org_id = ? AND status = ?", normalized, in.OrgID, "pending").
		First(&inv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Pending invitation not found")
		}
		return nil, err
	}

	inv.Status = "revoked"
	if err := s.DB.WithContext(ctx).Save(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

type ListInvitesInput struct {
	OrgID  string
	Status string
}

func (s *Service) ListOrgInvitations(ctx context.Context, in ListInvitesInput) ([]models.Invitation, error) {
	q := s.DB.WithContext(ctx).Where("org_id = ?", in.OrgID)
	if in.Status != "" {
		q = q.Where("status = ?", in.Status)
	}
	var invitations []models.Invitation
	if err := q.Order("created_at DESC").Find(&invitations).Error; err != nil {
		return nil, err
	}
	return invitations, nil
}

type CheckTokenResult struct {
	Email   string `json:"email"`
	Role    string `json:"role"`
	OrgID   string `json:"org_id"`
	Valid   bool   `json:"valid"`
	OrgName string `json:"org_name"`
	OrgCode string `json:"org_code"`
}

func (s *Service) CheckInvitationToken(ctx context.Context, token string) (*CheckTokenResult, error) {
	if token == "" {
		return nil, errors.New("Invitation token is required")
	}

	var inv models.Invitation
	if err := s.DB.WithContext(ctx).Where("invite_token = ?", token).First(&inv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Invalid invitation token")
		}
		return nil, err
	}

	if inv.Status != "pending" {
		return nil, errors.New("Invitation is no longer valid")
	}

	if inv.ExpiresAt.Before(time.Now()) {
		inv.Status = "expired"
		s.DB.WithContext(ctx).Save(&inv)
		return nil, errors.New("Invitation has expired")
	}

	var org models.Org
	orgName, orgCode := "", ""
	if err := s.DB.WithContext(ctx).Where("org_id = ?", inv.OrgID).First(&org).Error; err == nil {
		orgName = org.OrgName
		orgCode = org.OrgCode
	}

	return &CheckTokenResult{
		Email:   inv.Email,
		Role:    inv.Role,
		OrgID:   inv.OrgID.String(),
		Valid:   true,
		OrgName: orgName,
		OrgCode: orgCode,
	}, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
