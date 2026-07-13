package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/config"
	"gorm.io/gorm"
)

func TestSetupRouterRegistersHomeRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	app := SetupRouter(&gorm.DB{}, &config.Config{
		JWTSecret:      "abcdef0123456789abcdef0123456789",
		AuthCookieName: "wormhole_session",
	})

	routes := make(map[string]struct{})
	for _, route := range app.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range []string{
		"GET /api/v1/recommendations",
		"GET /api/v1/carousel-slides",
		"GET /api/v1/admin/recommendations",
		"POST /api/v1/admin/recommendations",
		"PATCH /api/v1/admin/recommendations/:id",
		"PUT /api/v1/admin/recommendations/sort",
		"PATCH /api/v1/admin/recommendations/:id/status",
		"GET /api/v1/admin/carousel-slides",
		"POST /api/v1/admin/carousel-slides",
		"PATCH /api/v1/admin/carousel-slides/:id",
		"PUT /api/v1/admin/carousel-slides/sort",
		"PATCH /api/v1/admin/carousel-slides/:id/status",
	} {
		if _, ok := routes[route]; !ok {
			t.Fatalf("route %q was not registered", route)
		}
	}
}
