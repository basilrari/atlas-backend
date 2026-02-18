// +build integration

package health

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestCollectHealth_RealRedis runs against real Redis when REDIS_URL is set.
// Run with: go test -tags=integration ./internal/health/... -run TestCollectHealth_RealRedis -v
func TestCollectHealth_RealRedis(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set, skipping integration test")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	ctx := context.Background()

	result := CollectHealth(ctx, rdb, nil)
	require.Equal(t, "connected", result.Dependencies["redis"].Status)
	require.NotNil(t, result.Dependencies["redis"].PingMs)
	require.Contains(t, []string{"ok", "issue"}, result.Status)
}
