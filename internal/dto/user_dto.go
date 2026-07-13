package dto

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
	ID       int64          `json:"id" example:"1"`
	Username string         `json:"username" example:"alice"`
	Email    string         `json:"email" example:"alice@example.com"`
	Nickname string         `json:"nickname" example:"Alice"`
	Roles    []RoleResponse `json:"roles"`
}

// RoleResponse 用户角色信息响应。
type RoleResponse struct {
	ID          int32  `json:"id" example:"1"`
	Code        string `json:"code" example:"admin"`
	Name        string `json:"name" example:"管理员"`
	Description string `json:"description" example:"可以维护资源中心配置"`
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
