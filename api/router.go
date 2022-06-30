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
	e.POST("/keygen", tssController.Keygen())
	e.POST("/import", tssController.Import())
	e.GET("/export", tssController.Export())
	e.POST("/regroup", tssController.Regroup())
}
