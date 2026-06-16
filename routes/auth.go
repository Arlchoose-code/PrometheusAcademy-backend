package routes

import (
	"academyprometheus/backend/config"
	authcontroller "academyprometheus/backend/controllers/auth"
	"academyprometheus/backend/middlewares"
	"academyprometheus/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterAuthRoutes(router *gin.RouterGroup, db *gorm.DB, cfg config.Config) {
	uploadService := services.NewUploadService(db, cfg)
	authController := authcontroller.NewController(db, cfg, uploadService)

	auth := router.Group("/auth")
	auth.POST("/register", authController.CreateUser)
	auth.POST("/login", authController.Login)
	auth.POST("/refresh", authController.Refresh)
	auth.POST("/logout", authController.Logout)

	protected := router.Group("")
	protected.Use(middlewares.AuthGuard(db, cfg))
	protected.GET("/auth/me", authController.GetCurrentUser)
	protected.PATCH("/auth/profile", authController.UpdateProfile)
	protected.POST("/auth/avatar", authController.CreateAvatar)
}
