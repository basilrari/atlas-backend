package policies

import (
	"context"

	"troo-backend/internal/middleware"

	"github.com/redis/go-redis/v9"
)

// DestroyUserSessions removes all sessions for a user (Express auth/policies/sessionInvalidation).
// Deletes each session key (session:<sid>) and the user_sessions:<user_id> set.
func DestroyUserSessions(ctx context.Context, rdb *redis.Client, userID string) {
	if userID == "" {
		return
	}
	key := "user_sessions:" + userID
	sessionIDs, err := rdb.SMembers(ctx, key).Result()
	if err != nil || len(sessionIDs) == 0 {
		rdb.Del(ctx, key)
		return
	}
	for _, sid := range sessionIDs {
		rdb.Del(ctx, middleware.SessionRedisPrefix+sid)
	}
	rdb.Del(ctx, key)
}
