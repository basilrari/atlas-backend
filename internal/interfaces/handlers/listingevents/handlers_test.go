package listingevents

import (
	"net/http/httptest"
	"testing"

	lesvc "troo-backend/internal/application/listingevents"
	"troo-backend/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLETest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Org{}, &domain.ListingEvent{}))
	svc := &lesvc.Service{DB: db}
	h := &Handlers{Service: svc}
	return h, db
}

func TestGetOrgListingEvents_NoOrg(t *testing.T) {
	h, _ := setupLETest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  "",
		})
		return c.Next()
	})
	app.Get("/get-org-listing-events", h.GetOrgListingEvents)

	req := httptest.NewRequest("GET", "/get-org-listing-events", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestGetOrgListingEvents_OrgNotFound(t *testing.T) {
	h, _ := setupLETest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  uuid.New().String(),
		})
		return c.Next()
	})
	app.Get("/get-org-listing-events", h.GetOrgListingEvents)

	req := httptest.NewRequest("GET", "/get-org-listing-events", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestGetOrgListingEvents_Success(t *testing.T) {
	h, db := setupLETest(t)
	orgID := uuid.New()
	require.NoError(t, db.Create(&domain.Org{
		OrgID: orgID, OrgName: "TestOrg", OrgCode: "TO-123456", CountryCode: "US",
	}).Error)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": uuid.New().String(),
			"org_id":  orgID.String(),
		})
		return c.Next()
	})
	app.Get("/get-org-listing-events", h.GetOrgListingEvents)

	req := httptest.NewRequest("GET", "/get-org-listing-events", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
