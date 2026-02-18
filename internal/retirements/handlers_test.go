package retirements

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"troo-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRetirementTest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RetirementCertificate{}))
	svc := &Service{DB: db}
	h := &Handlers{Service: svc}
	return h, db
}

func TestViewOrg_MissingOrgID(t *testing.T) {
	h, _ := setupRetirementTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  "",
		})
		return c.Next()
	})
	app.Get("/view-org", h.ViewOrg)

	req := httptest.NewRequest("GET", "/view-org", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestViewOrg_Success(t *testing.T) {
	h, db := setupRetirementTest(t)
	orgID := uuid.New()
	require.NoError(t, db.Create(&models.RetirementCertificate{
		CertificateID: uuid.New(), OrgID: orgID, ProjectID: uuid.New(),
		Amount: 100, RetiredAt: time.Now(), TransactionID: uuid.New(),
		CertificateNumber: "CERT-TEST-1", Status: "issued",
	}).Error)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  orgID.String(),
		})
		return c.Next()
	})
	app.Get("/view-org", h.ViewOrg)

	req := httptest.NewRequest("GET", "/view-org", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestViewOne_MissingID(t *testing.T) {
	h, _ := setupRetirementTest(t)
	app := fiber.New()
	app.Post("/view-one", h.ViewOne)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/view-one", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestViewOne_InvalidUUID(t *testing.T) {
	h, _ := setupRetirementTest(t)
	app := fiber.New()
	app.Post("/view-one", h.ViewOne)

	body, _ := json.Marshal(map[string]string{"certificate_id": "not-a-uuid"})
	req := httptest.NewRequest("POST", "/view-one", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestViewOne_NotFound(t *testing.T) {
	h, _ := setupRetirementTest(t)
	app := fiber.New()
	app.Post("/view-one", h.ViewOne)

	body, _ := json.Marshal(map[string]string{"certificate_id": uuid.New().String()})
	req := httptest.NewRequest("POST", "/view-one", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}
