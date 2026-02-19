package listings

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	listsvc "troo-backend/internal/application/listings"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handlers struct {
	Service *listsvc.Service
}

// POST /api/v1/listings/create-listing — 201 with { success, message, data }
func (h *Handlers) CreateListing(c *fiber.Ctx) error {
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	required := []string{
		"project_id", "credits_available", "price_per_credit", "project_name",
		"project_start_year", "registry", "category", "location_city",
		"location_state", "location_country", "thumbnail_url", "methodology",
	}
	for _, f := range required {
		if body[f] == nil || body[f] == "" {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Missing required field: %s", f)})
		}
	}

	projectID, err := uuid.Parse(asString(body["project_id"]))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Missing required field: project_id"})
	}

	var sellerID *uuid.UUID
	if s := asString(body["seller_id"]); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			sellerID = &id
		}
	}
	var extID *string
	if s := asString(body["external_trade_id"]); s != "" {
		extID = &s
	}

	listing, err := h.Service.CreateListing(c.Context(), listsvc.CreateListingInput{
		ProjectID:        projectID,
		SellerID:         sellerID,
		CreditsAvailable: asFloat(body["credits_available"]),
		PricePerCredit:   asFloat(body["price_per_credit"]),
		ExternalTradeID:  extID,
		ProjectName:      asString(body["project_name"]),
		ProjectStartYear: asInt(body["project_start_year"]),
		Registry:         asString(body["registry"]),
		Category:         asString(body["category"]),
		LocationCity:     asString(body["location_city"]),
		LocationState:    asString(body["location_state"]),
		LocationCountry:  asString(body["location_country"]),
		ThumbnailURL:     asString(body["thumbnail_url"]),
		Status:           asStringDef(body["status"], "open"),
		SdgNumbers:       asJSONSliceString(body["sdg_numbers"], "[]"),
		Methodology:      asString(body["methodology"]),
		VintageYear:      asInt(body["vintage_year"]),
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{"success": true, "message": "Listing created successfully", "data": listing})
}

// GET /api/v1/listings/get-all-listings — { success, message, data }
func (h *Handlers) GetAllListings(c *fiber.Ctx) error {
	listings, err := h.Service.GetAllListings(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Listings fetched successfully", "data": listings})
}

// GET /api/v1/listings/get-org-listings — standard res.success/res.error
func (h *Handlers) GetOrgListings(c *fiber.Ctx) error {
	orgID, err := actorOrgID(c)
	if err != nil {
		return response.Error(c, err.Error(), 403, nil)
	}
	data, err := h.Service.GetOrgListings(c.Context(), orgID)
	if err != nil {
		if err.Error() == "Organization not associated with user" {
			return response.Error(c, err.Error(), 403, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Organization listings fetched successfully", data, nil)
}

// GET /api/v1/listings/get-listing/:listing_id
func (h *Handlers) GetListingByID(c *fiber.Ctx) error {
	idStr := c.Params("listing_id")
	if idStr == "" {
		return response.Error(c, "listing_id is required", 400, nil)
	}
	listingID, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "Invalid listing_id format", 400, nil)
	}
	listing, err := h.Service.GetListingByID(c.Context(), listingID)
	if err != nil {
		switch err.Error() {
		case "listing_id is required":
			return response.Error(c, err.Error(), 400, nil)
		case "Listing not found":
			return response.Error(c, err.Error(), 404, nil)
		default:
			return response.Error(c, "Internal Server Error", 500, nil)
		}
	}
	return response.Success(c, "Listing fetched successfully", listing, nil)
}

// GET /api/v1/listings/get-all-active-listings
func (h *Handlers) GetAllActiveListings(c *fiber.Ctx) error {
	listings, err := h.Service.GetAllActiveListings(c.Context())
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Active listings fetched", listings, nil)
}

// GET /api/v1/listings/get-all-closed-listings
func (h *Handlers) GetAllClosedListings(c *fiber.Ctx) error {
	listings, err := h.Service.GetAllClosedListings(c.Context())
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Closed listings fetched", listings, nil)
}

// GET /api/v1/listings/get-org-active-listings
func (h *Handlers) GetOrgActiveListings(c *fiber.Ctx) error {
	orgID, err := actorOrgID(c)
	if err != nil {
		return response.Error(c, "User not linked to org", 403, nil)
	}
	listings, err := h.Service.GetOrgActiveListings(c.Context(), orgID)
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Active org listings fetched", listings, nil)
}

// GET /api/v1/listings/get-org-closed-listings
func (h *Handlers) GetOrgClosedListings(c *fiber.Ctx) error {
	orgID, err := actorOrgID(c)
	if err != nil {
		return response.Error(c, "User not linked to org", 403, nil)
	}
	listings, err := h.Service.GetOrgClosedListings(c.Context(), orgID)
	if err != nil {
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Closed org listings fetched", listings, nil)
}

// PUT /api/v1/listings/edit-listing
func (h *Handlers) EditListing(c *fiber.Ctx) error {
	var body struct {
		ListingID string   `json:"listing_id"`
		Price     *float64 `json:"price"`
		Quantity  *float64 `json:"quantity"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "listing_id, price and quantity are required", 400, nil)
	}
	if body.ListingID == "" || body.Price == nil || body.Quantity == nil {
		return response.Error(c, "listing_id, price and quantity are required", 400, nil)
	}
	orgID, err := actorOrgID(c)
	if err != nil {
		return response.Error(c, "User is not associated with an organization", 403, nil)
	}
	listingID, err := uuid.Parse(body.ListingID)
	if err != nil {
		return response.Error(c, "Invalid listing_id", 400, nil)
	}

	result, err := h.Service.EditListing(c.Context(), listsvc.EditListingInput{
		ListingID:   listingID,
		OrgID:       orgID,
		NewPrice:    body.Price,
		NewQuantity: body.Quantity,
	})
	if err != nil {
		statusMap := map[string]int{
			"Missing listing_id":                          400,
			"Invalid listing_id":                          400,
			"Invalid price":                               400,
			"Invalid quantity":                             400,
			"No valid changes provided":                    400,
			"Listing not found":                            404,
			"Unauthorized listing edit":                    403,
			"Registry listings cannot be edited by User":   403,
			"Insufficient credits to increase listing":     400,
			"Holdings not found":                           404,
			"Org not found":                                404,
		}
		if code, ok := statusMap[err.Error()]; ok {
			return response.Error(c, err.Error(), code, nil)
		}
		if strings.Contains(err.Error(), "Listing is not editable") {
			return response.Error(c, err.Error(), 400, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Listing updated successfully", result, nil)
}

// POST /api/v1/listings/cancel-listing
func (h *Handlers) CancelListing(c *fiber.Ctx) error {
	var body struct {
		ListingID string `json:"listing_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.ListingID == "" {
		return response.Error(c, "Invalid listing_id", 400, nil)
	}
	orgID, err := actorOrgID(c)
	if err != nil {
		return response.Error(c, "User not associated with organization", 403, nil)
	}
	listingID, err := uuid.Parse(body.ListingID)
	if err != nil {
		return response.Error(c, "Invalid listing_id", 400, nil)
	}

	result, err := h.Service.CancelListing(c.Context(), listingID, orgID)
	if err != nil {
		statusMap := map[string]int{
			"Listing not found":                   404,
			"Listing is not open":                 400,
			"Registry listings cannot be cancelled": 403,
			"Unauthorized":                        403,
			"Holdings not found":                  404,
		}
		if code, ok := statusMap[err.Error()]; ok {
			return response.Error(c, err.Error(), code, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}
	return response.Success(c, "Listing cancelled successfully", result, nil)
}

// --- helpers ---

func actorOrgID(c *fiber.Ctx) (uuid.UUID, error) {
	user := middleware.GetUser(c)
	if user == nil {
		return uuid.Nil, fmt.Errorf("User is not associated with any organization")
	}
	m, ok := user.(map[string]interface{})
	if !ok {
		return uuid.Nil, fmt.Errorf("User is not associated with any organization")
	}
	orgIDStr, _ := m["org_id"].(string)
	if orgIDStr == "" {
		return uuid.Nil, fmt.Errorf("User is not associated with any organization")
	}
	return uuid.Parse(orgIDStr)
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func asStringDef(v interface{}, def string) string {
	s := asString(v)
	if s == "" {
		return def
	}
	return s
}

// asJSONSliceString returns a JSON string for the sdg_numbers column (Postgres json type). Accepts string or slice.
func asJSONSliceString(v interface{}, def string) string {
	if v == nil {
		return def
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return def
		}
		return s
	}
	// Slice/array from JSON body (e.g. [7, 13]) — marshal to valid JSON
	bs, err := json.Marshal(v)
	if err != nil {
		return def
	}
	return string(bs)
}

func asFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func asInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		i, _ := strconv.Atoi(x)
		return i
	}
	return 0
}
