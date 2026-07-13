package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/response"
)

const RoleAdmin = "admin"

// RoleFinder 提供用户角色查询能力，repository.UserRepository 已实现该接口。
type RoleFinder interface {
	FindRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error)
}

// RequireRole 要求当前登录用户拥有指定角色。
// 该中间件必须放在 Auth 后面，因为它依赖 Auth 注入到 request context 的 claims。
func RequireRole(finder RoleFinder, roleCode string) gin.HandlerFunc {
	roleCode = strings.TrimSpace(roleCode)
	return func(c *gin.Context) {
		claims := auth.ClaimsFromContext(c.Request.Context())
		if claims == nil || claims.UserID == 0 {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			c.Abort()
			return
		}
		if finder == nil || roleCode == "" {
			response.Error(c, http.StatusInternalServerError, 50001, "权限模块未初始化", nil)
			c.Abort()
			return
		}

		roles, err := finder.FindRolesByUserID(c.Request.Context(), claims.UserID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, 50001, "权限校验失败", err.Error())
			c.Abort()
			return
		}
		for _, role := range roles {
			if strings.EqualFold(role.Code, roleCode) {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, 40301, "没有权限", nil)
		c.Abort()
	}
}

// RequireAdmin 要求当前登录用户拥有 admin 角色。
func RequireAdmin(finder RoleFinder) gin.HandlerFunc {
	return RequireRole(finder, RoleAdmin)
}
