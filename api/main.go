package main

import (
	"net/http"
	"os"

	"troo-backend/internal/config"
	"troo-backend/internal/interfaces/router"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

var fiberApp *fiber.App

func init() {
	cfg, err := config.Load()
	if err != nil {
		panic("config load: " + err.Error())
	}
	fiberApp, err = router.CreateApp(cfg)
	if err != nil {
		panic("app create: " + err.Error())
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	adaptor.FiberApp(fiberApp)(w, r)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := fiberApp.Listen(":" + port); err != nil {
		panic(err)
	}
}
