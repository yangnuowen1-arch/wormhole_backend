package service

import (
	"context"
	"errors"

	"github.com/yang/wormhole_backend/internal/auth"
)

// 跨层需要区分的哨兵错误，handler 用 errors.Is 判断并映射错误码。
var (
	// ErrUnauthenticated 未登录 / 无法从 context 取到当前用户。
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrUsernameTaken 用户名已被占用。
	ErrUsernameTaken = errors.New("username already taken")
	// ErrInvalidCredentials 用户名或密码错误。
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidKeycloakIdentity Keycloak 返回的身份缺少必要字段或超过本地约束。
	ErrInvalidKeycloakIdentity = errors.New("invalid Keycloak identity")
	// ErrKeycloakUsernameConflict 无法为首次 SSO 用户安全分配唯一的本地用户名。
	ErrKeycloakUsernameConflict = errors.New("Keycloak username conflict")
	// ErrForbidden 当前用户没有执行该操作的权限。
	ErrForbidden = errors.New("forbidden")
	// ErrQuickEntryNotFound 快速入口不存在。
	ErrQuickEntryNotFound = errors.New("quick entry not found")
	// ErrInvalidQuickEntry 快速入口参数不合法。
	ErrInvalidQuickEntry = errors.New("invalid quick entry")
	// ErrInvalidStatus 状态值不合法。
	ErrInvalidStatus = errors.New("invalid status")
	// ErrRecommendationItemNotFound 今日推荐不存在。
	ErrRecommendationItemNotFound = errors.New("recommendation item not found")
	// ErrInvalidRecommendationItem 今日推荐参数不合法。
	ErrInvalidRecommendationItem = errors.New("invalid recommendation item")
	// ErrCarouselSlideNotFound 幻灯片不存在。
	ErrCarouselSlideNotFound = errors.New("carousel slide not found")
	// ErrInvalidCarouselSlide 幻灯片参数不合法。
	ErrInvalidCarouselSlide = errors.New("invalid carousel slide")
	// ErrInvalidTimeRange 时间窗口不合法。
	ErrInvalidTimeRange = errors.New("invalid time range")
)

// currentUserID 从 context 取当前登录用户 ID，取不到返回 ErrUnauthenticated。
func currentUserID(ctx context.Context) (int64, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil || claims.UserID == 0 {
		return 0, ErrUnauthenticated
	}
	return claims.UserID, nil
}
