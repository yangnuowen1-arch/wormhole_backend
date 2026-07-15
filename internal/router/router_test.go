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
		"GET /api/v1/admin/users",
		"POST /api/v1/admin/users",
		"GET /api/v1/admin/users/:id",
		"PATCH /api/v1/admin/users/:id",
		"DELETE /api/v1/admin/users/:id",
		"PUT /api/v1/admin/users/:id/roles",
		"GET /api/v1/announcements",
		"GET /api/v1/admin/announcements",
		"POST /api/v1/admin/announcements",
		"PATCH /api/v1/admin/announcements/:id",
		"PATCH /api/v1/admin/announcements/:id/status",
		"GET /api/v1/admin/resource-categories",
		"POST /api/v1/admin/resource-categories",
		"PATCH /api/v1/admin/resource-categories/:id",
		"PUT /api/v1/admin/resource-categories/sort",
		"PATCH /api/v1/admin/resource-categories/:id/status",
		"DELETE /api/v1/admin/resource-categories/:id",
		"GET /api/v1/admin/resources",
		"POST /api/v1/admin/resources",
		"PATCH /api/v1/admin/resources/:id",
		"PUT /api/v1/admin/resources/sort",
		"PATCH /api/v1/admin/resources/:id/status",
		"DELETE /api/v1/admin/resources/:id",
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
