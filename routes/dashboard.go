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
	protected.Use(middlewares.AuthGuard(db, cfg), middlewares.RoleGuard("student", "admin"))
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
	protected.GET("/communications", dashboardController.ListCommunications)
	protected.POST("/communications", dashboardController.CreateCommunication)
	protected.GET("/communications/:id/messages", dashboardController.ListCommunicationMessages)
	protected.POST("/communications/:id/messages", dashboardController.ReplyCommunication)
	protected.GET("/notifications", dashboardController.ListNotifications)
	protected.POST("/notifications/mark-all-read", dashboardController.MarkAllNotificationsRead)
	protected.GET("/talent/apply-eligibility", publicController.GetTalentApplyEligibility)

	company := router.Group("")
	company.Use(middlewares.AuthGuard(db, cfg), middlewares.RoleGuard("company", "admin"))
	company.GET("/talent/company-dashboard", publicController.GetCompanyTalentDashboard)
	company.PATCH("/talent/company-applications/:id", publicController.UpdateCompanyCandidateDecision)
	company.GET("/talent/company-applications/:id/cv", publicController.DownloadCompanyCandidateCV)

	student := router.Group("")
	student.Use(middlewares.AuthGuard(db, cfg), middlewares.RoleGuard("student", "admin"))
	student.GET("/dashboard", dashboardController.GetOverview)
	student.POST("/courses/:slug/enroll", dashboardController.CreateCourseEnrollment)
	student.POST("/courses/:slug/reviews", dashboardController.CreateCourseReview)
	student.GET("/courses/:slug/player", dashboardController.GetCoursePlayer)
	student.POST("/topics/:id/complete", dashboardController.CompleteTopic)
	student.POST("/quizzes/:id/submit", dashboardController.SubmitQuiz)
	student.POST("/assignments/:id/submit", dashboardController.SubmitAssignment)
	student.POST("/events/:id/attend", dashboardController.AttendEvent)
	student.GET("/certificates/:uuid/download", dashboardController.DownloadCertificate)
	student.POST("/talent/jobs/:slug/apply", publicController.CreateTalentJobApplication)
}
