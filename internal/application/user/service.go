package user

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"troo-backend/internal/application/policies/user"
	"troo-backend/internal/domain"
	"troo-backend/internal/pkg/constants"
	"troo-backend/internal/pkg/validation"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Service holds DB and Redis for user operations.
type Service struct {
	DB  *gorm.DB
	Rdb *redis.Client
}

// CreateUserInput matches Express createUserService({ user_name, email, password, fullname }).
type CreateUserInput struct {
	UserName string `json:"user_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Fullname string `json:"fullname"`
}

// CreateUser creates a user (Express createUserService). Returns the created model (caller sanitizes password_hash).
func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	if in.UserName == "" || strings.TrimSpace(in.UserName) == "" {
		return nil, errors.New("Username is required and must be a non-empty string")
	}
	if in.Email == "" || !validation.IsValidEmail(in.Email) {
		return nil, errors.New("Invalid email format")
	}
	if in.Password == "" || !validation.IsValidPassword(in.Password) {
		return nil, errors.New("Invalid password format")
	}
	if in.Fullname == "" || strings.TrimSpace(in.Fullname) == "" {
		return nil, errors.New("Full name is required and must be a non-empty string")
	}
	trimmed := strings.TrimSpace(in.Fullname)
	if !validation.IsValidFullname(trimmed) {
		return nil, errors.New("Full name contains invalid characters (only letters, spaces, hyphens, and apostrophes allowed)")
	}

	userName := strings.TrimSpace(in.UserName)
	email := strings.TrimSpace(strings.ToLower(in.Email))
	fullname := titleCaseAndNormalize(trimmed)

	var existing domain.User
	if err := s.DB.WithContext(ctx).Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, errors.New("Email already registered")
	}
	if err := s.DB.WithContext(ctx).Where("user_name = ?", userName).First(&existing).Error; err == nil {
		return nil, errors.New("Username already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 10)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		UserName:     userName,
		Email:        email,
		PasswordHash: string(hash),
		Fullname:     fullname,
		Role:         constants.Viewer,
	}
	if err := s.DB.WithContext(ctx).Create(u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateUser updates allowed fields (Express updateUserService). Allowed: user_name, email, password, fullname, org_id.
func (s *Service) UpdateUser(ctx context.Context, userID string, fields map[string]interface{}) (*domain.User, error) {
	if userID == "" {
		return nil, errors.New("Missing user ID")
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, errors.New("Invalid user ID format (must be a valid UUID)")
	}
	if len(fields) == 0 {
		return nil, errors.New("Missing update fields")
	}

	allowed := map[string]bool{
		"user_name": true, "email": true, "password": true, "fullname": true, "org_id": true,
	}
	upd := make(map[string]interface{})
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		upd[k] = v
	}
	if len(upd) == 0 {
		return nil, errors.New("No valid update fields provided")
	}

	if e, ok := upd["email"].(string); ok && e != "" {
		if !validation.IsValidEmail(e) {
			return nil, errors.New("Invalid email format")
		}
		upd["email"] = strings.TrimSpace(strings.ToLower(e))
	}
	if p, ok := upd["password"].(string); ok && p != "" {
		if !validation.IsValidPassword(p) {
			return nil, errors.New("Invalid password format")
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(p), 10)
		upd["password_hash"] = string(hash)
		delete(upd, "password")
	}
	if fn, ok := upd["fullname"].(string); ok {
		if strings.TrimSpace(fn) == "" {
			return nil, errors.New("Full name must be a non-empty string")
		}
		trimmed := strings.TrimSpace(fn)
		if !validation.IsValidFullname(trimmed) {
			return nil, errors.New("Full name contains invalid characters")
		}
		upd["fullname"] = titleCaseAndNormalize(trimmed)
	}
	if un, ok := upd["user_name"].(string); ok {
		upd["user_name"] = strings.TrimSpace(un)
	}
	if oid, ok := upd["org_id"]; ok {
		if oid == nil {
			upd["org_id"] = nil
		} else if s, ok := oid.(string); ok && s != "" {
			parsed, err := uuid.Parse(s)
			if err != nil {
				return nil, errors.New("Invalid org_id")
			}
			upd["org_id"] = &parsed
		}
	}

	// Uniqueness: no other user (excluding this one) may have the new email or user_name
	if e, ok := upd["email"].(string); ok {
		var dup domain.User
		if err := s.DB.WithContext(ctx).Where("email = ? AND user_id != ?", e, userID).First(&dup).Error; err == nil {
			return nil, errors.New("Email already registered")
		}
	}
	if un, ok := upd["user_name"].(string); ok {
		var dup domain.User
		if err := s.DB.WithContext(ctx).Where("user_name = ? AND user_id != ?", un, userID).First(&dup).Error; err == nil {
			return nil, errors.New("Username already registered")
		}
	}

	result := s.DB.WithContext(ctx).Model(&domain.User{}).Where("user_id = ?", userID).Updates(upd)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("User not found")
	}

	var u domain.User
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ViewUser returns user by ID (Express viewUserService).
func (s *Service) ViewUser(ctx context.Context, userID string) (*domain.User, error) {
	if userID == "" {
		return nil, errors.New("Missing user ID")
	}
	var u domain.User
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("User not found")
		}
		return nil, err
	}
	return &u, nil
}

// UpdateUserRoleInput matches Express updateUserRoleService({ actorUserId, actorRole, targetUserId, targetRole, org_id }).
type UpdateUserRoleInput struct {
	ActorUserID  string
	ActorRole    string
	TargetUserID string
	TargetRole   string
	OrgID        *string
}

// UpdateUserRole updates target user's role after policy check and destroys their sessions (Express updateUserRoleService).
func (s *Service) UpdateUserRole(ctx context.Context, in UpdateUserRoleInput) (*domain.User, error) {
	if err := policies.ValidateRoleAssignment(s.DB, policies.ValidateRoleAssignmentParams{
		ActorRole:    in.ActorRole,
		TargetRole:   in.TargetRole,
		ActorUserID:  in.ActorUserID,
		TargetUserID: in.TargetUserID,
		OrgID:        in.OrgID,
	}); err != nil {
		return nil, err
	}
	var u domain.User
	if err := s.DB.WithContext(ctx).Where("user_id = ?", in.TargetUserID).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("User not found")
		}
		return nil, err
	}
	u.Role = in.TargetRole
	if err := s.DB.WithContext(ctx).Save(&u).Error; err != nil {
		return nil, err
	}
	policies.DestroyUserSessions(ctx, s.Rdb, in.TargetUserID)
	return &u, nil
}

// RemoveUserFromOrgInput matches Express removeUserFromOrgService({ actorUserId, actorRole, targetUserId, org_id }).
type RemoveUserFromOrgInput struct {
	ActorUserID  string
	ActorRole    string
	TargetUserID string
	OrgID        *string
}

// RemoveUserFromOrg validates via policy, sets target org_id=nil and role=viewer, destroys sessions (Express removeUserFromOrgService).
func (s *Service) RemoveUserFromOrg(ctx context.Context, in RemoveUserFromOrgInput) error {
	target, err := policies.ValidateOrgMembershipChange(s.DB, policies.ValidateOrgMembershipChangeParams{
		ActorUserID:  in.ActorUserID,
		ActorRole:    in.ActorRole,
		TargetUserID: in.TargetUserID,
		OrgID:        in.OrgID,
	})
	if err != nil {
		return err
	}
	target.OrgID = nil
	target.Role = constants.Viewer
	if err := s.DB.WithContext(ctx).Save(target).Error; err != nil {
		return err
	}
	policies.DestroyUserSessions(ctx, s.Rdb, in.TargetUserID)
	return nil
}

func titleCaseAndNormalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	runes := []rune(s)
	var b strings.Builder
	capitalize := true
	for _, r := range runes {
		if unicode.IsSpace(r) {
			if !capitalize {
				b.WriteRune(' ')
				capitalize = true
			}
			continue
		}
		if capitalize {
			b.WriteRune(unicode.ToUpper(r))
			capitalize = false
		} else {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
