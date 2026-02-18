package holdings

import (
	"context"
	"errors"

	"troo-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service encapsulates holdings operations.
type Service struct {
	DB *gorm.DB
}

// ViewHoldings returns holdings for an org (Express viewHoldingService).
func (s *Service) ViewHoldings(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("org_id is required")
	}

	var org domain.Org
	if err := s.DB.WithContext(ctx).Where("org_id = ?", orgID).First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Organization not found")
		}
		return nil, err
	}

	var holdings []domain.Holding
	if err := s.DB.WithContext(ctx).
		Where("org_id = ?", orgID).
		Find(&holdings).Error; err != nil {
		return nil, err
	}

	return holdings, nil
}

// ViewProjectByHoldingID mirrors getProjectByHoldingIdService.
func (s *Service) ViewProjectByHoldingID(ctx context.Context, holdingID, orgID uuid.UUID) (map[string]interface{}, error) {
	if holdingID == uuid.Nil {
		return nil, errors.New("holding_id is required")
	}

	var holding domain.Holding
	if err := s.DB.WithContext(ctx).Where("holding_id = ?", holdingID).First(&holding).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Holding not found")
		}
		return nil, err
	}

	if holding.OrgID != orgID {
		return nil, errors.New("Unauthorized access to holding")
	}

	var project domain.IcrProject
	if err := s.DB.WithContext(ctx).
		Where("id = ?", holding.ProjectID).
		First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Project not found")
		}
		return nil, err
	}

	// Build merged payload
	result := map[string]interface{}{
		"holding_id": holding.HoldingID,
		"project_id": holding.ProjectID,
	}

	// Use map serialization to mimic project.toJSON()
	result["project"] = project
	return result, nil
}
