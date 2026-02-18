package invitations

import (
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type Handlers struct {
	Service *Service
	Config  middleware.SessionConfig
}

// POST /api/v1/invitations/create-invite (INVITE_USER permission via middleware)
func (h *Handlers) SendInvite(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Role == "" {
		return response.Error(c, "Email and role are required", 400, nil)
	}

	actor := getActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	if actor.OrgID == "" {
		return response.Error(c, "User is not associated with any organization", 403, nil)
	}

	inv, err := h.Service.SendInvite(c.Context(), SendInviteInput{
		ActorUserID: actor.UserID,
		ActorRole:   actor.Role,
		ActorEmail:  actor.Email,
		OrgID:       actor.OrgID,
		Email:       body.Email,
		Role:        body.Role,
	})
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "Invitation sent successfully", inv, nil)
}

// POST /api/v1/invitations/resend-invite (INVITE_USER permission via middleware)
func (h *Handlers) ResendInvite(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return response.Error(c, "Email is required", 400, nil)
	}

	actor := getActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}

	inv, err := h.Service.ResendInvite(c.Context(), ResendInviteInput{
		Email: body.Email,
		OrgID: actor.OrgID,
	})
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "Invitation resent successfully", inv, nil)
}

// POST /api/v1/invitations/accept-invite (no extra permission, just auth)
func (h *Handlers) AcceptInvite(c *fiber.Ctx) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return response.Error(c, "Invitation token required", 400, nil)
	}

	actor := getActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}

	result, err := h.Service.AcceptInvite(c.Context(), AcceptInviteInput{
		Token:  body.Token,
		UserID: actor.UserID,
	})
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}

	// Regenerate session (Express: req.session.regenerate, set org_id + role)
	sid := middleware.RegenerateSessionID(c)
	middleware.SetSessionUser(c, middleware.SessionUser{
		UserID:   actor.UserID,
		Fullname: actor.Fullname,
		Email:    actor.Email,
		Role:     result.Role,
		OrgID:    &result.OrgID,
	})
	cookie := middleware.SessionCookieConfig(h.Config)
	cookie.Value = "s:" + sid
	c.Cookie(&cookie)

	return response.Success(c, "Invitation accepted successfully", result, nil)
}

// PATCH /api/v1/invitations/revoke-invite (INVITE_USER permission via middleware)
func (h *Handlers) RevokeInvite(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return response.Error(c, "Email is required", 400, nil)
	}

	actor := getActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}

	inv, err := h.Service.RevokeInvite(c.Context(), RevokeInviteInput{
		Email: body.Email,
		OrgID: actor.OrgID,
	})
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "Invitation revoked successfully", inv, nil)
}

// GET /api/v1/invitations/view-invites (VIEW_DATA permission via middleware)
func (h *Handlers) ListOrgInvitations(c *fiber.Ctx) error {
	actor := getActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}

	status := c.Query("status")
	invitations, err := h.Service.ListOrgInvitations(c.Context(), ListInvitesInput{
		OrgID:  actor.OrgID,
		Status: status,
	})
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "Invitations fetched successfully", invitations, nil)
}

// POST /api/v1/invitations/public/check-token (no auth)
func (h *Handlers) CheckToken(c *fiber.Ctx) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return response.Error(c, "token is required", 400, nil)
	}

	result, err := h.Service.CheckInvitationToken(c.Context(), body.Token)
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "Invitation token verified", result, nil)
}

type actorInfo struct {
	UserID   string
	Fullname string
	Email    string
	Role     string
	OrgID    string
}

func getActor(c *fiber.Ctx) *actorInfo {
	u := middleware.GetUser(c)
	if u == nil {
		return nil
	}
	m, ok := u.(map[string]interface{})
	if !ok {
		return nil
	}
	userID, _ := m["user_id"].(string)
	role, _ := m["role"].(string)
	email, _ := m["email"].(string)
	fullname, _ := m["fullname"].(string)
	if userID == "" {
		return nil
	}
	orgID := ""
	if o, ok := m["org_id"]; ok && o != nil {
		if s, ok := o.(string); ok {
			orgID = s
		}
	}
	return &actorInfo{UserID: userID, Fullname: fullname, Email: email, Role: role, OrgID: orgID}
}
