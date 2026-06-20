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
	auth.POST("/register", middlewares.RateLimit(cfg.AuthRateLimitPerMinute), authController.CreateUser)
	auth.POST("/login", middlewares.RateLimit(cfg.AuthRateLimitPerMinute), authController.Login)
	auth.POST("/otp/verify", middlewares.RateLimit(cfg.OTPVerifyRatePerMinute), authController.VerifyAuthOTP)
	auth.POST("/otp/resend", middlewares.RateLimit(cfg.OTPResendRatePerMinute), authController.ResendAuthOTP)
	auth.POST("/password-reset/request", middlewares.RateLimit(cfg.PasswordRatePerMinute), authController.RequestPasswordReset)
	auth.POST("/password-reset/confirm", middlewares.RateLimit(cfg.PasswordRatePerMinute), authController.ConfirmPasswordReset)
	auth.POST("/refresh", authController.Refresh)
	auth.POST("/logout", authController.Logout)

	protected := router.Group("")
	protected.Use(middlewares.AuthGuard(db, cfg))
	protected.GET("/auth/me", authController.GetCurrentUser)
	protected.PATCH("/auth/profile", authController.UpdateProfile)
	protected.POST("/auth/avatar", authController.CreateAvatar)
}
