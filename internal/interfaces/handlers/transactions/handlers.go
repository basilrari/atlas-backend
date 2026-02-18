package transactions

import (
	txsvc "troo-backend/internal/application/transactions"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type Handlers struct {
	Service *txsvc.Service
}

// GET /api/v1/transactions/get-transactions
func (h *Handlers) GetTransactions(c *fiber.Ctx) error {
	user := middleware.GetUser(c)
	if user == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	m, ok := user.(map[string]interface{})
	if !ok {
		return response.Error(c, "Authorization error", 500, nil)
	}
	orgIDStr, _ := m["org_id"].(string)

	data, errMsg, code := h.Service.ViewTransactions(c.Context(), orgIDStr)
	if errMsg != "" {
		return response.Error(c, errMsg, code, nil)
	}
	return response.Success(c, "Transactions fetched successfully", data, nil)
}
