package org

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"troo-backend/internal/domain"
	"troo-backend/internal/pkg/constants"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service encapsulates org-related operations.
type Service struct {
	DB *gorm.DB
}

// CreateOrgInput mirrors Express createOrgService payload.
type CreateOrgInput struct {
	OrgName            string  `json:"org_name"`
	CountryCode        string  `json:"country_code"`
	RegistrationID     *string `json:"registration_id"`
	LogoURL            *string `json:"logo_url"`
	IncorporationDocURL *string `json:"incorporation_doc_url"`
}

// generateOrgCode replicates Express generateOrgCode(org_name, org_id).
func generateOrgCode(orgName string, orgID uuid.UUID) string {
	onlyLetters := regexp.MustCompile(`[^A-Za-z]`).ReplaceAllString(orgName, "")
	prefix := strings.ToUpper(onlyLetters)
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	for len(prefix) < 2 {
		prefix += "X"
	}
	suffix := strings.ToUpper(strings.ReplaceAll(orgID.String(), "-", ""))
	if len(suffix) > 6 {
		suffix = suffix[:6]
	}
	return prefix + "-" + suffix
}

// CreateOrg creates an organization and updates the creator user to superadmin (Express createOrgService).
func (s *Service) CreateOrg(ctx context.Context, in CreateOrgInput, userID uuid.UUID) (*domain.Org, error) {
	if in.OrgName == "" || in.CountryCode == "" {
		return nil, errors.New("org_name and country_code are required")
	}

	orgID := uuid.New()
	orgCode := generateOrgCode(in.OrgName, orgID)

	org := &domain.Org{
		OrgID:       orgID,
		OrgName:     strings.TrimSpace(in.OrgName),
		OrgCode:     orgCode,
		CountryCode: strings.ToUpper(in.CountryCode),
	}
	if in.RegistrationID != nil {
		org.RegistrationID = in.RegistrationID
	}
	if in.LogoURL != nil {
		org.LogoURL = in.LogoURL
	}
	if in.IncorporationDocURL != nil {
		org.IncorporationDocURL = in.IncorporationDocURL
	}

	if err := s.DB.WithContext(ctx).Create(org).Error; err != nil {
		return nil, err
	}

	// Attach creator to org and make superadmin (Express: User.update({ org_id, role: 'superadmin' }, { where: { user_id } }))
	if err := s.DB.WithContext(ctx).Model(&domain.User{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"org_id": org.OrgID,
			"role":   constants.Superadmin,
		}).Error; err != nil {
		return nil, err
	}

	return org, nil
}

// GetOrgByID returns org + employees (Express getOrgByIdService).
func (s *Service) GetOrgByID(ctx context.Context, orgID uuid.UUID) (map[string]interface{}, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Missing org_id")
	}
	var org domain.Org
	if err := s.DB.WithContext(ctx).Where("org_id = ?", orgID).First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Org not found")
		}
		return nil, err
	}

	var employees []struct {
		UserID    uuid.UUID `json:"user_id"`
		Fullname  string    `json:"fullname"`
		Email     string    `json:"email"`
		UserName  string    `json:"user_name"`
		Role      string    `json:"role"`
		CreatedAt string    `json:"createdAt"`
	}
	if err := s.DB.WithContext(ctx).
		Model(&domain.User{}).
		Select("user_id, fullname, email, user_name, role, created_at").
		Where("org_id = ?", orgID).
		Order("created_at ASC").
		Scan(&employees).Error; err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"org_id":               org.OrgID,
		"org_name":             org.OrgName,
		"org_code":             org.OrgCode,
		"country_code":         org.CountryCode,
		"registration_id":      org.RegistrationID,
		"logo_url":             org.LogoURL,
		"incorporation_doc_url": org.IncorporationDocURL,
		"createdAt":            org.CreatedAt,
		"updatedAt":            org.UpdatedAt,
		"employees":            employees,
	}
	return result, nil
}

// UpdateOrg updates allowed fields (Express updateOrgService).
func (s *Service) UpdateOrg(ctx context.Context, orgID uuid.UUID, fields map[string]interface{}) (*domain.Org, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Missing org_id")
	}
	if len(fields) == 0 {
		return nil, errors.New("No update fields provided")
	}

	allowed := map[string]bool{
		"org_name":             true,
		"country_code":         true,
		"registration_id":      true,
		"logo_url":             true,
		"incorporation_doc_url": true,
	}
	valid := make(map[string]interface{})
	for k, v := range fields {
		if allowed[k] {
			valid[k] = v
		}
	}
	if len(valid) == 0 {
		return nil, errors.New("No valid fields to update")
	}

	result := s.DB.WithContext(ctx).Model(&domain.Org{}).
		Where("org_id = ?", orgID).
		Updates(valid)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("Org not found")
	}

	var org domain.Org
	if err := s.DB.WithContext(ctx).Where("org_id = ?", orgID).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}
