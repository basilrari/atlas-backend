package health

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHealth_WithNilRedis(t *testing.T) {
	ctx := context.Background()
	result := CollectHealth(ctx, nil, nil)
	assert.Equal(t, "issue", result.Status)
	assert.Equal(t, "disconnected", result.Dependencies["database"].Status)
	assert.Equal(t, "disconnected", result.Dependencies["redis"].Status)
	assert.NotNil(t, result.Runtime)
	assert.NotNil(t, result.Traffic)
	assert.Equal(t, 0, result.Traffic.TotalRequests)
}

func TestCollectHealth_WithMiniredis(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	ctx := context.Background()

	// No keys set: should get status "connected" for Redis, traffic zeros
	result := CollectHealth(ctx, rdb, nil)
	assert.Equal(t, "connected", result.Dependencies["redis"].Status)
	assert.Equal(t, "disconnected", result.Dependencies["database"].Status)
	assert.Equal(t, 0, result.Traffic.TotalRequests)
	assert.Equal(t, "100", result.Traffic.SuccessRate)

	// Set traffic keys (same as middleware)
	require.NoError(t, rdb.Set(ctx, "health:global:req_total", "10", 0).Err())
	require.NoError(t, rdb.Set(ctx, "health:global:req_errors", "2", 0).Err())
	require.NoError(t, rdb.Set(ctx, "health:global:res_time_total", "150.5", 0).Err())
	require.NoError(t, rdb.Set(ctx, "health:global:res_count", "10", 0).Err())
	require.NoError(t, rdb.Set(ctx, "health:global:start_time", "1000000", 0).Err())

	result2 := CollectHealth(ctx, rdb, nil)
	assert.Equal(t, 10, result2.Traffic.TotalRequests)
	assert.Equal(t, 2, result2.Traffic.FailedCount)
	assert.Equal(t, 8, result2.Traffic.SuccessCount)
	assert.Equal(t, "80.0", result2.Traffic.SuccessRate)
	assert.Equal(t, "15.05", result2.Traffic.AvgResponseTime)
}
