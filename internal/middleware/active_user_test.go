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

func TestRequireActiveUserRejectsDisabledAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabled := int16(0)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(auth.WithClaims(c.Request.Context(), &auth.Claims{UserID: 7}))
		c.Next()
	})
	router.Use(RequireActiveUser(&activeUserFinderStub{user: &model.User{ID: 7, Status: &disabled}}))
	handled := false
	router.GET("/protected", func(c *gin.Context) {
		handled = true
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
	if handled {
		t.Fatal("disabled user reached protected handler")
	}
}

type activeUserFinderStub struct {
	user *model.User
	err  error
}

func (s *activeUserFinderStub) FindByID(ctx context.Context, id int64) (*model.User, error) {
	return s.user, s.err
}
