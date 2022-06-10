package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// InitRouting Initialize Router
func InitRouting(e *echo.Echo, tssController TssController) {
	//f, err := os.OpenFile("rosen-tss.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	//if err != nil {
	//}
	//defer func(f *os.File) {
	//	err := f.Close()
	//	if err != nil {
	//
	//	}
	//}(f)

	// Middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
		//Output: f,
	}))
	e.Use(middleware.Recover())

	e.POST("/keygen", tssController.Keygen())
	e.POST("/sign", tssController.Sign())
	e.POST("/regroup", tssController.Regroup())
	e.POST("/message", tssController.Message())
	e.POST("/import", tssController.Import())
	e.GET("/export", tssController.Export())
}
