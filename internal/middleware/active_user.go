package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/response"
)

// ActiveUserFinder 提供用户状态查询能力，repository.UserRepository 已实现该接口。
type ActiveUserFinder interface {
	FindByID(ctx context.Context, id int64) (*model.User, error)
}

// RequireActiveUser 拒绝已被管理员停用或已逻辑删除的用户。必须置于 Auth 之后，
// 因为它依赖 Auth 写入 request context 的用户 claims。
func RequireActiveUser(finder ActiveUserFinder) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := auth.ClaimsFromContext(c.Request.Context())
		if claims == nil || claims.UserID <= 0 {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			c.Abort()
			return
		}
		if finder == nil {
			response.Error(c, http.StatusInternalServerError, 50001, "用户状态模块未初始化", nil)
			c.Abort()
			return
		}

		user, err := finder.FindByID(c.Request.Context(), claims.UserID)
		if err != nil || user == nil {
			response.Error(c, http.StatusUnauthorized, 40101, "登录已失效，请重新登录", nil)
			c.Abort()
			return
		}
		if user.Status != nil && *user.Status != 1 {
			response.Error(c, http.StatusForbidden, 40302, "账号已停用", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
