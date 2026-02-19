package trading

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"troo-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	DB *gorm.DB
}

// SellCredits mirrors Express sellCreditsService (transactional).
func (s *Service) SellCredits(ctx context.Context, orgID, projectID uuid.UUID, amount, price float64) (map[string]interface{}, error) {
	var result map[string]interface{}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var org domain.Org
		if err := tx.Where("org_id = ?", orgID).First(&org).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("Org not found")
			}
			return err
		}

		var holding domain.Holding
		if err := tx.Where("org_id = ? AND project_id = ?", orgID, projectID).First(&holding).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("No holdings found for this project")
			}
			return err
		}

		available := holding.CreditBalance - holding.LockedForSale
		if available < amount {
			return errors.New("Insufficient credits to sell")
		}

		var project domain.IcrProject
		if err := tx.Where("id = ?", projectID).First(&project).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("Project not found")
			}
			return err
		}

		var existingListing domain.Listing
		err := tx.Where("seller_id = ? AND project_id = ? AND price_per_credit = ? AND status = ?", orgID, projectID, price, "open").First(&existingListing).Error

		if err == nil {
			existingListing.CreditsAvailable = math.Round((existingListing.CreditsAvailable+amount)*100) / 100
			if err := tx.Save(&existingListing).Error; err != nil {
				return err
			}
			holding.LockedForSale = math.Round((holding.LockedForSale+amount)*100) / 100
			if err := tx.Save(&holding).Error; err != nil {
				return err
			}
			eventDataBytes, _ := json.Marshal(map[string]interface{}{
				"credits_added":         amount,
				"new_credits_available": existingListing.CreditsAvailable,
				"price_per_credit":      existingListing.PricePerCredit,
			})
			if err := tx.Create(&domain.ListingEvent{
				ListingID:    existingListing.ListingID,
				EventType:    "UPDATED",
				ActorOrgCode: &org.OrgCode,
				EventData:    datatypes.JSON(eventDataBytes),
			}).Error; err != nil {
				return err
			}
			result = map[string]interface{}{
				"listing_id":        existingListing.ListingID,
				"credits_available": existingListing.CreditsAvailable,
			}
			return nil
		}

		holding.LockedForSale = math.Round((holding.LockedForSale+amount)*100) / 100
		if err := tx.Save(&holding).Error; err != nil {
			return err
		}

		projectName := ""
		if project.FullName != nil {
			projectName = *project.FullName
		}

		listing := domain.Listing{
			SellerID:         &orgID,
			ProjectID:        projectID,
			CreditsAvailable: amount,
			PricePerCredit:   price,
			Status:           "open",
			ProjectName:      projectName,
			Registry:         project.Registry,
			ThumbnailURL:     safeStr(project.Thumbnail),
			LocationCity:     safeStr(project.City),
			LocationState:    safeStr(project.State),
			LocationCountry:  safeStr(project.CountryCode),
			Methodology:      "N/A",
			Category:         "N/A",
			SdgNumbers:       "[]", // Postgres json column requires valid JSON
		}

		if err := tx.Create(&listing).Error; err != nil {
			return err
		}
		eventDataBytes, _ := json.Marshal(map[string]interface{}{
			"credits_available": listing.CreditsAvailable,
			"price_per_credit":   listing.PricePerCredit,
		})
		if err := tx.Create(&domain.ListingEvent{
			ListingID:    listing.ListingID,
			EventType:    "CREATED",
			ActorOrgCode: &org.OrgCode,
			EventData:    datatypes.JSON(eventDataBytes),
		}).Error; err != nil {
			return err
		}
		result = map[string]interface{}{
			"listing_id":        listing.ListingID,
			"credits_available": listing.CreditsAvailable,
		}
		return nil
	})

	return result, err
}

// TransferCredits mirrors Express transferCreditsService (transactional).
func (s *Service) TransferCredits(ctx context.Context, fromOrgID, projectID uuid.UUID, toOrgCode string, amount float64) (map[string]interface{}, error) {
	var result map[string]interface{}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var toOrg domain.Org
		if err := tx.Where("org_code = ?", toOrgCode).First(&toOrg).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("Target organization not found")
			}
			return err
		}

		if fromOrgID == toOrg.OrgID {
			return errors.New("Cannot transfer to the same organization")
		}

		var sender domain.Holding
		if err := tx.Where("org_id = ? AND project_id = ?", fromOrgID, projectID).First(&sender).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("No Holdings found for this project")
			}
			return err
		}

		available := sender.CreditBalance - sender.LockedForSale
		if available < amount {
			return errors.New("Insufficient available credits to transfer")
		}

		sender.CreditBalance = math.Round((sender.CreditBalance-amount)*100) / 100
		if err := tx.Save(&sender).Error; err != nil {
			return err
		}

		var receiver domain.Holding
		err := tx.Where("org_id = ? AND project_id = ?", toOrg.OrgID, projectID).First(&receiver).Error
		if err == gorm.ErrRecordNotFound {
			receiver = domain.Holding{
				OrgID:         toOrg.OrgID,
				ProjectID:     projectID,
				CreditBalance: amount,
			}
			if err := tx.Create(&receiver).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			receiver.CreditBalance = math.Round((receiver.CreditBalance+amount)*100) / 100
			if err := tx.Save(&receiver).Error; err != nil {
				return err
			}
		}

		txRecord := domain.Transaction{
			Type:      "transfer",
			FromOrgID: &fromOrgID,
			ToOrgID:   &toOrg.OrgID,
			ProjectID: projectID,
			Amount:    amount,
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return err
		}

		result = map[string]interface{}{
			"transferred":  amount,
			"to_org_code":  toOrgCode,
		}
		return nil
	})

	return result, err
}

// RetireCredits mirrors Express retireCreditsService (transactional).
func (s *Service) RetireCredits(ctx context.Context, orgID, projectID uuid.UUID, amount float64, purpose, beneficiary *string) (map[string]interface{}, error) {
	var result map[string]interface{}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var holding domain.Holding
		if err := tx.Where("org_id = ? AND project_id = ?", orgID, projectID).First(&holding).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("No holdings found")
			}
			return err
		}

		available := holding.CreditBalance - holding.LockedForSale
		if available < amount {
			return errors.New("Insufficient available credits to retire")
		}

		holding.CreditBalance = math.Round((holding.CreditBalance-amount)*100) / 100
		if err := tx.Save(&holding).Error; err != nil {
			return err
		}

		txRecord := domain.Transaction{
			Type:      "retire",
			FromOrgID: &orgID,
			ProjectID: projectID,
			Amount:    amount,
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return err
		}

		cert := domain.RetirementCertificate{
			OrgID:             orgID,
			ProjectID:         projectID,
			Amount:            amount,
			RetiredAt:         time.Now(),
			Purpose:           purpose,
			Beneficiary:       beneficiary,
			TransactionID:     txRecord.TxID,
			CertificateNumber: fmt.Sprintf("CERT-%d", time.Now().UnixMilli()),
			Status:            "issued",
		}
		if err := tx.Create(&cert).Error; err != nil {
			return err
		}

		result = map[string]interface{}{
			"message":            "Credits retired successfully",
			"certificate_id":     cert.CertificateID,
			"certificate_number": cert.CertificateNumber,
			"retired_amount":     amount,
		}
		return nil
	})

	return result, err
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
