package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/dal/model"
)

func TestRequireAdminRejectsUnauthenticatedRequest(t *testing.T) {
	router := newRoleTestRouter(t, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireAdminRejectsNonAdmin(t *testing.T) {
	router := newRoleTestRouter(t, &auth.Claims{UserID: 7, Username: "alice"}, []model.Role{
		{Code: "user", Name: "普通用户"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireAdminAllowsAdmin(t *testing.T) {
	router := newRoleTestRouter(t, &auth.Claims{UserID: 7, Username: "admin"}, []model.Role{
		{Code: "admin", Name: "管理员"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func newRoleTestRouter(t *testing.T, claims *auth.Claims, roles []model.Role) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	chain := []gin.HandlerFunc{}
	if claims != nil {
		chain = append(chain, func(c *gin.Context) {
			c.Request = c.Request.WithContext(auth.WithClaims(c.Request.Context(), claims))
			c.Next()
		})
	}
	chain = append(chain, RequireAdmin(&roleFinderStub{roles: roles}))
	chain = append(chain, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/admin", chain...)
	return router
}

type roleFinderStub struct {
	roles []model.Role
}

func (s *roleFinderStub) FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	return s.roles, nil
}

var _ RoleFinder = (*roleFinderStub)(nil)
