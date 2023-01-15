package homey

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func TestDataPack(t *testing.T) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	h := New()
	// Routes
	e.Static("/", "./example/public")
	e.GET("/ws", h.Echo())

	// Start server
	e.Logger.Fatal(e.Start(":1323"))
}
