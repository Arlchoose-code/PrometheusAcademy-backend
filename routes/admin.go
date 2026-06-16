package routes

import (
	"academyprometheus/backend/config"
	admincontroller "academyprometheus/backend/controllers/admin"
	"academyprometheus/backend/middlewares"
	"academyprometheus/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterAdminRoutes(router *gin.RouterGroup, db *gorm.DB, cfg config.Config) {
	admin := router.Group("/admin")
	admin.Use(middlewares.AdminNotFoundGuard(db, cfg))
	uploadService := services.NewUploadService(db, cfg)
	adminController := admincontroller.NewController(db, cfg, uploadService)

	admin.GET("/overview", adminController.GetOverview)

	admin.GET("/contact-leads", adminController.ListContactLeads)
	admin.PUT("/contact-leads/:id", adminController.UpdateContactLead)
	admin.GET("/newsletter", adminController.ListNewsletterSubscribers)

	admin.GET("/users", adminController.ListUsers)

	admin.GET("/notifications", adminController.ListNotifications)
	admin.POST("/notifications/mark-all-read", adminController.MarkAllNotificationsRead)

	admin.POST("/users/:id/reset-password", adminController.ResetUserPassword)
	admin.PATCH("/users/:id/role", adminController.UpdateUserRole)

	admin.POST("/settings/logo", adminController.UpdateSiteLogo)
	admin.GET("/settings", adminController.ListSettings)
	admin.PATCH("/settings", adminController.UpdateSettings)
	admin.POST("/settings/favicon", adminController.UpdateFavicon)

	admin.GET("/seo", adminController.ListSEO)
	admin.PUT("/seo/:slug", adminController.UpdateSEO)
	admin.POST("/seo/:slug/og-image", adminController.UpdateSEOImage)
	admin.GET("/email-templates", adminController.ListEmailTemplates)
	admin.PUT("/email-templates/:key", adminController.UpdateEmailTemplate)

	registerCourseAdminRoutes(admin, adminController)
	registerCMSAdminRoutes(admin, adminController)
	registerCommerceAdminRoutes(admin, adminController)
	registerConsultationAdminRoutes(admin, adminController)
	registerTalentAdminRoutes(admin, adminController)
}

func registerCourseAdminRoutes(admin *gin.RouterGroup, adminController *admincontroller.Controller) {
	admin.GET("/course-categories", adminController.ListCourseCategories)
	admin.POST("/course-categories", adminController.CreateCourseCategory)
	admin.PUT("/course-categories/:id", adminController.UpdateCourseCategory)
	admin.DELETE("/course-categories/:id", adminController.DeleteCourseCategory)

	admin.GET("/courses", adminController.ListCourses)

	admin.GET("/courses/:id", adminController.GetCourse)

	admin.POST("/courses", adminController.CreateCourse)
	admin.PUT("/courses/:id", adminController.UpdateCourse)
	admin.DELETE("/courses/:id", adminController.DeleteCourse)
	admin.POST("/courses/:id/thumbnail", adminController.UpdateCourseThumbnail)

	admin.POST("/courses/:id/modules", adminController.CreateCourseModule)
	admin.PUT("/modules/:id", adminController.UpdateCourseModule)
	admin.DELETE("/modules/:id", adminController.DeleteCourseModule)
	admin.PUT("/modules/reorder", adminController.ReorderCourseModules)

	admin.POST("/modules/:id/topics", adminController.CreateTopic)
	admin.PUT("/topics/:id", adminController.UpdateTopic)
	admin.DELETE("/topics/:id", adminController.DeleteTopic)
	admin.PUT("/topics/reorder", adminController.ReorderTopics)
	admin.POST("/topics/:id/attachments", adminController.CreateTopicAttachment)
	admin.DELETE("/topic-attachments/:id", adminController.DeleteTopicAttachment)
	admin.POST("/topics/:id/assignments", adminController.CreateAssignment)
	admin.PUT("/assignments/:id", adminController.UpdateAssignment)
	admin.DELETE("/assignments/:id", adminController.DeleteAssignment)
	admin.GET("/assignments/:id/submissions", adminController.ListAssignmentSubmissions)
	admin.PUT("/assignment-submissions/:id", adminController.UpdateAssignmentSubmission)

	admin.POST("/topics/:id/blocks", adminController.CreateTopicBlock)
	admin.PUT("/topic-blocks/:id", adminController.UpdateTopicBlock)
	admin.DELETE("/topic-blocks/:id", adminController.DeleteTopicBlock)
	admin.PUT("/topic-blocks/reorder", adminController.ReorderTopicBlocks)
	admin.POST("/topic-blocks/:id/file", adminController.UpdateTopicBlockFile)

	admin.POST("/modules/:id/quizzes", adminController.CreateQuiz)
	admin.PUT("/modules/:id/quiz", adminController.UpdateModuleQuiz)
	admin.PUT("/quizzes/:id", adminController.UpdateQuiz)
	admin.DELETE("/quizzes/:id", adminController.DeleteQuiz)
	admin.PUT("/quizzes/reorder", adminController.ReorderQuizzes)
	admin.POST("/quizzes/:id/questions", adminController.CreateQuizQuestion)
	admin.PUT("/quiz-questions/:id", adminController.UpdateQuizQuestion)
	admin.DELETE("/quiz-questions/:id", adminController.DeleteQuizQuestion)
	admin.POST("/quiz-questions/:id/answers", adminController.CreateQuizAnswer)
	admin.PUT("/quiz-answers/:id", adminController.UpdateQuizAnswer)
	admin.DELETE("/quiz-answers/:id", adminController.DeleteQuizAnswer)
	admin.GET("/quizzes/:id/submissions", adminController.ListQuizSubmissions)
	admin.PUT("/quiz-submissions/:id/review", adminController.ReviewQuizSubmission)

	admin.PUT("/courses/:id/drip-schedules", adminController.UpdateCourseDripSchedules)
}

func registerCMSAdminRoutes(admin *gin.RouterGroup, adminController *admincontroller.Controller) {
	admin.GET("/cms/pages", adminController.ListPages)
	admin.PUT("/cms/pages/:slug", adminController.UpdatePage)
	admin.POST("/cms/pages/:slug/image", adminController.UpdatePageImage)

	admin.GET("/cms/faqs", adminController.ListFAQs)
	admin.POST("/cms/faqs", adminController.CreateFAQ)
	admin.PUT("/cms/faqs/reorder", adminController.ReorderFAQs)
	admin.PUT("/cms/faqs/:id", adminController.UpdateFAQ)
	admin.DELETE("/cms/faqs/:id", adminController.DeleteFAQ)

	admin.GET("/cms/testimonials", adminController.ListTestimonials)
	admin.POST("/cms/testimonials", adminController.CreateTestimonial)
	admin.PUT("/cms/testimonials/:id", adminController.UpdateTestimonial)
	admin.DELETE("/cms/testimonials/:id", adminController.DeleteTestimonial)
	admin.POST("/cms/testimonials/:id/avatar", adminController.UpdateTestimonialAvatar)

	admin.GET("/cms/banners", adminController.ListBanners)
	admin.POST("/cms/banners", adminController.CreateBanner)
	admin.PUT("/cms/banners/:id", adminController.UpdateBanner)
	admin.DELETE("/cms/banners/:id", adminController.DeleteBanner)
	admin.POST("/cms/banners/:id/image", adminController.UpdateBannerImage)

	admin.GET("/media", adminController.ListMedia)
	admin.POST("/media", adminController.CreateMedia)
	admin.DELETE("/media/:id", adminController.DeleteMedia)
}

func registerCommerceAdminRoutes(admin *gin.RouterGroup, adminController *admincontroller.Controller) {
	admin.GET("/product-categories", adminController.ListProductCategories)
	admin.POST("/product-categories", adminController.CreateProductCategory)
	admin.PUT("/product-categories/:id", adminController.UpdateProductCategory)
	admin.DELETE("/product-categories/:id", adminController.DeleteProductCategory)

	admin.GET("/products", adminController.ListProducts)
	admin.POST("/products", adminController.CreateProduct)
	admin.PUT("/products/:id", adminController.UpdateProduct)
	admin.DELETE("/products/:id", adminController.DeleteProduct)
	admin.POST("/products/:id/thumbnail", adminController.UpdateProductThumbnail)
	admin.GET("/products/:id/files", adminController.ListProductFiles)
	admin.POST("/products/:id/files", adminController.CreateProductFile)
	admin.DELETE("/product-files/:id", adminController.DeleteProductFile)

	admin.GET("/transactions", adminController.ListTransactions)
	admin.GET("/coupons", adminController.ListCoupons)
	admin.POST("/coupons", adminController.CreateCoupon)
	admin.PUT("/coupons/:id", adminController.UpdateCoupon)
	admin.DELETE("/coupons/:id", adminController.DeleteCoupon)
}

func registerConsultationAdminRoutes(admin *gin.RouterGroup, adminController *admincontroller.Controller) {
	admin.GET("/consultations/settings", adminController.GetConsultationSettings)
	admin.PUT("/consultations/settings", adminController.UpdateConsultationSettings)
	admin.GET("/consultations/slots", adminController.ListConsultationSlots)
	admin.POST("/consultations/slots", adminController.CreateConsultationSlot)
	admin.PUT("/consultations/slots/:id", adminController.UpdateConsultationSlot)
	admin.DELETE("/consultations/slots/:id", adminController.DeleteConsultationSlot)
	admin.GET("/consultations/bookings", adminController.ListConsultationBookings)
	admin.PUT("/consultations/bookings/:id", adminController.UpdateConsultationBooking)
}

func registerTalentAdminRoutes(admin *gin.RouterGroup, adminController *admincontroller.Controller) {
	admin.GET("/talent/jobs", adminController.ListTalentJobs)
	admin.POST("/talent/jobs", adminController.CreateTalentJob)
	admin.PUT("/talent/jobs/:id", adminController.UpdateTalentJob)
	admin.DELETE("/talent/jobs/:id", adminController.DeleteTalentJob)

	admin.GET("/talent/hiring", adminController.ListHiringInquiries)
	admin.PUT("/talent/hiring/:id", adminController.UpdateHiringInquiry)
	admin.GET("/talent/plus", adminController.ListTalentPlusApplications)
	admin.PUT("/talent/plus/:id", adminController.UpdateTalentPlusApplication)
	admin.GET("/talent/applications", adminController.ListTalentApplications)
	admin.GET("/talent/jobs/:id/applications", adminController.ListTalentJobApplications)
	admin.PUT("/talent/applications/:id", adminController.UpdateTalentApplication)
	admin.GET("/talent/applications/:id/cv", adminController.DownloadTalentApplicationCV)

	admin.GET("/partner/applications", adminController.ListPartnerApplications)
	admin.PUT("/partner/applications/:id", adminController.UpdatePartnerApplication)
	admin.GET("/partners", adminController.ListPartners)
	admin.POST("/partners", adminController.CreatePartner)
	admin.PUT("/partners/:id", adminController.UpdatePartner)
	admin.DELETE("/partners/:id", adminController.DeletePartner)
	admin.POST("/partners/:id/logo", adminController.UpdatePartnerLogo)

	admin.GET("/lead-notes/:type/:id", adminController.ListLeadNotes)
	admin.POST("/lead-notes/:type/:id", adminController.CreateLeadNote)

	admin.GET("/events", adminController.ListEvents)
	admin.POST("/events", adminController.CreateEvent)
	admin.PUT("/events/:id", adminController.UpdateEvent)
	admin.DELETE("/events/:id", adminController.DeleteEvent)
}
