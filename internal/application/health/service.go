package health

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"troo-backend/internal/middleware"

	"github.com/redis/go-redis/v9"
)

// DBPinger is optional for health check. If nil, database is reported as disconnected.
type DBPinger interface {
	Ping() error
}

// CollectResult matches Express collectHealth() return shape for /health/json and dashboard.
type CollectResult struct {
	Status       string                 `json:"status"`
	Runtime      RuntimeInfo            `json:"runtime"`
	Traffic      TrafficInfo            `json:"traffic"`
	Dependencies map[string]DepStatus   `json:"dependencies"`
}

type RuntimeInfo struct {
	UptimeSeconds int64       `json:"uptimeSeconds"`
	Memory        MemoryInfo  `json:"memory"`
	CPU           CPUInfo     `json:"cpu"`
	Platform      string      `json:"platform"`
	GoVersion     string      `json:"goVersion"`
}

type MemoryInfo struct {
	RSS     int `json:"rss"`
	HeapUsed int `json:"heapUsed"`
}

type CPUInfo struct {
	LoadAvg []string `json:"loadAvg"`
}

type TrafficInfo struct {
	TotalRequests   int         `json:"totalRequests"`
	SuccessCount    int         `json:"successCount"`
	FailedCount     int         `json:"failedCount"`
	SuccessRate     string      `json:"successRate"`
	AvgResponseTime interface{} `json:"avgResponseTime"`
	LastRequest     interface{} `json:"lastRequest"`
}

type DepStatus struct {
	Status string      `json:"status"`
	PingMs interface{} `json:"pingMs"`
}

// CollectHealth gathers health data from Redis, optional DB, and external HTTP pings.
func CollectHealth(ctx context.Context, rdb *redis.Client, db DBPinger) CollectResult {
	result := CollectResult{
		Dependencies: make(map[string]DepStatus),
	}

	// Database
	var dbStatus string = "disconnected"
	var dbPingMs *int64
	if db != nil {
		start := time.Now()
		if err := db.Ping(); err == nil {
			ms := time.Since(start).Milliseconds()
			dbPingMs = &ms
			dbStatus = "connected"
		} else {
			dbStatus = "error"
		}
	}
	result.Dependencies["database"] = DepStatus{Status: dbStatus, PingMs: dbPingMs}

	// Redis + traffic stats
	var redisStatus string = "disconnected"
	var redisPingMs *int64
	stats := TrafficInfo{AvgResponseTime: 0, SuccessRate: "100"}
	startTimeMs := time.Now().UnixMilli()

	if rdb != nil {
		start := time.Now()
		if err := rdb.Ping(ctx).Err(); err == nil {
			ms := time.Since(start).Milliseconds()
			redisPingMs = &ms
			redisStatus = "connected"

			totalReq, _ := rdb.Get(ctx, middleware.KeyReqTotal).Result()
			totalErr, _ := rdb.Get(ctx, middleware.KeyReqErrors).Result()
			totalTime, _ := rdb.Get(ctx, middleware.KeyResTime).Result()
			resCount, _ := rdb.Get(ctx, middleware.KeyResCount).Result()
			startTimeStr, _ := rdb.Get(ctx, middleware.KeyStartTime).Result()
			lastReqStr, _ := rdb.Get(ctx, middleware.KeyLastReq).Result()

			if startTimeStr != "" {
				if t, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
					startTimeMs = t
				}
			} else {
				rdb.Set(ctx, middleware.KeyStartTime, startTimeMs, 0)
			}

			stats.TotalRequests, _ = strconv.Atoi(totalReq)
			stats.FailedCount, _ = strconv.Atoi(totalErr)
			stats.SuccessCount = stats.TotalRequests - stats.FailedCount
			if stats.TotalRequests > 0 {
				stats.SuccessRate = strconv.FormatFloat(float64(stats.SuccessCount)/float64(stats.TotalRequests)*100, 'f', 1, 64)
			}
			timeSum, _ := strconv.ParseFloat(totalTime, 64)
			countSum, _ := strconv.Atoi(resCount)
			if countSum > 0 {
				stats.AvgResponseTime = strconv.FormatFloat(timeSum/float64(countSum), 'f', 2, 64)
			}
			if lastReqStr != "" {
				var lastReq map[string]interface{}
				_ = json.Unmarshal([]byte(lastReqStr), &lastReq)
				stats.LastRequest = lastReq
			}
		} else {
			redisStatus = "error"
		}
	}
	result.Dependencies["redis"] = DepStatus{Status: redisStatus, PingMs: redisPingMs}

	// Runtime (Go equivalent of Node process). Express: startTime in ms, uptimeSeconds = (now - startTime)/1000
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	uptimeSec := (time.Now().UnixMilli() - startTimeMs) / 1000
	if uptimeSec < 0 {
		uptimeSec = 0
	}
	result.Runtime = RuntimeInfo{
		UptimeSeconds: uptimeSec,
		Memory:       MemoryInfo{RSS: int(m.Alloc / 1024 / 1024), HeapUsed: int(m.HeapInuse / 1024 / 1024)},
		CPU:          CPUInfo{LoadAvg: []string{"0.00", "0.00", "0.00"}},
		Platform:     runtime.GOOS + " (" + runtime.GOARCH + ")",
		GoVersion:    runtime.Version(),
	}
	result.Traffic = stats
	result.Traffic.TotalRequests = stats.TotalRequests
	result.Traffic.SuccessCount = stats.SuccessCount
	result.Traffic.FailedCount = stats.FailedCount
	result.Traffic.SuccessRate = stats.SuccessRate
	result.Traffic.AvgResponseTime = stats.AvgResponseTime
	result.Traffic.LastRequest = stats.LastRequest

	// External pings (frontend, Stripe)
	fePing := httpPing("https://troo.earth", 3*time.Second)
	stripePing := httpPing("https://api.stripe.com/healthcheck", 3*time.Second)
	feStatus := "unreachable"
	if fePing != nil {
		feStatus = "reachable"
	}
	stripeStatus := "unreachable"
	if stripePing != nil {
		stripeStatus = "reachable"
	}
	result.Dependencies["frontend"] = DepStatus{Status: feStatus, PingMs: fePing}
	result.Dependencies["stripe"] = DepStatus{Status: stripeStatus, PingMs: stripePing}

	if dbStatus == "connected" && redisStatus == "connected" {
		result.Status = "ok"
	} else {
		result.Status = "issue"
	}
	return result
}

func httpPing(url string, timeout time.Duration) *int64 {
	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	ms := time.Since(start).Milliseconds()
	return &ms
}
