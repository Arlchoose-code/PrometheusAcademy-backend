package routes

import (
	"academyprometheus/backend/config"
	publiccontroller "academyprometheus/backend/controllers/public"
	"academyprometheus/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterPublicRoutes(router *gin.RouterGroup, db *gorm.DB, cfg config.Config) {
	uploadService := services.NewUploadService(db, cfg)
	publicController := publiccontroller.NewController(db, cfg, uploadService)

	router.GET("/health", publicController.GetHealth)
	router.GET("/uploads/*filepath", publicController.ServeUpload)
	router.GET("/settings/public", publicController.GetPublicSettings)
	router.GET("/homepage", publicController.GetHomepage)
	router.POST("/newsletter", publicController.CreateNewsletterSubscription)
	router.POST("/contact", publicController.CreateContactLead)
	router.GET("/faqs/public", publicController.ListPublicFAQs)
	router.GET("/banners/public", publicController.ListPublicBanners)
	router.GET("/pages/:slug", publicController.GetPage)
	router.GET("/seo/:slug", publicController.GetSEO)
	router.GET("/course-categories", publicController.ListCourseCategories)
	router.GET("/courses", publicController.ListCourses)
	router.GET("/courses/:slug", publicController.GetCourse)
	router.GET("/product-categories", publicController.ListProductCategories)
	router.GET("/products", publicController.ListProducts)
	router.GET("/products/:slug", publicController.GetProduct)
	router.POST("/payments/midtrans/webhook", publicController.HandleMidtransWebhook)

	registerTalentPublicRoutes(router, db, cfg, uploadService)
}

func registerTalentPublicRoutes(router *gin.RouterGroup, db *gorm.DB, cfg config.Config, uploadService *services.UploadService) {
	publicController := publiccontroller.NewController(db, cfg, uploadService)
	router.GET("/talent/landing", publicController.GetTalentLanding)
	router.POST("/talent/hiring", publicController.CreateHiringInquiry)
	router.POST("/talent/plus", publicController.CreateTalentPlusApplication)
	router.GET("/partners", publicController.ListPartners)
	router.POST("/partner/apply", publicController.CreatePartnerApplication)
	router.GET("/events", publicController.ListEvents)
}
