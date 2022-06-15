package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// InitRouting Initialize Router
func InitRouting(e *echo.Echo, tssController TssController) {
	// Middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))
	e.Use(middleware.Recover())

	e.POST("/sign", tssController.Sign())
	e.POST("/message", tssController.Message())
}
