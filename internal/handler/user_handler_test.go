package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/config"
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
		})
	}))
	issuer = keycloakServer.URL

	cfg := &config.Config{
		KeycloakEnabled:         true,
		KeycloakIssuerURL:       issuer,
		KeycloakClientID:        "wormhole",
		KeycloakClientSecret:    "client-secret",
		KeycloakRedirectURL:     "http://backend.test/api/v1/auth/sso/callback",
		KeycloakFrontendURL:     "http://frontend.test",
		KeycloakScopes:          []string{"openid", "profile", "email"},
		KeycloakStateSecret:     "0123456789abcdef0123456789abcdef",
		KeycloakStateCookieName: "wormhole_oidc_state",
		KeycloakHTTPTimeout:     time.Second,
		AuthCookieName:          "wormhole_session",
		AuthCookieSameSite:      "lax",
		JWTSecret:               "abcdef0123456789abcdef0123456789",
		JWTExpireHrs:            1,
	}
	h, err := NewUserHandler(nil, cfg)
	if err != nil {
		keycloakServer.Close()
		t.Fatalf("new user handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/auth/sso/login", h.StartSSO)
	router.GET("/api/v1/auth/sso/callback", h.CallbackSSO)
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
