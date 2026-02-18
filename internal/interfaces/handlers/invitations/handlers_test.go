package invitations

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	invsvc "troo-backend/internal/application/invitations"
	"troo-backend/internal/domain"
	"troo-backend/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvitationsTest(t *testing.T) (*Handlers, *invsvc.Service, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.User{}, &domain.Org{}, &domain.Invitation{}))
	svc := &invsvc.Service{DB: db}
	h := &Handlers{
		Service: svc,
		Config:  middleware.SessionConfig{AllowCrossSiteDev: false, IsProduction: false},
	}
	return h, svc, db
}

func TestCheckToken_MissingToken(t *testing.T) {
	h, _, _ := setupInvitationsTest(t)
	app := fiber.New()
	app.Post("/check-token", h.CheckToken)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/check-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestCheckToken_InvalidToken(t *testing.T) {
	h, _, _ := setupInvitationsTest(t)
	app := fiber.New()
	app.Post("/check-token", h.CheckToken)

	body, _ := json.Marshal(map[string]string{"token": "nonexistent-token"})
	req := httptest.NewRequest("POST", "/check-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestCheckToken_ValidToken(t *testing.T) {
	h, _, db := setupInvitationsTest(t)

	orgID := uuid.New()
	require.NoError(t, db.Create(&domain.Org{
		OrgID: orgID, OrgName: "TestOrg", OrgCode: "TO-123456", CountryCode: "US",
	}).Error)

	require.NoError(t, db.Create(&domain.Invitation{
		InviteID: uuid.New(), OrgID: orgID, Email: "inv@test.com", Role: "viewer",
		InviteToken: "valid-token-abc", Status: "pending",
		CreatedBy: "someone", ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}).Error)

	app := fiber.New()
	app.Post("/check-token", h.CheckToken)

	body, _ := json.Marshal(map[string]string{"token": "valid-token-abc"})
	req := httptest.NewRequest("POST", "/check-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestSendInvite_MissingFields(t *testing.T) {
	h, _, _ := setupInvitationsTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(), "role": "admin", "email": "admin@test.com",
			"fullname": "Admin", "org_id": uuid.New().String(),
		})
		return c.Next()
	})
	app.Post("/create-invite", h.SendInvite)

	body, _ := json.Marshal(map[string]string{"email": "test@test.com"})
	req := httptest.NewRequest("POST", "/create-invite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestSendInvite_SelfInvite(t *testing.T) {
	h, _, db := setupInvitationsTest(t)

	orgID := uuid.New()
	require.NoError(t, db.Create(&domain.Org{
		OrgID: orgID, OrgName: "TestOrg", OrgCode: "TO-123456", CountryCode: "US",
	}).Error)

	actorEmail := "admin@test.com"

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		oid := orgID.String()
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(), "role": "superadmin", "email": actorEmail,
			"fullname": "Admin", "org_id": oid,
		})
		return c.Next()
	})
	app.Post("/create-invite", h.SendInvite)

	body, _ := json.Marshal(map[string]string{"email": actorEmail, "role": "viewer"})
	req := httptest.NewRequest("POST", "/create-invite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	errObj, _ := result["error"].(map[string]interface{})
	assert.Equal(t, "You cannot invite yourself", errObj["message"])
}

func TestRevokeInvite_MissingEmail(t *testing.T) {
	h, _, _ := setupInvitationsTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(), "role": "admin", "email": "admin@test.com",
			"fullname": "Admin", "org_id": uuid.New().String(),
		})
		return c.Next()
	})
	app.Patch("/revoke-invite", h.RevokeInvite)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("PATCH", "/revoke-invite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}
