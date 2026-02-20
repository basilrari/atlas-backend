package auth

import (
	"context"

	authsvc "troo-backend/internal/application/auth"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
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

	// Set cookie so client gets troo.sid = "s:"+sessionID (Express: no domain when setting)
	cookie := middleware.SessionCookieConfig(h.Config)
	cookie.Value = "s:" + sessionID
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
	sessionID := middleware.GetSessionID(c)
	sessionUser := middleware.GetUser(c)

	// Debug logging for auth/me (check server logs to see why 401)
	if sessionID == "" {
		cookieVal := c.Cookies(middleware.SessionCookieName)
		log.Info().Str("path", "/auth/me").
			Bool("cookie_present", cookieVal != "").
			Int("cookie_len", len(cookieVal)).
			Msg("auth/me: no session id — missing cookie or invalid format")
	} else if sessionUser == nil {
		log.Info().Str("path", "/auth/me").Str("session_id_prefix", truncate(sessionID, 8)).
			Msg("auth/me: session id present but no user in session data (Redis key may be missing or empty)")
	}

	user, err := authsvc.VerifyUser(sessionUser)
	if err != nil {
		log.Info().Str("path", "/auth/me").Err(err).
			Bool("session_user_nil", sessionUser == nil).
			Msg("auth/me: returning 401 Not authenticated")
		return response.Error(c, "Not authenticated", fiber.StatusUnauthorized, nil)
	}
	log.Info().Str("path", "/auth/me").Str("user_id", user.UserID).
		Msg("auth/me: success")
	return response.Success(c, "Authenticated", fiber.Map{"user": user}, nil)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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
