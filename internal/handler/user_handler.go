package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/keycloak"
	"github.com/yang/wormhole_backend/internal/response"
	"github.com/yang/wormhole_backend/internal/service"
)

const (
	oidcStateTTL        = 10 * time.Minute
	oidcStateCookiePath = "/api/v1/auth/sso"
)

// UserHandler 用户 HTTP 层。
type UserHandler struct {
	service      service.UserService
	cfg          *config.Config
	keycloak     *keycloak.Client
	stateManager *auth.OIDCStateManager
}

// NewUserHandler 构造 UserHandler。
func NewUserHandler(svc service.UserService, cfg *config.Config) (*UserHandler, error) {
	h := &UserHandler{service: svc, cfg: cfg}
	if cfg != nil && cfg.KeycloakEnabled {
		kc, err := keycloak.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		stateManager, err := auth.NewOIDCStateManager(cfg.KeycloakStateSecret, oidcStateTTL)
		if err != nil {
			return nil, err
		}
		h.keycloak = kc
		h.stateManager = stateManager
	}
	return h, nil
}

// Register 用户注册。
// Deprecated: 统一 SSO 后该兼容接口不对前端开放，也不出现在 Swagger 文档中。
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	user, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			response.Error(c, http.StatusBadRequest, 40001, "用户名已被占用", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "注册失败", err.Error())
		return
	}
	response.Created(c, user)
}

// Login 用户登录。
// Deprecated: 统一 SSO 后该兼容接口不对前端开放，也不出现在 Swagger 文档中。
func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	resp, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, 40101, "用户不存在", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "登录失败", err.Error())
		return
	}
	response.Success(c, resp)
}

// StartSSO 发起 Keycloak Authorization Code + PKCE 登录。
// 前端的 SSO 按钮可直接跳转到：GET /api/v1/auth/sso/login?return_to=/
// @Summary 发起 Keycloak SSO 登录
// @Description 前端点击 SSO 按钮时应使用 window.location.href 跳转到本接口，不建议用 fetch。本接口生成 OIDC state/nonce/PKCE verifier，写入 HttpOnly 临时 Cookie，然后 302 重定向到 Keycloak 登录页。登录成功后 Keycloak 会回调 /auth/sso/callback。
// @Tags auth
// @Produce json
// @Param return_to query string false "登录成功后跳回的前端路径，只允许相对路径或 KEYCLOAK_FRONTEND_URL 同源绝对地址，例如 /navigation" default(/navigation)
// @Success 302 {string} string "Redirect to Keycloak authorization endpoint"
// @Failure 404 {object} dto.ErrorAPIResponse "SSO 未启用"
// @Failure 500 {object} dto.ErrorAPIResponse "创建或编码 SSO 状态失败"
// @Failure 502 {object} dto.ErrorAPIResponse "获取 Keycloak 授权地址失败"
// @Router /auth/sso/login [get]
func (h *UserHandler) StartSSO(c *gin.Context) {
	if !h.ssoReady() {
		response.Error(c, http.StatusNotFound, 40401, "SSO 未启用", nil)
		return
	}

	returnTo := h.safeReturnTo(c.Query("return_to"))
	loginState, ok := h.reusableLoginState(c, returnTo)
	if !ok {
		var err error
		loginState, err = h.stateManager.NewLoginState(returnTo)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, 50001, "创建 SSO 状态失败", err.Error())
			return
		}
	}

	encodedState, err := h.stateManager.Encode(loginState)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "编码 SSO 状态失败", err.Error())
		return
	}

	authorizationURL, err := h.keycloak.AuthorizationURL(c.Request.Context(), loginState.State, loginState.Nonce, loginState.Verifier)
	if err != nil {
		response.Error(c, http.StatusBadGateway, 50201, "获取 Keycloak 授权地址失败", err.Error())
		return
	}

	h.setCookie(c, h.oidcStateCookieName(loginState.State), encodedState, int(oidcStateTTL.Seconds()), oidcStateCookiePath)
	c.Redirect(http.StatusFound, authorizationURL)
}

// CallbackSSO 接收 Keycloak 回调，用 code 换 token，验证 ID Token，建立本应用会话。
// @Summary Keycloak SSO 登录回调
// @Description 该接口由 Keycloak 重定向调用，前端无需主动调用。后端校验 state，用 code 换取 token，验证 ID Token 签名/issuer/audience/nonce，按 Keycloak sub 创建或更新本地用户，写入 HttpOnly 会话 Cookie（默认 wormhole_session），最后 302 重定向回 return_to 对应的前端页面。
// @Tags auth
// @Produce json
// @Param code query string false "Keycloak authorization code；Keycloak 登录成功时携带"
// @Param state query string false "OIDC state；必须与后端临时 Cookie 中的 state 一致"
// @Param error query string false "Keycloak error；Keycloak 登录失败时携带"
// @Success 302 {string} string "Redirect to frontend return_to page"
// @Failure 400 {object} dto.ErrorAPIResponse "SSO 状态过期/无效、state 不匹配或缺少授权码"
// @Failure 401 {object} dto.ErrorAPIResponse "Keycloak 返回登录错误"
// @Failure 404 {object} dto.ErrorAPIResponse "SSO 未启用"
// @Failure 500 {object} dto.ErrorAPIResponse "建立本地会话失败"
// @Failure 502 {object} dto.ErrorAPIResponse "Keycloak token 交换或 ID Token 校验失败"
// @Router /auth/sso/callback [get]
func (h *UserHandler) CallbackSSO(c *gin.Context) {
	if !h.ssoReady() {
		response.Error(c, http.StatusNotFound, 40401, "SSO 未启用", nil)
		return
	}

	if oidcErr := c.Query("error"); oidcErr != "" {
		h.clearOIDCStateCookie(c, h.oidcStateCookieName(c.Query("state")))
		response.Error(c, http.StatusUnauthorized, 40102, "Keycloak 登录失败", oidcErr)
		return
	}

	queryState := c.Query("state")
	stateCookieName, stateCookie, ok := h.oidcStateCookie(c, queryState)
	if !ok {
		response.Error(c, http.StatusBadRequest, 40002, "SSO 状态已过期，请重新登录", nil)
		return
	}
	loginState, err := h.stateManager.Decode(stateCookie)
	if err != nil {
		h.clearOIDCStateCookie(c, stateCookieName)
		response.Error(c, http.StatusBadRequest, 40002, "SSO 状态无效，请重新登录", err.Error())
		return
	}
	if queryState != loginState.State {
		h.clearOIDCStateCookie(c, stateCookieName)
		response.Error(c, http.StatusBadRequest, 40002, "SSO state 校验失败", nil)
		return
	}

	code := c.Query("code")
	if code == "" {
		h.clearOIDCStateCookie(c, stateCookieName)
		response.Error(c, http.StatusBadRequest, 40001, "缺少授权码", nil)
		return
	}

	identity, err := h.keycloak.ExchangeAndVerify(c.Request.Context(), code, loginState.Verifier, loginState.Nonce)
	if err != nil {
		h.clearOIDCStateCookie(c, stateCookieName)
		response.Error(c, http.StatusBadGateway, 50202, "Keycloak token 校验失败", err.Error())
		return
	}

	loginResp, err := h.service.LoginWithKeycloak(c.Request.Context(), identity)
	if err != nil {
		h.clearOIDCStateCookie(c, stateCookieName)
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidKeycloakIdentity) || errors.Is(err, service.ErrKeycloakUsernameConflict) {
			status = http.StatusBadRequest
		}
		response.Error(c, status, 50002, "建立本地会话失败", err.Error())
		return
	}

	h.clearOIDCStateCookie(c, stateCookieName)
	h.setCookie(c, h.cfg.AuthCookieName, loginResp.Token, h.cfg.JWTExpireHrs*3600, "/")
	c.Redirect(http.StatusFound, h.safeReturnTo(loginState.ReturnTo))
}

// Logout 清除本应用会话 Cookie。Keycloak 单点登出可后续再接 end_session_endpoint。
// @Summary 退出登录
// @Description 清除后端本应用 HttpOnly 会话 Cookie（默认 wormhole_session）。前端调用时必须带 credentials: "include"。当前接口只退出本应用会话，不一定退出 Keycloak 全局会话。
// @Tags auth
// @Produce json
// @Success 200 {object} dto.LogoutAPIResponse "退出成功"
// @Router /auth/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	if h.cfg != nil {
		h.clearCookie(c, h.cfg.AuthCookieName, "/")
	}
	response.Success(c, gin.H{"logged_out": true})
}

// Me 获取当前登录用户信息
// @Summary 获取当前登录用户信息
// @Description 前端路由权限判断接口。浏览器登录后会自动携带 HttpOnly Cookie，前端必须使用 credentials: "include"；也兼容 Authorization: Bearer <token>。返回 200 表示已登录，返回 401 表示未登录或会话失效。
// @Tags user
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} dto.UserAPIResponse "当前用户信息"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取用户信息失败"
// @Router /users/me [get]
func (h *UserHandler) Me(c *gin.Context) {
	user, err := h.service.Me(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "获取用户信息失败", err.Error())
		return
	}
	response.Success(c, user)
}

func (h *UserHandler) ssoReady() bool {
	return h != nil && h.cfg != nil && h.cfg.KeycloakEnabled && h.keycloak != nil && h.stateManager != nil
}

func (h *UserHandler) setCookie(c *gin.Context, name, value string, maxAge int, path string) {
	c.SetSameSite(sameSiteMode(h.cfg.AuthCookieSameSite))
	c.SetCookie(name, value, maxAge, path, "", h.cfg.AuthCookieSecure, true)
}

func (h *UserHandler) clearCookie(c *gin.Context, name, path string) {
	h.setCookie(c, name, "", -1, path)
}

func (h *UserHandler) oidcStateCookieName(state string) string {
	if !isOIDCStateToken(state) {
		return h.cfg.KeycloakStateCookieName
	}
	return h.cfg.KeycloakStateCookieName + "_" + state
}

func (h *UserHandler) oidcStateCookie(c *gin.Context, state string) (string, string, bool) {
	if !isOIDCStateToken(state) {
		return "", "", false
	}

	name := h.oidcStateCookieName(state)
	value, err := c.Cookie(name)
	if err == nil && value != "" {
		return name, value, true
	}

	// Backward compatibility for users who started SSO before the per-state
	// cookie naming scheme was deployed.
	value, err = c.Cookie(h.cfg.KeycloakStateCookieName)
	if err == nil && value != "" {
		return h.cfg.KeycloakStateCookieName, value, true
	}
	return "", "", false
}

func (h *UserHandler) reusableLoginState(c *gin.Context, returnTo string) (auth.OIDCLoginState, bool) {
	for _, cookie := range c.Request.Cookies() {
		if !h.isOIDCStateCookieName(cookie.Name) || cookie.Value == "" {
			continue
		}
		loginState, err := h.stateManager.Decode(cookie.Value)
		if err != nil || loginState.ReturnTo != returnTo {
			continue
		}
		if cookie.Name == h.oidcStateCookieName(loginState.State) {
			return loginState, true
		}
	}

	value, err := c.Cookie(h.cfg.KeycloakStateCookieName)
	if err != nil || value == "" {
		return auth.OIDCLoginState{}, false
	}
	loginState, err := h.stateManager.Decode(value)
	if err != nil || loginState.ReturnTo != returnTo {
		return auth.OIDCLoginState{}, false
	}
	return loginState, true
}

func (h *UserHandler) isOIDCStateCookieName(name string) bool {
	prefix := h.cfg.KeycloakStateCookieName + "_"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	return isOIDCStateToken(strings.TrimPrefix(name, prefix))
}

func (h *UserHandler) clearOIDCStateCookie(c *gin.Context, name string) {
	if name != "" {
		h.clearCookie(c, name, oidcStateCookiePath)
	}
	if name != h.cfg.KeycloakStateCookieName {
		h.clearCookie(c, h.cfg.KeycloakStateCookieName, oidcStateCookiePath)
	}
}

func (h *UserHandler) safeReturnTo(raw string) string {
	frontend := strings.TrimRight(h.cfg.KeycloakFrontendURL, "/")
	if frontend == "" {
		frontend = "/"
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return frontend
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return frontend + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return frontend
	}
	allowed, err := url.Parse(frontend)
	if err != nil || allowed.Scheme == "" || allowed.Host == "" {
		return frontend
	}
	if parsed.Scheme == allowed.Scheme && parsed.Host == allowed.Host {
		return parsed.String()
	}
	return frontend
}

func isOIDCStateToken(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '-' || ch == '_':
		default:
			return false
		}
	}
	return true
}

func sameSiteMode(value string) http.SameSite {
	switch strings.ToLower(value) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
