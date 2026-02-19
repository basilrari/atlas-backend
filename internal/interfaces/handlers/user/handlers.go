package user

import (
	usersvc "troo-backend/internal/application/user"
	"troo-backend/internal/domain"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const userSessionsPrefix = "user_sessions:"

// Handlers holds the user service and session config for create-user (session + cookie).
type Handlers struct {
	Service *usersvc.Service
	Config  middleware.SessionConfig
}

// CreateUserRequest body (Express: user_name, email, password, fullname).
type CreateUserRequest struct {
	UserName string `json:"user_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Fullname string `json:"fullname"`
}

// CreateUser POST /api/v1/users/create-user — create user, regenerate session, SAdd user_sessions, set cookie, return 201 with data.user.
func (h *Handlers) CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "Missing required fields", 400, nil)
	}
	if req.UserName == "" || req.Email == "" || req.Password == "" || req.Fullname == "" {
		return response.Error(c, "Missing required fields", 400, nil)
	}

	u, err := h.Service.CreateUser(c.Context(), usersvc.CreateUserInput{
		UserName: req.UserName,
		Email:    req.Email,
		Password: req.Password,
		Fullname: req.Fullname,
	})
	if err != nil {
		return mapCreateError(c, err)
	}

	safe := safeUser(u)
	orgIDStr := nilUUIDString(u.OrgID)

	// Rotate session and set identity (Express: session.regenerate, session.user, sAdd user_sessions)
	sid := middleware.RegenerateSessionID(c)
	middleware.SetSessionUser(c, middleware.SessionUser{
		UserID:   u.UserID.String(),
		Fullname: u.Fullname,
		Email:    u.Email,
		Role:     u.Role,
		OrgID:    orgIDStr,
	})
	if h.Service.Rdb != nil {
		_ = h.Service.Rdb.SAdd(c.Context(), userSessionsPrefix+u.UserID.String(), sid).Err()
	}

	// Cookie (same as login)
	cookie := middleware.SessionCookieConfig(h.Config)
	cookie.Value = "s:" + sid
	c.Cookie(&cookie)

	return response.SuccessCreated(c, "User created successfully", fiber.Map{"user": safe}, nil)
}

// UpdateUser PUT /api/v1/users/update-user — updates the session user (user_id from session).
func (h *Handlers) UpdateUser(c *fiber.Ctx) error {
	actor := getSessionActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	userID := actor.UserID
	if _, err := uuid.Parse(userID); err != nil {
		return response.Error(c, "Invalid user ID format (must be a valid UUID)", 400, nil)
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil || len(body) == 0 {
		return response.Error(c, "Missing update fields", 400, nil)
	}

	u, err := h.Service.UpdateUser(c.Context(), userID, body)
	if err != nil {
		return mapUpdateError(c, err)
	}
	return response.Success(c, "User updated successfully", fiber.Map{"user": safeUser(u)}, nil)
}

// ViewUser GET /api/v1/users/view-user — returns the session user (user_id from session).
func (h *Handlers) ViewUser(c *fiber.Ctx) error {
	actor := getSessionActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}
	userID := actor.UserID

	u, err := h.Service.ViewUser(c.Context(), userID)
	if err != nil {
		return mapViewError(c, err)
	}
	return response.Success(c, "User found", fiber.Map{"user": safeUser(u)}, nil)
}

// UpdateRoleRequest body: user_id, role (Express updateUserRoleController).
type UpdateRoleRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// UpdateRole PATCH /api/v1/users/update-role — requires ASSIGN_ROLE (middleware applied on route).
func (h *Handlers) UpdateRole(c *fiber.Ctx) error {
	var req UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "user_id and role are required", 400, nil)
	}
	if req.UserID == "" || req.Role == "" {
		return response.Error(c, "user_id and role are required", 400, nil)
	}

	actor := getSessionActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}

	u, err := h.Service.UpdateUserRole(c.Context(), usersvc.UpdateUserRoleInput{
		ActorUserID:  actor.UserID,
		ActorRole:    actor.Role,
		TargetUserID: req.UserID,
		TargetRole:   req.Role,
		OrgID:        actor.OrgID,
	})
	if err != nil {
		// Express: return res.error(error.message, 400)
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "User role updated successfully", fiber.Map{"user": safeUser(u)}, nil)
}

// RemoveUserRequest body: user_id (Express removeUserFromOrgController).
type RemoveUserRequest struct {
	UserID string `json:"user_id"`
}

// RemoveUser DELETE /api/v1/users/remove-user — requires REMOVE_USER (middleware applied on route).
func (h *Handlers) RemoveUser(c *fiber.Ctx) error {
	var req RemoveUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "user_id is required", 400, nil)
	}
	if req.UserID == "" {
		return response.Error(c, "user_id is required", 400, nil)
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		return response.Error(c, "Invalid user ID format (must be a valid UUID)", 400, nil)
	}

	actor := getSessionActor(c)
	if actor == nil {
		return response.Unauthorized(c, "Unauthorized")
	}

	err := h.Service.RemoveUserFromOrg(c.Context(), usersvc.RemoveUserFromOrgInput{
		ActorUserID:  actor.UserID,
		ActorRole:    actor.Role,
		TargetUserID: req.UserID,
		OrgID:        actor.OrgID,
	})
	if err != nil {
		return response.Error(c, err.Error(), 400, nil)
	}
	return response.Success(c, "User removed from organization", nil, nil)
}

type sessionActor struct {
	UserID string
	Role   string
	OrgID  *string
}

func getSessionActor(c *fiber.Ctx) *sessionActor {
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
	if userID == "" || role == "" {
		return nil
	}
	var orgID *string
	if o, ok := m["org_id"]; ok && o != nil {
		if s, ok := o.(string); ok {
			orgID = &s
		}
	}
	return &sessionActor{UserID: userID, Role: role, OrgID: orgID}
}

func safeUser(u *domain.User) fiber.Map {
	orgID := interface{}(nil)
	if u.OrgID != nil {
		orgID = u.OrgID.String()
	}
	return fiber.Map{
		"user_id":   u.UserID.String(),
		"fullname":  u.Fullname,
		"user_name": u.UserName,
		"email":     u.Email,
		"org_id":    orgID,
		"role":      u.Role,
		"createdAt": u.CreatedAt,
		"updatedAt": u.UpdatedAt,
	}
}

func nilUUIDString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

// status maps for Express parity (userController statusMap)
func mapCreateError(c *fiber.Ctx, err error) error {
	msg := err.Error()
	status := 500
	switch {
	case msg == "Invalid email format", msg == "Invalid password format",
		msg == "Full name is required and must be a non-empty string",
		msg == "Full name contains invalid characters (only letters, spaces, hyphens, and apostrophes allowed)",
		msg == "Username is required and must be a non-empty string":
		status = 400
	case msg == "Email already registered", msg == "Username already registered":
		status = 409
	}
	return response.Error(c, msg, status, nil)
}

func mapUpdateError(c *fiber.Ctx, err error) error {
	msg := err.Error()
	status := 500
	switch {
	case msg == "Missing user ID", msg == "Missing update fields", msg == "No valid update fields provided",
		msg == "Invalid email format", msg == "Invalid password format", msg == "Invalid org_id",
		msg == "Full name must be a non-empty string", msg == "Full name contains invalid characters",
		msg == "Invalid user ID format (must be a valid UUID)":
		status = 400
	case msg == "Email already registered", msg == "Username already registered":
		status = 409
	case msg == "User not found":
		status = 404
	}
	return response.Error(c, msg, status, nil)
}

func mapViewError(c *fiber.Ctx, err error) error {
	msg := err.Error()
	status := 500
	switch {
	case msg == "Missing user ID":
		status = 400
	case msg == "User not found":
		status = 404
	}
	return response.Error(c, msg, status, nil)
}
