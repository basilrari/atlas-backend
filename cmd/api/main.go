package main

import (
	"context"
	"fmt"
	"net/http"

	"troo-backend/internal/config"
	"troo-backend/internal/interfaces/router"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var fiberApp *fiber.App
var appCfg *config.Config
var startupDB *gorm.DB
var startupRdb *redis.Client

func init() {
	cfg, err := config.Load()
	if err != nil {
		panic("config load: " + err.Error())
	}
	appCfg = cfg
	app, db, rdb, err := router.CreateApp(cfg)
	if err != nil {
		panic("app create: " + err.Error())
	}
	fiberApp = app
	startupDB = db
	startupRdb = rdb
}

func Handler(w http.ResponseWriter, r *http.Request) {
	adaptor.FiberApp(fiberApp)(w, r)
}

func main() {
	const port = "8888"

	// Verify connections before printing (Express-style startup logs)
	if startupDB != nil {
		sqlDB, err := startupDB.DB()
		if err != nil {
			panic("Supabase (Postgres): get DB: " + err.Error())
		}
		if err := sqlDB.Ping(); err != nil {
			panic("Supabase (Postgres) connection failed: " + err.Error())
		}
		fmt.Println("Supabase (Postgres) connected")
	}
	if startupRdb != nil {
		if err := startupRdb.Ping(context.Background()).Err(); err != nil {
			panic("Redis connection failed: " + err.Error())
		}
		fmt.Println("Redis connected")
	}
	fmt.Printf("Server running at http://localhost:%s\n", port)
	fmt.Printf("Health check: http://localhost:%s/health/json\n", port)
	fmt.Println("---")

	if err := fiberApp.Listen(":" + port); err != nil {
		panic(err)
	}
}
