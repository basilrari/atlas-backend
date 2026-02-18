package listings

import (
	"context"
	"errors"
	"fmt"
	"math"

	"troo-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	DB *gorm.DB
}

type CreateListingInput struct {
	ProjectID        uuid.UUID
	SellerID         *uuid.UUID
	CreditsAvailable float64
	PricePerCredit   float64
	ExternalTradeID  *string
	ProjectName      string
	ProjectStartYear int
	Registry         string
	Category         string
	LocationCity     string
	LocationState    string
	LocationCountry  string
	ThumbnailURL     string
	Status           string
	SdgNumbers       string
	Methodology      string
	VintageYear      int
}

func (s *Service) CreateListing(ctx context.Context, in CreateListingInput) (*models.Listing, error) {
	status := in.Status
	if status == "" {
		status = "open"
	}
	listing := &models.Listing{
		ProjectID:        in.ProjectID,
		SellerID:         in.SellerID,
		CreditsAvailable: in.CreditsAvailable,
		PricePerCredit:   in.PricePerCredit,
		ExternalTradeID:  in.ExternalTradeID,
		ProjectName:      in.ProjectName,
		ProjectStartYear: in.ProjectStartYear,
		Registry:         in.Registry,
		Category:         in.Category,
		LocationCity:     in.LocationCity,
		LocationState:    in.LocationState,
		LocationCountry:  in.LocationCountry,
		ThumbnailURL:     in.ThumbnailURL,
		Status:           status,
		SdgNumbers:       in.SdgNumbers,
		Methodology:      in.Methodology,
		VintageYear:      in.VintageYear,
	}

	if err := s.DB.WithContext(ctx).Create(listing).Error; err != nil {
		return nil, fmt.Errorf("Failed to create listing: %v", err)
	}
	return listing, nil
}

func (s *Service) GetAllListings(ctx context.Context) ([]models.Listing, error) {
	var listings []models.Listing
	if err := s.DB.WithContext(ctx).Find(&listings).Error; err != nil {
		return nil, fmt.Errorf("Failed to fetch listings: %v", err)
	}
	return listings, nil
}

func (s *Service) GetOrgListings(ctx context.Context, orgID uuid.UUID) ([]models.Listing, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Organization not associated with user")
	}
	var listings []models.Listing
	if err := s.DB.WithContext(ctx).Where("seller_id = ?", orgID).Order("created_at DESC").Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetListingByID(ctx context.Context, listingID uuid.UUID) (*models.Listing, error) {
	if listingID == uuid.Nil {
		return nil, errors.New("listing_id is required")
	}
	var listing models.Listing
	if err := s.DB.WithContext(ctx).Where("listing_id = ?", listingID).First(&listing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Listing not found")
		}
		return nil, err
	}
	return &listing, nil
}

func (s *Service) GetAllActiveListings(ctx context.Context) ([]models.Listing, error) {
	var listings []models.Listing
	if err := s.DB.WithContext(ctx).Where("status = ?", "open").Order("created_at DESC").Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetAllClosedListings(ctx context.Context) ([]models.Listing, error) {
	var listings []models.Listing
	if err := s.DB.WithContext(ctx).Where("status = ?", "closed").Order("updated_at DESC").Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetOrgActiveListings(ctx context.Context, orgID uuid.UUID) ([]models.Listing, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Org not found in session")
	}
	var listings []models.Listing
	if err := s.DB.WithContext(ctx).Where("seller_id = ? AND status = ?", orgID, "open").Order("created_at DESC").Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetOrgClosedListings(ctx context.Context, orgID uuid.UUID) ([]models.Listing, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Org not found in session")
	}
	var listings []models.Listing
	if err := s.DB.WithContext(ctx).Where("seller_id = ? AND status = ?", orgID, "closed").Order("updated_at DESC").Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

type EditListingInput struct {
	ListingID   uuid.UUID
	OrgID       uuid.UUID
	NewPrice    *float64
	NewQuantity *float64
}

func (s *Service) EditListing(ctx context.Context, in EditListingInput) (*models.Listing, error) {
	if in.ListingID == uuid.Nil {
		return nil, errors.New("Missing listing_id")
	}
	if in.OrgID == uuid.Nil {
		return nil, errors.New("Missing org_id")
	}

	var listing models.Listing
	if err := s.DB.WithContext(ctx).Where("listing_id = ?", in.ListingID).First(&listing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Listing not found")
		}
		return nil, err
	}
	if listing.Status != "open" {
		return nil, errors.New("Listing is not editable")
	}
	if listing.SellerID == nil {
		return nil, errors.New("Registry listings cannot be edited by User")
	}
	if *listing.SellerID != in.OrgID {
		return nil, errors.New("Unauthorized listing edit")
	}

	updates := map[string]interface{}{}

	if in.NewPrice != nil {
		price := *in.NewPrice
		if math.IsNaN(price) || price <= 0 {
			return nil, errors.New("Invalid price")
		}
		if price != listing.PricePerCredit {
			updates["price_per_credit"] = price
		}
	}

	if in.NewQuantity != nil {
		qty := *in.NewQuantity
		if math.IsNaN(qty) || qty <= 0 {
			return nil, errors.New("Invalid quantity")
		}
		currentQty := listing.CreditsAvailable
		delta := qty - currentQty

		if delta != 0 {
			var holding models.Holding
			if err := s.DB.WithContext(ctx).Where("org_id = ? AND project_id = ?", in.OrgID, listing.ProjectID).First(&holding).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil, errors.New("Holdings not found")
				}
				return nil, err
			}

			locked := holding.LockedForSale
			balance := holding.CreditBalance
			available := balance - locked

			if delta > 0 && available < delta {
				return nil, errors.New("Insufficient credits to increase listing")
			}

			newLocked := locked + delta
			if newLocked < 0 {
				return nil, errors.New("Invalid locked_for_sale state")
			}

			if err := s.DB.WithContext(ctx).Model(&holding).Update("locked_for_sale", newLocked).Error; err != nil {
				return nil, err
			}

			updates["credits_available"] = qty
		}
	}

	if len(updates) == 0 {
		return nil, errors.New("No valid changes provided")
	}

	if err := s.DB.WithContext(ctx).Model(&listing).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.DB.WithContext(ctx).Where("listing_id = ?", in.ListingID).First(&listing)
	return &listing, nil
}

func (s *Service) CancelListing(ctx context.Context, listingID, orgID uuid.UUID) (*models.Listing, error) {
	var listing models.Listing
	if err := s.DB.WithContext(ctx).Where("listing_id = ?", listingID).First(&listing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Listing not found")
		}
		return nil, err
	}
	if listing.Status != "open" {
		return nil, errors.New("Listing is not open")
	}
	if listing.SellerID == nil {
		return nil, errors.New("Registry listings cannot be cancelled")
	}
	if *listing.SellerID != orgID {
		return nil, errors.New("Unauthorized")
	}

	var holding models.Holding
	if err := s.DB.WithContext(ctx).Where("org_id = ? AND project_id = ?", orgID, listing.ProjectID).First(&holding).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Holdings not found")
		}
		return nil, err
	}

	newLocked := holding.LockedForSale - listing.CreditsAvailable
	if newLocked < 0 {
		return nil, errors.New("Invalid locked state")
	}

	if err := s.DB.WithContext(ctx).Model(&holding).Update("locked_for_sale", newLocked).Error; err != nil {
		return nil, err
	}

	listing.Status = "closed"
	if err := s.DB.WithContext(ctx).Save(&listing).Error; err != nil {
		return nil, err
	}

	return &listing, nil
}
