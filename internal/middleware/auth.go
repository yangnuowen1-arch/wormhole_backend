package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/response"
)

// Auth 校验本应用会话令牌，成功后把 claims 放入请求 context。
// 首选 Authorization: Bearer <token>，没有该请求头时再读取 HttpOnly 会话 Cookie，
// 因而既兼容旧客户端，也支持 Keycloak BFF 登录后的浏览器会话。
func Auth(secret, sessionCookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := tokenFromRequest(c, sessionCookieName)
		if !ok || tokenStr == "" {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(secret, tokenStr)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, 40101, "登录已失效，请重新登录", nil)
			c.Abort()
			return
		}

		// 把 claims 注入 request context，供 service 层通过 auth.ClaimsFromContext 读取。
		ctx := auth.WithClaims(c.Request.Context(), claims)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func tokenFromRequest(c *gin.Context, sessionCookieName string) (string, bool) {
	if header := strings.TrimSpace(c.GetHeader("Authorization")); header != "" {
		token, ok := strings.CutPrefix(header, "Bearer ")
		return token, ok && token != ""
	}
	if sessionCookieName == "" {
		return "", false
	}
	cookie, err := c.Request.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	return cookie.Value, cookie.Value != ""
}
