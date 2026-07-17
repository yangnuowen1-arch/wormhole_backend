package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/keycloak"
	"github.com/yang/wormhole_backend/internal/service"
)

func TestStartSSOUsesStateSpecificCookies(t *testing.T) {
	router, h, cleanup := newSSOTestRouter(t)
	defer cleanup()

	state1, cookie1 := performStartSSO(t, router)
	state2, cookie2 := performStartSSO(t, router)

	if state1 == state2 {
		t.Fatal("expected independent SSO starts to receive different state values")
	}
	if cookie1.Name == cookie2.Name {
		t.Fatal("expected independent SSO starts to receive different state cookie names")
	}
	if cookie1.Name != h.oidcStateCookieName(state1) {
		t.Fatalf("first state cookie name = %q, want %q", cookie1.Name, h.oidcStateCookieName(state1))
	}
	if cookie2.Name != h.oidcStateCookieName(state2) {
		t.Fatalf("second state cookie name = %q, want %q", cookie2.Name, h.oidcStateCookieName(state2))
	}
	if cookie1.Name == h.cfg.KeycloakStateCookieName || cookie2.Name == h.cfg.KeycloakStateCookieName {
		t.Fatal("expected state-specific cookies, got legacy shared cookie name")
	}
}

func TestStartSSOReusesExistingPendingState(t *testing.T) {
	router, _, cleanup := newSSOTestRouter(t)
	defer cleanup()

	state1, cookie1 := performStartSSO(t, router)
	state2, cookie2 := performStartSSO(t, router, cookie1)

	if state2 != state1 {
		t.Fatalf("state after repeated start = %q, want existing state %q", state2, state1)
	}
	if cookie2.Name != cookie1.Name {
		t.Fatalf("state cookie name after repeated start = %q, want %q", cookie2.Name, cookie1.Name)
	}
}

func TestCallbackSSOUsesMatchingStateCookie(t *testing.T) {
	router, h, cleanup := newSSOTestRouter(t)
	defer cleanup()

	loginState, err := h.stateManager.NewLoginState("http://frontend.test/navigation")
	if err != nil {
		t.Fatalf("create login state: %v", err)
	}
	encodedState, err := h.stateManager.Encode(loginState)
	if err != nil {
		t.Fatalf("encode login state: %v", err)
	}

	otherState, err := h.stateManager.NewLoginState("http://frontend.test/navigation")
	if err != nil {
		t.Fatalf("create other login state: %v", err)
	}
	otherEncodedState, err := h.stateManager.Encode(otherState)
	if err != nil {
		t.Fatalf("encode other login state: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/callback?state="+url.QueryEscape(loginState.State), nil)
	req.AddCookie(&http.Cookie{Name: h.oidcStateCookieName(otherState.State), Value: otherEncodedState})
	req.AddCookie(&http.Cookie{Name: h.oidcStateCookieName(loginState.State), Value: encodedState})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("callback status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode callback response: %v", err)
	}
	if body.Code != 40001 {
		t.Fatalf("callback code = %d, want missing-code response 40001", body.Code)
	}
}

func TestLogoutSSORedirectsToKeycloakAndClearsLocalCookies(t *testing.T) {
	router, h, cleanup := newSSOTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: h.cfg.AuthCookieName, Value: "application-session"})
	req.AddCookie(&http.Cookie{Name: h.keycloakIDTokenCookieName(), Value: "keycloak-id-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("logout status = %d, want %d; body: %s", w.Code, http.StatusFound, w.Body.String())
	}
	redirectURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse logout redirect URL: %v", err)
	}
	if redirectURL.Path != "/logout" {
		t.Fatalf("logout redirect path = %q, want /logout", redirectURL.Path)
	}
	query := redirectURL.Query()
	if got, want := query.Get("id_token_hint"), "keycloak-id-token"; got != want {
		t.Fatalf("id_token_hint = %q, want %q", got, want)
	}
	if got, want := query.Get("client_id"), h.cfg.KeycloakClientID; got != want {
		t.Fatalf("client_id = %q, want %q", got, want)
	}
	if got, want := query.Get("post_logout_redirect_uri"), "http://frontend.test/login"; got != want {
		t.Fatalf("post_logout_redirect_uri = %q, want %q", got, want)
	}

	cleared := map[string]*http.Cookie{}
	for _, cookie := range w.Result().Cookies() {
		cleared[cookie.Name] = cookie
	}
	for _, name := range []string{h.cfg.AuthCookieName, h.keycloakIDTokenCookieName()} {
		cookie, ok := cleared[name]
		if !ok {
			t.Fatalf("logout response did not clear cookie %q", name)
		}
		if cookie.Value != "" || cookie.MaxAge >= 0 {
			t.Fatalf("cookie %q = value %q, max-age %d; want deletion cookie", name, cookie.Value, cookie.MaxAge)
		}
	}
}

func TestLogoutSSOOnlyAllowsConfiguredFrontendReturnURL(t *testing.T) {
	router, _, cleanup := newSSOTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout?return_to="+url.QueryEscape("https://attacker.example/logout"), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("logout status = %d, want %d; body: %s", w.Code, http.StatusFound, w.Body.String())
	}
	redirectURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse logout redirect URL: %v", err)
	}
	if got, want := redirectURL.Query().Get("post_logout_redirect_uri"), "http://frontend.test"; got != want {
		t.Fatalf("post_logout_redirect_uri = %q, want %q", got, want)
	}
}

func TestAssignRolesReturnsUpdatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &userServiceStub{
		assignRoles: func(_ context.Context, userID int64, req dto.AssignUserRolesRequest) (dto.UserResponse, error) {
			if userID != 8 {
				t.Fatalf("user ID = %d, want 8", userID)
			}
			if len(req.RoleCodes) != 1 || req.RoleCodes[0] != "admin" {
				t.Fatalf("role codes = %v, want [admin]", req.RoleCodes)
			}
			return dto.UserResponse{
				ID:       8,
				Username: "alice",
				Roles:    []dto.RoleResponse{{ID: 1, Code: "admin", Name: "管理员"}},
			}, nil
		},
	}
	h, err := NewUserHandler(stub, &config.Config{})
	if err != nil {
		t.Fatalf("NewUserHandler returned error: %v", err)
	}
	router := gin.New()
	router.PUT("/api/v1/admin/users/:id/roles", h.AssignRoles)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/8/roles", strings.NewReader(`{"roleCodes":["admin"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		Code int              `json:"code"`
		Data dto.UserResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || body.Data.ID != 8 || len(body.Data.Roles) != 1 || body.Data.Roles[0].Code != "admin" {
		t.Fatalf("response = %+v, want updated admin user", body)
	}
}

func newSSOTestRouter(t *testing.T) (*gin.Engine, *UserHandler, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var issuer string
	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/auth",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/jwks",
			"end_session_endpoint":   issuer + "/logout",
		})
	}))
	issuer = keycloakServer.URL

	cfg := &config.Config{
		KeycloakEnabled:           true,
		KeycloakIssuerURL:         issuer,
		KeycloakClientID:          "wormhole",
		KeycloakClientSecret:      "client-secret",
		KeycloakRedirectURL:       "http://backend.test/api/v1/auth/sso/callback",
		KeycloakFrontendURL:       "http://frontend.test",
		KeycloakScopes:            []string{"openid", "profile", "email"},
		KeycloakStateSecret:       "0123456789abcdef0123456789abcdef",
		KeycloakStateCookieName:   "wormhole_oidc_state",
		KeycloakIDTokenCookieName: "wormhole_oidc_id_token",
		KeycloakHTTPTimeout:       time.Second,
		AuthCookieName:            "wormhole_session",
		AuthCookieSameSite:        "lax",
		JWTSecret:                 "abcdef0123456789abcdef0123456789",
		JWTExpireHrs:              1,
	}
	h, err := NewUserHandler(nil, cfg)
	if err != nil {
		keycloakServer.Close()
		t.Fatalf("new user handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/auth/sso/login", h.StartSSO)
	router.GET("/api/v1/auth/sso/callback", h.CallbackSSO)
	router.GET("/api/v1/auth/logout", h.LogoutSSO)
	return router, h, keycloakServer.Close
}

func performStartSSO(t *testing.T, router *gin.Engine, cookies ...*http.Cookie) (string, *http.Cookie) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/login?return_to=/navigation", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("start SSO status = %d, want %d; body: %s", w.Code, http.StatusFound, w.Body.String())
	}

	location := w.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect URL %q: %v", location, err)
	}
	state := redirectURL.Query().Get("state")
	if state == "" {
		t.Fatalf("redirect URL %q does not contain state", location)
	}

	for _, cookie := range w.Result().Cookies() {
		if cookie.Value == "" {
			continue
		}
		return state, cookie
	}
	t.Fatal("start SSO response did not set a state cookie")
	return "", nil
}

type userServiceStub struct {
	assignRoles func(context.Context, int64, dto.AssignUserRolesRequest) (dto.UserResponse, error)
	listUsers   func(context.Context) ([]dto.UserResponse, error)
	getUser     func(context.Context, int64) (dto.UserResponse, error)
	createUser  func(context.Context, dto.CreateAdminUserRequest) (dto.UserResponse, error)
	updateUser  func(context.Context, int64, dto.UpdateAdminUserRequest) (dto.UserResponse, error)
	deleteUser  func(context.Context, int64) error
}

func (s *userServiceStub) Register(ctx context.Context, req dto.RegisterRequest) (dto.UserResponse, error) {
	return dto.UserResponse{}, nil
}

func (s *userServiceStub) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	return dto.LoginResponse{}, nil
}

func (s *userServiceStub) LoginWithKeycloak(ctx context.Context, identity keycloak.Identity) (dto.LoginResponse, error) {
	return dto.LoginResponse{}, nil
}

func (s *userServiceStub) Me(ctx context.Context) (dto.UserResponse, error) {
	return dto.UserResponse{}, nil
}

func (s *userServiceStub) ListUsers(ctx context.Context) ([]dto.UserResponse, error) {
	if s.listUsers == nil {
		return nil, nil
	}
	return s.listUsers(ctx)
}

func (s *userServiceStub) GetUser(ctx context.Context, userID int64) (dto.UserResponse, error) {
	if s.getUser == nil {
		return dto.UserResponse{}, nil
	}
	return s.getUser(ctx, userID)
}

func (s *userServiceStub) CreateUser(ctx context.Context, req dto.CreateAdminUserRequest) (dto.UserResponse, error) {
	if s.createUser == nil {
		return dto.UserResponse{}, nil
	}
	return s.createUser(ctx, req)
}

func (s *userServiceStub) UpdateUser(ctx context.Context, userID int64, req dto.UpdateAdminUserRequest) (dto.UserResponse, error) {
	if s.updateUser == nil {
		return dto.UserResponse{}, nil
	}
	return s.updateUser(ctx, userID, req)
}

func (s *userServiceStub) DeleteUser(ctx context.Context, userID int64) error {
	if s.deleteUser == nil {
		return nil
	}
	return s.deleteUser(ctx, userID)
}

func (s *userServiceStub) AssignRoles(ctx context.Context, userID int64, req dto.AssignUserRolesRequest) (dto.UserResponse, error) {
	if s.assignRoles == nil {
		return dto.UserResponse{}, nil
	}
	return s.assignRoles(ctx, userID, req)
}

var _ service.UserService = (*userServiceStub)(nil)
