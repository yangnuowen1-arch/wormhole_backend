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
	resourceRepo := repository.NewResourceRepository(db)
	resourceService := service.NewResourceService(resourceRepo)
	searchHistoryRepo := repository.NewSearchHistoryRepository(db)
	searchHistoryService := service.NewSearchHistoryService(searchHistoryRepo)
	commonToolRepo := repository.NewCommonToolRepository(db)
	commonToolService := service.NewCommonToolService(commonToolRepo, resourceRepo)
	quickEntryRepo := repository.NewQuickEntryRepository(db)
	quickEntryService := service.NewQuickEntryService(quickEntryRepo, userRepo)
	recommendationRepo := repository.NewRecommendationItemRepository(db)
	recommendationService := service.NewRecommendationItemService(recommendationRepo, userRepo, resourceRepo)
	carouselSlideRepo := repository.NewCarouselSlideRepository(db)
	carouselSlideService := service.NewCarouselSlideService(carouselSlideRepo, userRepo)
	resourceHandler := handler.NewResourceHandler(resourceService, searchHistoryService, commonToolService, quickEntryService, recommendationService, carouselSlideService)

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
		private.GET("/resource-categories", resourceHandler.ListCategories)

		resources := private.Group("/resources")
		{
			resources.GET("", resourceHandler.ListResources)
			resources.GET("/search", resourceHandler.SearchResources)
			resources.GET("/:identifier", resourceHandler.GetResource)
		}

		searchHistory := private.Group("/search-history")
		{
			searchHistory.POST("", resourceHandler.RecordSearchHistory)
			searchHistory.GET("/recent", resourceHandler.ListRecentSearchHistory)
			searchHistory.DELETE("", resourceHandler.ClearSearchHistory)
		}

		commonTools := private.Group("/common-tools")
		{
			commonTools.GET("", resourceHandler.ListCommonTools)
			commonTools.POST("", resourceHandler.AddCommonTool)
			commonTools.DELETE("/:resourceId", resourceHandler.RemoveCommonTool)
			commonTools.PUT("/sort", resourceHandler.SortCommonTools)
		}

		private.GET("/quick-entries", resourceHandler.ListQuickEntries)
		private.GET("/recommendations", resourceHandler.ListRecommendations)
		private.GET("/carousel-slides", resourceHandler.ListCarouselSlides)

		admin := private.Group("/admin")
		admin.Use(middleware.RequireAdmin(userRepo))

		adminQuickEntries := admin.Group("/quick-entries")
		{
			adminQuickEntries.GET("", resourceHandler.AdminListQuickEntries)
			adminQuickEntries.POST("", resourceHandler.AdminCreateQuickEntry)
			adminQuickEntries.PATCH("/:id", resourceHandler.AdminUpdateQuickEntry)
			adminQuickEntries.PUT("/sort", resourceHandler.AdminSortQuickEntries)
			adminQuickEntries.PATCH("/:id/status", resourceHandler.AdminUpdateQuickEntryStatus)
		}

		adminRecommendations := admin.Group("/recommendations")
		{
			adminRecommendations.GET("", resourceHandler.AdminListRecommendations)
			adminRecommendations.POST("", resourceHandler.AdminCreateRecommendation)
			adminRecommendations.PATCH("/:id", resourceHandler.AdminUpdateRecommendation)
			adminRecommendations.PUT("/sort", resourceHandler.AdminSortRecommendations)
			adminRecommendations.PATCH("/:id/status", resourceHandler.AdminUpdateRecommendationStatus)
		}

		adminCarouselSlides := admin.Group("/carousel-slides")
		{
			adminCarouselSlides.GET("", resourceHandler.AdminListCarouselSlides)
			adminCarouselSlides.POST("", resourceHandler.AdminCreateCarouselSlide)
			adminCarouselSlides.PATCH("/:id", resourceHandler.AdminUpdateCarouselSlide)
			adminCarouselSlides.PUT("/sort", resourceHandler.AdminSortCarouselSlides)
			adminCarouselSlides.PATCH("/:id/status", resourceHandler.AdminUpdateCarouselSlideStatus)
		}
	}

	return r
}
