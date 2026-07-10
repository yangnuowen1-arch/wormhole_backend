package router

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/config"
	"github.com/yang/wormhole_backend/internal/handler"
	"github.com/yang/wormhole_backend/internal/middleware"
	"github.com/yang/wormhole_backend/internal/repository"
	"github.com/yang/wormhole_backend/internal/service"
	"gorm.io/gorm"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/yang/wormhole_backend/docs"
)

// SetupRouter 手动装配依赖并注册路由。
func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	// Swagger 文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 依赖装配：repo → service → handler
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, cfg)
	userHandler, err := handler.NewUserHandler(userService, cfg)
	if err != nil {
		log.Fatalf("认证模块初始化失败: %v", err)
	}

	api := r.Group("/api/v1")

	// 公开接口（无需鉴权）
	authGroup := api.Group("/auth")
	{
		if cfg.KeycloakEnabled {
			authGroup.GET("/sso/login", userHandler.StartSSO)
			authGroup.GET("/sso/callback", userHandler.CallbackSSO)
			authGroup.POST("/logout", userHandler.Logout)
		} else {
			authGroup.POST("/register", userHandler.Register)
			authGroup.POST("/login", userHandler.Login)
		}
	}

	// 需登录接口
	private := api.Group("")
	private.Use(middleware.Auth(cfg.JWTSecret, cfg.AuthCookieName))
	{
		private.GET("/users/me", userHandler.Me)
	}

	return r
}
