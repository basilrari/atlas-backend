package listingevents

import (
	"context"
	"errors"

	"troo-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	DB *gorm.DB
}

func (s *Service) GetOrgListingEvents(ctx context.Context, orgID uuid.UUID) ([]domain.ListingEvent, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Organization ID is required")
	}

	var org domain.Org
	if err := s.DB.WithContext(ctx).Where("org_id = ?", orgID).Select("org_code").First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Organization not found")
		}
		return nil, err
	}

	var events []domain.ListingEvent
	if err := s.DB.WithContext(ctx).Where("actor_org_code = ?", org.OrgCode).Order(`"createdAt" ASC`).Find(&events).Error; err != nil {
		return nil, err
	}

	return events, nil
}
