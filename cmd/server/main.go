package main

import (
	"log"

	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/db"
	"github.com/yang/wormhole_backend/internal/router"
)

// @title        Wormhole Backend API
// @version      1.0
// @description  Wormhole 后端服务 API 文档。Keycloak SSO 模式下，前端通过 /auth/sso/login 发起登录，后端写入 HttpOnly Cookie；前端鉴权判断请调用 /users/me 并携带 credentials: include。普通业务接口要求登录，/admin 开头的管理接口额外要求当前用户拥有 admin 角色。
// @BasePath     /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 兼容 Bearer token：Authorization: Bearer <token>
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name wormhole_session
// @description SSO 登录成功后由后端写入的 HttpOnly 会话 Cookie，前端 JS 不能读取，fetch/axios 请求需携带 credentials/withCredentials。
func main() {
	cfg := config.LoadConfig()

	database := db.ConnectDB(cfg)

	app := router.SetupRouter(database, cfg)

	log.Printf("服务启动，监听端口 :%s", cfg.AppPort)
	if err := app.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
