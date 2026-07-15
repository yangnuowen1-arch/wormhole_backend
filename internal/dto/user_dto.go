package dto

import "time"

// RegisterRequest 注册请求体。
// 注：数据库已改为 Keycloak 方案，users 表无 password 列，此处不再收集密码。
type RegisterRequest struct {
	// 用户名，3-64 个字符。
	Username string `json:"username" binding:"required,min=3,max=64" example:"alice"`
	// 邮箱，可选。
	Email string `json:"email" binding:"omitempty,email,max=128" example:"alice@example.com"`
	// 昵称，可选。
	Nickname string `json:"nickname" example:"Alice"`
}

// LoginRequest 登录请求体。
type LoginRequest struct {
	// 用户名。仅 KEYCLOAK_ENABLED=false 的兼容模式使用。
	Username string `json:"username" binding:"required" example:"alice"`
}

// LoginResponse 登录成功返回。
type LoginResponse struct {
	// 后端签发的应用 JWT。SSO 模式下主要写入 HttpOnly Cookie，前端通常不用读取。
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	// 当前用户资料。
	User UserResponse `json:"user"`
}

// UserResponse 用户信息响应。
type UserResponse struct {
	ID          int64          `json:"id" example:"1"`
	KeycloakID  string         `json:"keycloakId" example:"65b336b8-b4c5-4bce-bb36-7831fc22b641"`
	Username    string         `json:"username" example:"alice"`
	Email       string         `json:"email" example:"alice@example.com"`
	Nickname    string         `json:"nickname" example:"Alice"`
	Avatar      string         `json:"avatar" example:"https://example.com/avatar.png"`
	Status      int16          `json:"status" example:"1"`
	LastLoginAt *time.Time     `json:"lastLoginAt"`
	CreatedAt   *time.Time     `json:"createdAt"`
	UpdatedAt   *time.Time     `json:"updatedAt"`
	Roles       []RoleResponse `json:"roles"`
}

// RoleResponse 用户角色信息响应。
type RoleResponse struct {
	ID          int32  `json:"id" example:"1"`
	Code        string `json:"code" example:"admin"`
	Name        string `json:"name" example:"管理员"`
	Description string `json:"description" example:"可以维护资源中心配置"`
}

// AssignUserRolesRequest 管理员为指定用户设置的完整角色集合。
// 角色使用 roles.code，而不是会随环境变化的数据库主键。
type AssignUserRolesRequest struct {
	RoleCodes []string `json:"roleCodes" binding:"required,min=1,max=20,dive,required,max=32"`
}

// CreateAdminUserRequest 管理员创建用户。启用 Keycloak SSO 时，keycloakId 必须是
// Keycloak 中该用户的 sub；本服务只管理本地业务用户资料和角色，不创建 Keycloak 账号。
type CreateAdminUserRequest struct {
	KeycloakID string   `json:"keycloakId" binding:"omitempty,max=64" example:"65b336b8-b4c5-4bce-bb36-7831fc22b641"`
	Username   string   `json:"username" binding:"required,min=3,max=64" example:"alice"`
	Email      string   `json:"email" binding:"omitempty,email,max=128" example:"alice@example.com"`
	Nickname   string   `json:"nickname" binding:"omitempty,max=64" example:"Alice"`
	Avatar     string   `json:"avatar" binding:"omitempty,max=255" example:"https://example.com/avatar.png"`
	Status     *int16   `json:"status" binding:"omitempty,oneof=0 1" example:"1"`
	RoleCodes  []string `json:"roleCodes" binding:"omitempty,max=20,dive,required,max=32" example:"user"`
}

// UpdateAdminUserRequest 管理员部分更新用户资料。字段未传时不会修改；传入空字符串
// 可以清空 email、nickname、avatar。
type UpdateAdminUserRequest struct {
	Username *string `json:"username" example:"alice"`
	Email    *string `json:"email" example:"alice@example.com"`
	Nickname *string `json:"nickname" example:"Alice"`
	Avatar   *string `json:"avatar" example:"https://example.com/avatar.png"`
	Status   *int16  `json:"status" binding:"omitempty,oneof=0 1" example:"1"`
}

// DeleteAdminUserResponse 管理员删除（逻辑停用）用户的响应。
type DeleteAdminUserResponse struct {
	Deleted bool `json:"deleted" example:"true"`
}

// LogoutResponse 退出登录响应数据。
type LogoutResponse struct {
	LoggedOut bool `json:"logged_out" example:"true"`
}

// UserAPIResponse 用户响应包裹体。
type UserAPIResponse struct {
	Code      int          `json:"code" example:"0"`
	Message   string       `json:"message" example:"success"`
	Data      UserResponse `json:"data"`
	Error     *string      `json:"error" example:""`
	RequestID string       `json:"requestId" example:"5547e998-1127-4c9d-ae7e-f3508c42b96c"`
	Timestamp string       `json:"timestamp" example:"2026-07-10T16:30:01+08:00"`
}

// UserListAPIResponse 用户列表响应包裹体。
type UserListAPIResponse struct {
	Code      int            `json:"code" example:"0"`
	Message   string         `json:"message" example:"success"`
	Data      []UserResponse `json:"data"`
	Error     *string        `json:"error" example:""`
	RequestID string         `json:"requestId" example:"5547e998-1127-4c9d-ae7e-f3508c42b96c"`
	Timestamp string         `json:"timestamp" example:"2026-07-10T16:30:01+08:00"`
}

// DeleteAdminUserAPIResponse 删除用户响应包裹体。
type DeleteAdminUserAPIResponse struct {
	Code      int                     `json:"code" example:"0"`
	Message   string                  `json:"message" example:"success"`
	Data      DeleteAdminUserResponse `json:"data"`
	Error     *string                 `json:"error" example:""`
	RequestID string                  `json:"requestId" example:"5547e998-1127-4c9d-ae7e-f3508c42b96c"`
	Timestamp string                  `json:"timestamp" example:"2026-07-10T16:30:01+08:00"`
}

// LoginAPIResponse 登录响应包裹体。
type LoginAPIResponse struct {
	Code      int           `json:"code" example:"0"`
	Message   string        `json:"message" example:"success"`
	Data      LoginResponse `json:"data"`
	Error     *string       `json:"error" example:""`
	RequestID string        `json:"requestId" example:"5547e998-1127-4c9d-ae7e-f3508c42b96c"`
	Timestamp string        `json:"timestamp" example:"2026-07-10T16:30:01+08:00"`
}

// LogoutAPIResponse 退出登录响应包裹体。
type LogoutAPIResponse struct {
	Code      int            `json:"code" example:"0"`
	Message   string         `json:"message" example:"success"`
	Data      LogoutResponse `json:"data"`
	Error     *string        `json:"error" example:""`
	RequestID string         `json:"requestId" example:"5547e998-1127-4c9d-ae7e-f3508c42b96c"`
	Timestamp string         `json:"timestamp" example:"2026-07-10T16:30:01+08:00"`
}

// ErrorAPIResponse 错误响应包裹体。
type ErrorAPIResponse struct {
	Code      int     `json:"code" example:"40101"`
	Message   string  `json:"message" example:"未登录，请先登录"`
	Data      *string `json:"data" example:""`
	Error     string  `json:"error" example:"token is expired"`
	RequestID string  `json:"requestId" example:"5547e998-1127-4c9d-ae7e-f3508c42b96c"`
	Timestamp string  `json:"timestamp" example:"2026-07-10T16:30:01+08:00"`
}
