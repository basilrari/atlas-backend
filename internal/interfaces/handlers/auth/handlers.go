package auth

import (
	"context"

	authsvc "troo-backend/internal/application/auth"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const userSessionsPrefix = "user_sessions:"

// Handlers holds dependencies for auth endpoints.
type Handlers struct {
	UserFinder authsvc.UserFinder
	Rdb        *redis.Client
	Config     middleware.SessionConfig
}

// LoginRequest body (Express: req.body email, password).
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login POST /api/v1/auth/login — authenticate, create session, SAdd user_sessions:user_id, set cookie, return success.
func (h *Handlers) Login(c *fiber.Ctx) error {
	if h.UserFinder == nil {
		return response.Error(c, "Internal Server Error", fiber.StatusInternalServerError, nil)
	}
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "Email and password are required", fiber.StatusBadRequest, nil)
	}
	if req.Email == "" || req.Password == "" {
		return response.Error(c, "Email and password are required", fiber.StatusBadRequest, nil)
	}

	user, err := h.UserFinder.FindByEmailAndPassword(req.Email, req.Password)
	if err != nil {
		switch err {
		case authsvc.ErrEmailPasswordRequired:
			return response.Error(c, err.Error(), fiber.StatusBadRequest, nil)
		case authsvc.ErrInvalidEmail, authsvc.ErrIncorrectPassword:
			return response.Error(c, err.Error(), fiber.StatusUnauthorized, nil)
		default:
			return response.Error(c, "Internal Server Error", fiber.StatusInternalServerError, nil)
		}
	}

	// Regenerate session ID (new session for this login)
	sessionID := middleware.RegenerateSessionID(c)
	orgIDStr := nilString(user.OrgID)

	middleware.SetSessionUser(c, middleware.SessionUser{
		UserID:   user.UserID.String(),
		Fullname: user.Fullname,
		Email:    user.Email,
		Role:     user.Role,
		OrgID:    orgIDStr,
	})

	// Track session in Redis (Express: redisClient.sAdd(`user_sessions:${user.user_id}`, req.sessionID))
	ctx := context.Background()
	if err := h.Rdb.SAdd(ctx, userSessionsPrefix+user.UserID.String(), sessionID).Err(); err != nil {
		return response.Error(c, "Internal Server Error", fiber.StatusInternalServerError, nil)
	}

	// Set cookie so client gets troo.sid = "s:"+sessionID (Express compatibility)
	cookie := middleware.SessionCookieConfig(h.Config)
	cookie.Value = "s:" + sessionID
	cookie.Expires = cookie.Expires // use default from config
	if h.Config.IsProduction && !h.Config.AllowCrossSiteDev {
		cookie.Domain = ".troo.earth"
	}
	c.Cookie(&cookie)

	// Response: standard success with user object (no password)
	return response.Success(c, "Login successful", fiber.Map{
		"user": fiber.Map{
			"user_id":  user.UserID.String(),
			"fullname": user.Fullname,
			"email":    user.Email,
			"role":     user.Role,
			"org_id":   orgIDStr,
		},
	}, nil)
}

// Me GET /api/v1/auth/me — return current session user in standard success format.
func (h *Handlers) Me(c *fiber.Ctx) error {
	sessionUser := middleware.GetUser(c)
	user, err := authsvc.VerifyUser(sessionUser)
	if err != nil {
		return response.Error(c, "Not authenticated", fiber.StatusUnauthorized, nil)
	}
	return response.Success(c, "Authenticated", fiber.Map{"user": user}, nil)
}

// Logout DELETE /api/v1/auth/logout — SRem user_sessions:user_id, Del session key, clear cookie, return success.
func (h *Handlers) Logout(c *fiber.Ctx) error {
	sessionID := middleware.GetSessionID(c)
	sessionUser := middleware.GetUser(c)

	ctx := context.Background()

	if sessionUser != nil && sessionID != "" {
		if m, ok := sessionUser.(map[string]interface{}); ok {
			if userID, _ := m["user_id"].(string); userID != "" {
				_ = h.Rdb.SRem(ctx, userSessionsPrefix+userID, sessionID).Err()
			}
		}
	}

	// Destroy session in Redis (Express: session.destroy)
	if sessionID != "" {
		_ = h.Rdb.Del(ctx, middleware.SessionRedisPrefix+sessionID).Err()
	}

	middleware.DestroySession(c)

	// Clear cookie (same options as Express)
	cookie := middleware.SessionCookieConfig(h.Config)
	cookie.Value = ""
	cookie.Expires = cookie.Expires
	cookie.MaxAge = -1
	if h.Config.IsProduction && !h.Config.AllowCrossSiteDev {
		cookie.Domain = ".troo.earth"
	}
	c.Cookie(&cookie)

	return response.Success(c, "Logged out successfully", nil, nil)
}

func nilString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}
