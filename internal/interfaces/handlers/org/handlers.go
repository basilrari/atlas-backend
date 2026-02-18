package org

import (
	"encoding/json"

	orgsvc "troo-backend/internal/application/org"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handlers bundles org handlers with dependencies.
type Handlers struct {
	Service *orgsvc.Service
	Config  middleware.SessionConfig
}

// CreateOrg POST /api/v1/orgs/create-org
func (h *Handlers) CreateOrg(c *fiber.Ctx) error {
	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return response.Error(c, "org_name and country_code are required", 400, nil)
	}

	orgName, _ := body["org_name"].(string)
	countryCode, _ := body["country_code"].(string)
	if orgName == "" || countryCode == "" {
		return response.Error(c, "org_name and country_code are required", 400, nil)
	}

	actor := middleware.GetUser(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	m, ok := actor.(map[string]interface{})
	if !ok {
		return response.Error(c, "Authorization error", 500, nil)
	}
	actorIDStr, _ := m["user_id"].(string)
	if actorIDStr == "" {
		return response.Unauthorized(c, "Unauthorized")
	}
	actorID, err := uuid.Parse(actorIDStr)
	if err != nil {
		return response.Error(c, "Authorization error", 500, nil)
	}

	var regIDPtr, logoPtr, docPtr *string
	if v, ok := body["registration_id"].(string); ok {
		regIDPtr = &v
	}
	if v, ok := body["logo_url"].(string); ok {
		logoPtr = &v
	}
	if v, ok := body["incorporation_doc_url"].(string); ok {
		docPtr = &v
	}

	org, err := h.Service.CreateOrg(c.Context(), orgsvc.CreateOrgInput{
		OrgName:             orgName,
		CountryCode:         countryCode,
		RegistrationID:      regIDPtr,
		LogoURL:             logoPtr,
		IncorporationDocURL: docPtr,
	}, actorID)
	if err != nil {
		if err.Error() == "org_name and country_code are required" {
			return response.Error(c, err.Error(), 400, nil)
		}
		return response.Error(c, "Internal Server Error", 500, nil)
	}

	// Regenerate session because role privilege changed (Express: set org_id + role superadmin)
	sessionID := middleware.RegenerateSessionID(c)
	orgIDStr := org.OrgID.String()
	fullname, _ := m["fullname"].(string)
	email, _ := m["email"].(string)
	middleware.SetSessionUser(c, middleware.SessionUser{
		UserID:   actorIDStr,
		Fullname: fullname,
		Email:    email,
		Role:     "superadmin",
		OrgID:    &orgIDStr,
	})

	cookie := middleware.SessionCookieConfig(h.Config)
	cookie.Value = "s:" + sessionID
	c.Cookie(&cookie)

	// User asked for 201 on create endpoints; use SuccessCreated
	return response.SuccessCreated(c, "Organization created successfully", org, nil)
}

// ViewOrg GET /api/v1/orgs/view-org
func (h *Handlers) ViewOrg(c *fiber.Ctx) error {
	actor := middleware.GetUser(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	m, ok := actor.(map[string]interface{})
	if !ok {
		return response.Error(c, "Authorization error", 500, nil)
	}
	orgIDStr, _ := m["org_id"].(string)
	if orgIDStr == "" {
		return response.Error(c, "User is not associated with any organization", 403, nil)
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return response.Error(c, "User is not associated with any organization", 403, nil)
	}

	org, err := h.Service.GetOrgByID(c.Context(), orgID)
	if err != nil {
		switch err.Error() {
		case "Missing org_id":
			return response.Error(c, err.Error(), 400, nil)
		case "Org not found":
			return response.Error(c, err.Error(), 404, nil)
		default:
			return response.Error(c, "Internal Server Error", 500, nil)
		}
	}
	return response.Success(c, "Organization fetched successfully", org, nil)
}

// UpdateOrg PATCH /api/v1/orgs/update-org/:id
func (h *Handlers) UpdateOrg(c *fiber.Ctx) error {
	idStr := c.Params("id")
	if idStr == "" {
		return response.Error(c, "Missing org_id", 400, nil)
	}
	orgID, err := uuid.Parse(idStr)
	if err != nil {
		// Express doesn't validate UUID format; treat as missing/invalid org
		return response.Error(c, "Missing org_id", 400, nil)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return response.Error(c, "No update fields provided", 400, nil)
	}

	org, err := h.Service.UpdateOrg(c.Context(), orgID, body)
	if err != nil {
		switch err.Error() {
		case "Missing org_id", "No update fields provided", "No valid fields to update":
			return response.Error(c, err.Error(), 400, nil)
		case "Org not found":
			return response.Error(c, err.Error(), 404, nil)
		default:
			return response.Error(c, "Internal Server Error", 500, nil)
		}
	}
	return response.Success(c, "Organization updated successfully", org, nil)
}
