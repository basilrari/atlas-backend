package handler

import (
	"net/http"

	"troo-backend/app"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

var fiberApp *fiber.App

func init() {
	var err error
	fiberApp, err = app.New()
	if err != nil {
		panic("app create: " + err.Error())
	}
}

// Handler is the Vercel serverless entry point. All requests are rewritten here.
func Handler(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = r.URL.String()
	adaptor.FiberApp(fiberApp)(w, r)
}
