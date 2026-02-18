package retirements

import (
	"context"

	"troo-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	DB *gorm.DB
}

type ViewOrgResult struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Code  int         `json:"code,omitempty"`
}

func (s *Service) ViewOrgRetirements(ctx context.Context, orgID string) (*ViewOrgResult, error) {
	if orgID == "" {
		return &ViewOrgResult{Error: "org_id missing from session", Code: 401}, nil
	}

	var certs []domain.RetirementCertificate
	if err := s.DB.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&certs).Error; err != nil {
		return nil, err
	}
	return &ViewOrgResult{Data: certs}, nil
}

type ViewOneResult struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Code  int         `json:"code,omitempty"`
}

func (s *Service) ViewOneRetirement(ctx context.Context, certificateID uuid.UUID) (*ViewOneResult, error) {
	if certificateID == uuid.Nil {
		return &ViewOneResult{Error: "certificate_id is required", Code: 400}, nil
	}

	var cert domain.RetirementCertificate
	if err := s.DB.WithContext(ctx).Where("certificate_id = ?", certificateID).First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ViewOneResult{Error: "Retirement certificate not found", Code: 404}, nil
		}
		return nil, err
	}
	return &ViewOneResult{Data: cert}, nil
}
