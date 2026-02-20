package listings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"troo-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/datatypes"
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

func (s *Service) CreateListing(ctx context.Context, in CreateListingInput) (*domain.Listing, error) {
	status := in.Status
	if status == "" {
		status = "open"
	}
	listing := &domain.Listing{
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
		SdgNumbers:       domain.SDGNumbers(in.SdgNumbers),
		Methodology:      in.Methodology,
		VintageYear:      in.VintageYear,
	}

	tx := s.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Create(listing).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("Failed to create listing: %v", err)
	}
	// Match Express: createListingEvent(CREATED) after listing create (same transaction).
	eventDataBytes, _ := json.Marshal(map[string]interface{}{
		"price_per_credit":  listing.PricePerCredit,
		"credits_available": listing.CreditsAvailable,
		"source":            "registry",
	})
	if err := tx.Create(&domain.ListingEvent{
		ListingID:    listing.ListingID,
		EventType:    "CREATED",
		EventData:    datatypes.JSON(eventDataBytes),
		ActorOrgCode: nil,
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("Failed to create listing event: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("Failed to create listing: %v", err)
	}
	return listing, nil
}

func (s *Service) GetAllListings(ctx context.Context) ([]domain.Listing, error) {
	var listings []domain.Listing
	if err := s.DB.WithContext(ctx).Find(&listings).Error; err != nil {
		return nil, fmt.Errorf("Failed to fetch listings: %v", err)
	}
	return listings, nil
}

func (s *Service) GetOrgListings(ctx context.Context, orgID uuid.UUID) ([]domain.Listing, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Organization not associated with user")
	}
	var listings []domain.Listing
	if err := s.DB.WithContext(ctx).Where("seller_id = ?", orgID).Order(`"createdAt" DESC`).Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

// GetListingByIDResult matches Express getListingByIdService return shape: { listing_id, price_per_credit, credits_available, seller, project }.
type GetListingByIDResult struct {
	ListingID        uuid.UUID       `json:"listing_id"`
	PricePerCredit   float64         `json:"price_per_credit"`
	CreditsAvailable float64         `json:"credits_available"`
	Seller           GetListingSeller `json:"seller"`
	Project          *domain.IcrProject `json:"project"`
}

// GetListingSeller matches Express seller object: type 'org' | 'registry' | 'unknown'.
type GetListingSeller struct {
	Type    string  `json:"type"` // "org" | "registry" | "unknown"
	OrgID   *string `json:"org_id,omitempty"`
	OrgName *string `json:"org_name,omitempty"`
	OrgCode *string `json:"org_code,omitempty"`
	Name    *string `json:"name,omitempty"` // for type registry
}

func (s *Service) GetListingByID(ctx context.Context, listingID uuid.UUID) (*GetListingByIDResult, error) {
	if listingID == uuid.Nil {
		return nil, errors.New("listing_id is required")
	}
	var listing domain.Listing
	if err := s.DB.WithContext(ctx).Where("listing_id = ?", listingID).First(&listing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Listing not found")
		}
		return nil, err
	}
	var project domain.IcrProject
	if err := s.DB.WithContext(ctx).Where("id = ?", listing.ProjectID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Project not found for listing")
		}
		return nil, err
	}
	var seller GetListingSeller
	if listing.SellerID != nil {
		var org domain.Org
		if err := s.DB.WithContext(ctx).Where("org_id = ?", *listing.SellerID).First(&org).Error; err != nil {
			seller = GetListingSeller{Type: "unknown"}
		} else {
			oid, oname, ocode := org.OrgID.String(), org.OrgName, org.OrgCode
			seller = GetListingSeller{
				Type:    "org",
				OrgID:   &oid,
				OrgName: &oname,
				OrgCode: &ocode,
			}
		}
	} else {
		name := listing.Registry
		if name == "" {
			name = "Registry"
		}
		seller = GetListingSeller{Type: "registry", Name: &name}
	}
	return &GetListingByIDResult{
		ListingID:        listing.ListingID,
		PricePerCredit:   listing.PricePerCredit,
		CreditsAvailable: listing.CreditsAvailable,
		Seller:           seller,
		Project:          &project,
	}, nil
}

func (s *Service) GetAllActiveListings(ctx context.Context) ([]domain.Listing, error) {
	var listings []domain.Listing
	if err := s.DB.WithContext(ctx).Where("status = ?", "open").Order(`"createdAt" DESC`).Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetAllClosedListings(ctx context.Context) ([]domain.Listing, error) {
	var listings []domain.Listing
	if err := s.DB.WithContext(ctx).Where("status = ?", "closed").Order(`"updatedAt" DESC`).Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetOrgActiveListings(ctx context.Context, orgID uuid.UUID) ([]domain.Listing, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Org not found in session")
	}
	var listings []domain.Listing
	if err := s.DB.WithContext(ctx).Where("seller_id = ? AND status = ?", orgID, "open").Order(`"createdAt" DESC`).Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (s *Service) GetOrgClosedListings(ctx context.Context, orgID uuid.UUID) ([]domain.Listing, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("Org not found in session")
	}
	var listings []domain.Listing
	if err := s.DB.WithContext(ctx).Where("seller_id = ? AND status = ?", orgID, "closed").Order(`"updatedAt" DESC`).Find(&listings).Error; err != nil {
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

func (s *Service) EditListing(ctx context.Context, in EditListingInput) (*domain.Listing, error) {
	if in.ListingID == uuid.Nil {
		return nil, errors.New("Missing listing_id")
	}
	if in.OrgID == uuid.Nil {
		return nil, errors.New("Missing org_id")
	}

	var listing domain.Listing
	if err := s.DB.WithContext(ctx).Where("listing_id = ?", in.ListingID).First(&listing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("Listing not found")
		}
		return nil, err
	}
	if listing.Status != "open" {
		return nil, fmt.Errorf("Listing is not editable (status: %q). Only open listings can be edited", listing.Status)
	}
	if listing.SellerID == nil {
		return nil, errors.New("Registry listings cannot be edited by User")
	}
	if *listing.SellerID != in.OrgID {
		return nil, errors.New("Unauthorized listing edit")
	}

	updates := map[string]interface{}{}
	eventData := make(map[string]interface{})

	if in.NewPrice != nil {
		price := *in.NewPrice
		if math.IsNaN(price) || price <= 0 {
			return nil, errors.New("Invalid price")
		}
		if price != listing.PricePerCredit {
			updates["price_per_credit"] = price
			eventData["new_price_per_credit"] = price
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
			var holding domain.Holding
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
			if delta < 0 && -delta > currentQty {
				return nil, errors.New("Cannot reduce listing below already sold amount")
			}

			newLocked := locked + delta
			if newLocked < 0 {
				return nil, errors.New("Invalid locked_for_sale state")
			}

			updates["credits_available"] = qty
			eventData["quantity_delta"] = delta
			eventData["new_credits_available"] = qty
		}
	}

	if len(updates) == 0 {
		return nil, errors.New("No valid changes provided")
	}

	tx := s.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	// Apply holding locked_for_sale change if quantity changed (same tx as listing + event).
	if qty, ok := updates["credits_available"].(float64); ok {
		currentQty := listing.CreditsAvailable
		delta := qty - currentQty
		var holding domain.Holding
		if err := tx.Where("org_id = ? AND project_id = ?", in.OrgID, listing.ProjectID).First(&holding).Error; err != nil {
			tx.Rollback()
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New("Holdings not found")
			}
			return nil, err
		}
		newLocked := holding.LockedForSale + delta
		if err := tx.Model(&holding).Update("locked_for_sale", newLocked).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Model(&listing).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	var org domain.Org
	if err := tx.Where("org_id = ?", in.OrgID).Select("org_code").First(&org).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("Org not found")
	}
	eventDataBytes, _ := json.Marshal(eventData)
	if err := tx.Create(&domain.ListingEvent{
		ListingID:    listing.ListingID,
		EventType:    "UPDATED",
		ActorOrgCode: &org.OrgCode,
		EventData:    datatypes.JSON(eventDataBytes),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	s.DB.WithContext(ctx).Where("listing_id = ?", in.ListingID).First(&listing)
	return &listing, nil
}

func (s *Service) CancelListing(ctx context.Context, listingID, orgID uuid.UUID) (*domain.Listing, error) {
	var listing domain.Listing
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

	var holding domain.Holding
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

	tx := s.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Model(&holding).Update("locked_for_sale", newLocked).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	listing.Status = "closed"
	if err := tx.Save(&listing).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	var org domain.Org
	if err := tx.Where("org_id = ?", orgID).Select("org_code").First(&org).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("Org not found")
	}
	eventDataBytes, _ := json.Marshal(map[string]interface{}{"remaining_credits": listing.CreditsAvailable})
	if err := tx.Create(&domain.ListingEvent{
		ListingID:    listing.ListingID,
		EventType:    "CANCELLED",
		ActorOrgCode: &org.OrgCode,
		EventData:    datatypes.JSON(eventDataBytes),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &listing, nil
}
