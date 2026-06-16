package routes

import (
	"academyprometheus/backend/config"
	dashboardcontroller "academyprometheus/backend/controllers/dashboard"
	publiccontroller "academyprometheus/backend/controllers/public"
	"academyprometheus/backend/middlewares"
	"academyprometheus/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterDashboardRoutes(router *gin.RouterGroup, db *gorm.DB, cfg config.Config) {
	uploadService := services.NewUploadService(db, cfg)
	dashboardController := dashboardcontroller.NewController(db, cfg)
	publicController := publiccontroller.NewController(db, cfg, uploadService)

	protected := router.Group("")
	protected.Use(middlewares.AuthGuard(db, cfg))
	protected.POST("/products/:slug/purchase", publicController.CreateProductPurchase)
	protected.GET("/products/:slug/review-eligibility", publicController.GetProductReviewEligibility)
	protected.POST("/products/:slug/reviews", publicController.CreateProductReview)
	protected.POST("/coupons/apply", publicController.ApplyCoupon)
	protected.POST("/orders/:id/pay", dashboardController.PayOrder)
	protected.POST("/orders/:id/sync", dashboardController.SyncOrder)
	protected.POST("/orders/:id/cancel", dashboardController.CancelOrder)
	protected.GET("/orders/:id/invoice", dashboardController.GetOrderInvoice)
	protected.GET("/product-files/:id/download", dashboardController.DownloadProductFile)
	protected.GET("/consultation/slots", dashboardController.ListConsultationSlots)
	protected.GET("/consultation/bookings", dashboardController.ListConsultationBookings)
	protected.POST("/consultation/slots/:id/book", dashboardController.BookConsultationSlot)
	protected.PATCH("/consultation/bookings/:id", dashboardController.UpdateConsultationBooking)

	student := router.Group("")
	student.Use(middlewares.AuthGuard(db, cfg), middlewares.RoleGuard("student", "admin"))
	student.GET("/dashboard", dashboardController.GetOverview)
	student.POST("/courses/:slug/enroll", dashboardController.CreateCourseEnrollment)
	student.POST("/courses/:slug/reviews", dashboardController.CreateCourseReview)
	student.GET("/courses/:slug/player", dashboardController.GetCoursePlayer)
	student.POST("/topics/:id/complete", dashboardController.CompleteTopic)
	student.POST("/quizzes/:id/submit", dashboardController.SubmitQuiz)
	student.POST("/assignments/:id/submit", dashboardController.SubmitAssignment)
	student.GET("/certificates/:uuid/download", dashboardController.DownloadCertificate)
	student.GET("/talent/jobs", publicController.ListTalentJobs)
	student.GET("/talent/jobs/:slug", publicController.GetTalentJob)
	student.POST("/talent/jobs/:slug/apply", publicController.CreateTalentJobApplication)
}
