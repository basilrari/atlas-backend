package transactions

import (
	"context"

	"troo-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	DB *gorm.DB
}

type FormattedTx struct {
	TxID             uuid.UUID   `json:"tx_id"`
	Type             string      `json:"type"`
	Amount           float64     `json:"amount"`
	CreatedAt        interface{} `json:"created_at"`
	FromOrgCode      *string     `json:"from_org_code"`
	ToOrgCode        *string     `json:"to_org_code"`
	ProjectID        uuid.UUID   `json:"project_id"`
	ProjectName      *string     `json:"project_name"`
	ProjectThumbnail *string     `json:"project_thumbnail"`
}

func (s *Service) ViewTransactions(ctx context.Context, orgID string) (interface{}, string, int) {
	if orgID == "" {
		return nil, "org_id missing from session", 401
	}

	var txs []domain.Transaction
	if err := s.DB.WithContext(ctx).
		Where("from_org_id = ? OR to_org_id = ?", orgID, orgID).
		Order(`"createdAt" DESC`).
		Find(&txs).Error; err != nil {
		return nil, "Internal Server Error", 500
	}

	if len(txs) == 0 {
		return []interface{}{}, "", 0
	}

	orgIDs := map[uuid.UUID]bool{}
	projIDs := map[uuid.UUID]bool{}
	for _, tx := range txs {
		if tx.FromOrgID != nil {
			orgIDs[*tx.FromOrgID] = true
		}
		if tx.ToOrgID != nil {
			orgIDs[*tx.ToOrgID] = true
		}
		projIDs[tx.ProjectID] = true
	}

	orgCodeMap := map[string]string{}
	if len(orgIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(orgIDs))
		for id := range orgIDs {
			ids = append(ids, id)
		}
		var orgs []domain.Org
		s.DB.WithContext(ctx).Where("org_id IN ?", ids).Select("org_id, org_code").Find(&orgs)
		for _, o := range orgs {
			orgCodeMap[o.OrgID.String()] = o.OrgCode
		}
	}

	projMap := map[string]struct {
		Name      *string
		Thumbnail *string
	}{}
	if len(projIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(projIDs))
		for id := range projIDs {
			ids = append(ids, id)
		}
		var projs []domain.IcrProject
		s.DB.WithContext(ctx).Where("id IN ?", ids).Select(`id, "fullName", thumbnail`).Find(&projs)
		for _, p := range projs {
			projMap[p.ID.String()] = struct {
				Name      *string
				Thumbnail *string
			}{Name: p.FullName, Thumbnail: p.Thumbnail}
		}
	}

	out := make([]FormattedTx, len(txs))
	for i, tx := range txs {
		ft := FormattedTx{
			TxID:      tx.TxID,
			Type:      tx.Type,
			Amount:    tx.Amount,
			CreatedAt: tx.CreatedAt,
			ProjectID: tx.ProjectID,
		}
		if tx.FromOrgID != nil {
			if code, ok := orgCodeMap[tx.FromOrgID.String()]; ok {
				ft.FromOrgCode = &code
			}
		}
		if tx.ToOrgID != nil {
			if code, ok := orgCodeMap[tx.ToOrgID.String()]; ok {
				ft.ToOrgCode = &code
			}
		}
		if p, ok := projMap[tx.ProjectID.String()]; ok {
			ft.ProjectName = p.Name
			ft.ProjectThumbnail = p.Thumbnail
		}
		out[i] = ft
	}

	return out, "", 0
}
