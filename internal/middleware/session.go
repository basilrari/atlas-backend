package middleware

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SessionConfig for Redis-backed session; cookie and Redis format match Express/connect-redis.
type SessionConfig struct {
	Secret            string
	RedisURL          string
	AllowCrossSiteDev bool
	IsProduction      bool
}

const (
	sessionCookieName  = "troo.sid"
	SessionCookieName  = "troo.sid" // exported for auth/me debug logging
	sessionPrefix      = "session:"
	SessionRedisPrefix = "session:" // exported for auth logout (Del key)
	sessionMaxAge      = 24 * time.Hour
)

// SessionUser is the shape stored in session under "user" (Express parity).
type SessionUser struct {
	UserID   string  `json:"user_id"`
	Fullname string  `json:"fullname"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	OrgID    *string `json:"org_id"`
}

// Session returns a Fiber middleware that loads/saves session from Redis.
// Cookie name "troo.sid", Redis key prefix "session:", same TTL and flags as Express.
func Session(cfg SessionConfig) (fiber.Handler, *redis.Client, error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, err
	}
	rdb := redis.NewClient(opt)

	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies(sessionCookieName)
		// Express cookie may be "s:id" or "s:id.signature"; use first part as id
		if strings.HasPrefix(sessionID, "s:") {
			parts := strings.SplitN(sessionID[2:], ".", 2)
			sessionID = parts[0]
		}
		key := sessionPrefix + sessionID

		var data map[string]interface{}
		if sessionID != "" {
			b, err := rdb.Get(context.Background(), key).Bytes()
			if err == nil {
				_ = json.Unmarshal(b, &data)
			}
		}
		if data == nil {
			data = make(map[string]interface{})
		}

		// Store in Locals for handlers
		c.Locals("session_data", data)
		if u, ok := data["user"]; ok {
			c.Locals("user", u)
		} else {
			c.Locals("user", nil)
		}
		c.Locals("session_id", sessionID)

		err := c.Next()
		if err != nil {
			return err
		}

		// Persist if we have a session id (e.g. after login)
		if sid, _ := c.Locals("session_id").(string); sid != "" {
			updated, _ := c.Locals("session_data").(map[string]interface{})
			if updated != nil {
				b, _ := json.Marshal(updated)
				rdb.Set(context.Background(), sessionPrefix+sid, b, sessionMaxAge)
			}
		}
		return nil
	}, rdb, nil
}

// GetSessionID returns the current session ID from context (for login/logout).
func GetSessionID(c *fiber.Ctx) string {
	sid, _ := c.Locals("session_id").(string)
	return sid
}

// SetSessionUser sets the user in the session and marks session for save.
// Call after login/register; use RegenerateSessionID first to get a new id.
func SetSessionUser(c *fiber.Ctx, user SessionUser) {
	data, _ := c.Locals("session_data").(map[string]interface{})
	if data == nil {
		data = make(map[string]interface{})
	}
	data["user"] = map[string]interface{}{
		"user_id":  user.UserID,
		"fullname": user.Fullname,
		"email":    user.Email,
		"role":     user.Role,
		"org_id":   user.OrgID,
	}
	c.Locals("session_data", data)
	c.Locals("user", data["user"])
}

// RegenerateSessionID creates a new session ID and sets it in Locals (cookie set by handler).
// Cookie value should be "s:"+returned ID for Express compatibility.
func RegenerateSessionID(c *fiber.Ctx) string {
	newID := uuid.New().String()
	c.Locals("session_id", newID)
	return newID
}

// DestroySession clears user and session data from Locals; caller must clear cookie and Redis.
func DestroySession(c *fiber.Ctx) {
	c.Locals("session_data", make(map[string]interface{}))
	c.Locals("user", nil)
}

// SessionCookieConfig returns cookie options matching Express (for SetCookie/ClearCookie).
func SessionCookieConfig(cfg SessionConfig) fiber.Cookie {
	sameSite := "Lax"
	if cfg.AllowCrossSiteDev {
		sameSite = "None"
	}
	secure := cfg.IsProduction && cfg.AllowCrossSiteDev
	return fiber.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	}
}
