package listings

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	listsvc "troo-backend/internal/application/listings"
	"troo-backend/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupListingsTest(t *testing.T) (*Handlers, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Listing{}, &domain.Holding{}))
	svc := &listsvc.Service{DB: db}
	h := &Handlers{Service: svc}
	return h, db
}

func TestCreateListing_MissingField(t *testing.T) {
	h, _ := setupListingsTest(t)
	app := fiber.New()
	app.Post("/create-listing", h.CreateListing)

	body, _ := json.Marshal(map[string]interface{}{
		"credits_available":  100,
		"price_per_credit":   10,
		"project_name":       "Test",
		"project_start_year": 2020,
		"registry":           "Registry",
		"category":           "Cat",
		"location_city":      "City",
		"location_state":     "State",
		"location_country":   "Country",
		"thumbnail_url":      "http://example.com",
		"methodology":        "Meth",
	})

	req := httptest.NewRequest("POST", "/create-listing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, false, result["success"])
	assert.Equal(t, "Missing required field: project_id", result["message"])
}

func TestGetAllListings_EmptyDB(t *testing.T) {
	h, _ := setupListingsTest(t)
	app := fiber.New()
	app.Get("/get-all-listings", h.GetAllListings)

	req := httptest.NewRequest("GET", "/get-all-listings", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Listings fetched successfully", result["message"])
}

func TestGetListingByID_InvalidUUID(t *testing.T) {
	h, _ := setupListingsTest(t)
	app := fiber.New()
	app.Get("/get-listing/:listing_id", h.GetListingByID)

	req := httptest.NewRequest("GET", "/get-listing/not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestEditListing_MissingFields(t *testing.T) {
	h, _ := setupListingsTest(t)
	app := fiber.New()
	app.Put("/edit-listing", h.EditListing)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("PUT", "/edit-listing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestCancelListing_InvalidID(t *testing.T) {
	h, _ := setupListingsTest(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{
			"user_id": "00000000-0000-0000-0000-000000000001",
			"role":    "admin",
			"email":   "admin@test.com",
			"org_id":  "00000000-0000-0000-0000-000000000002",
		})
		return c.Next()
	})
	app.Post("/cancel-listing", h.CancelListing)

	body, _ := json.Marshal(map[string]string{"listing_id": "not-a-uuid"})
	req := httptest.NewRequest("POST", "/cancel-listing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}
