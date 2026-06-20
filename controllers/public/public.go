package public

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Controller struct {
	db              *gorm.DB
	cfg             config.Config
	uploadService   *services.UploadService
	settingsService services.SettingsService
	homepageService services.HomepageService
}

func NewController(db *gorm.DB, cfg config.Config, uploadService *services.UploadService) *Controller {
	return &Controller{
		db:              db,
		cfg:             cfg,
		uploadService:   uploadService,
		settingsService: services.NewSettingsService(db),
		homepageService: services.NewHomepageService(db),
	}
}

func (h *Controller) GetHealth(c *gin.Context) {
	dbStatus := "disconnected"
	if h.db != nil {
		if sqlDB, err := h.db.DB(); err == nil && sqlDB.Ping() == nil {
			dbStatus = "connected"
		}
	}

	c.JSON(http.StatusOK, structs.Response{
		Success: true,
		Message: "API is healthy",
		Data: gin.H{
			"database": dbStatus,
		},
	})
}

func (h *Controller) ServeUpload(c *gin.Context) {
	cleanPath := path.Clean("/" + c.Param("filepath"))
	if strings.HasPrefix(cleanPath, "/certificates/") || strings.HasPrefix(cleanPath, "/product-files/") || strings.HasPrefix(cleanPath, "/invoices/") {
		c.Status(http.StatusNotFound)
		return
	}
	effectiveCfg := services.EffectiveStorageConfig(c.Request.Context(), h.db, h.cfg)
	publicPath := "/uploads/" + strings.TrimPrefix(cleanPath, "/")
	reader, info, err := services.OpenStoredPublicPath(c.Request.Context(), h.db, h.cfg, publicPath)
	if err == nil {
		defer reader.Close()
		if info.ContentType != "" {
			c.Header("Content-Type", info.ContentType)
		}
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = io.Copy(c.Writer, reader)
		return
	}
	c.File(filepath.Join(effectiveCfg.StoragePath, "uploads", filepath.FromSlash(strings.TrimPrefix(cleanPath, "/"))))
}

func (h *Controller) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingsService.GetPublicSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{
			Success: false,
			Message: "Failed to load public settings",
		})
		return
	}

	c.JSON(http.StatusOK, structs.Response{
		Success: true,
		Message: "Public settings loaded",
		Data:    settings,
	})
}

func (h *Controller) GetHomepage(c *gin.Context) {
	homepage, err := h.homepageService.GetHomepage(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{
			Success: false,
			Message: "Failed to load homepage data",
		})
		return
	}

	c.JSON(http.StatusOK, structs.Response{
		Success: true,
		Message: "Homepage data loaded",
		Data:    homepage,
	})
}

func (h *Controller) CreateNewsletterSubscription(c *gin.Context) {
	var req structs.NewsletterSubscriberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid newsletter payload"})
		return
	}

	fullName := strings.TrimSpace(req.FullName)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if fullName == "" || email == "" || !strings.Contains(email, "@") {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Name and valid email are required"})
		return
	}
	if !req.GDPRConsent {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Consent is required"})
		return
	}

	subscriber := models.NewsletterSubscriber{
		FullName:     fullName,
		Email:        email,
		GDPRConsent:  true,
		SubscribedAt: time.Now(),
	}
	if err := h.db.WithContext(c.Request.Context()).Where(models.NewsletterSubscriber{Email: email}).Assign(subscriber).FirstOrCreate(&subscriber).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save subscriber"})
		return
	}
	mailerSettings, err := services.LoadMailerSettings(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: "Subscription saved, but email provider settings could not be loaded"})
		return
	}
	if strings.EqualFold(mailerSettings.Provider, "gohighlevel") {
		if _, err := services.SyncGHLContact(c.Request.Context(), mailerSettings, fullName, email, []string{mailerSettings.NewsletterTag}); err != nil {
			c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: "Subscription saved, but GoHighLevel contact sync failed"})
			return
		}
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Newsletter subscription saved", Data: subscriber})
}

func (h *Controller) CreateContactLead(c *gin.Context) {
	var req structs.ContactLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid contact payload"})
		return
	}

	lead := models.ContactLead{
		Name:        strings.TrimSpace(req.Name),
		Email:       strings.ToLower(strings.TrimSpace(req.Email)),
		Subject:     strings.TrimSpace(req.Subject),
		Message:     strings.TrimSpace(req.Message),
		GDPRConsent: req.GDPRConsent,
		Status:      "new",
	}
	if lead.Name == "" || lead.Email == "" || !strings.Contains(lead.Email, "@") || lead.Subject == "" || lead.Message == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Name, email, subject, and message are required"})
		return
	}
	if !lead.GDPRConsent {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Consent is required"})
		return
	}

	if err := h.db.WithContext(c.Request.Context()).Create(&lead).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save contact message"})
		return
	}
	mailerSettings, err := services.LoadMailerSettings(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: "Contact saved, but email provider settings could not be loaded"})
		return
	}
	if strings.EqualFold(mailerSettings.Provider, "gohighlevel") {
		if _, err := services.SyncGHLContact(c.Request.Context(), mailerSettings, lead.Name, lead.Email, []string{mailerSettings.ContactLeadTag}); err != nil {
			c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: "Contact saved, but GoHighLevel contact sync failed"})
			return
		}
	}
	_ = h.notifyAdminsAboutContactLead(c.Request.Context(), lead)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Contact message saved", Data: lead})
}

func (h *Controller) ListPublicFAQs(c *gin.Context) {
	var faqs []models.FAQ
	if err := h.db.WithContext(c.Request.Context()).Order("`order` asc, id asc").Limit(8).Find(&faqs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load FAQs"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "FAQs loaded", Data: faqs})
}

func (h *Controller) ListPublicBanners(c *gin.Context) {
	var banners []models.Banner
	if err := h.db.WithContext(c.Request.Context()).Where("is_active = ?", true).Order("`order` asc, id asc").Limit(3).Find(&banners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load banners"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Banners loaded", Data: banners})
}

func (h *Controller) GetPage(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid page slug"})
		return
	}

	var page models.Page
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ?", slug).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Page not found"})
		return
	}

	var sections []models.PageSection
	h.db.WithContext(c.Request.Context()).Where("page_slug = ? AND is_active = ?", slug, true).Order("`order` asc, id asc").Find(&sections)

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Page loaded", Data: gin.H{"page": page, "sections": sections}})
}

func (h *Controller) ListPageSections(c *gin.Context) {
	pageSlug := strings.TrimSpace(c.Param("slug"))
	if pageSlug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Page slug is required"})
		return
	}
	var sections []models.PageSection
	if err := h.db.WithContext(c.Request.Context()).Where("page_slug = ? AND is_active = ?", pageSlug, true).Order("`order` asc, id asc").Find(&sections).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load page sections"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Page sections loaded", Data: sections})
}

func (h *Controller) GetSEO(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid SEO slug"})
		return
	}

	var seo models.SEOMeta
	if err := h.db.WithContext(c.Request.Context()).Where("page_slug = ?", slug).First(&seo).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "SEO meta not found"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "SEO meta loaded", Data: seo})
}

func (h *Controller) notifyAdminsAboutContactLead(ctx context.Context, lead models.ContactLead) error {
	var admins []models.User
	if err := h.db.WithContext(ctx).Where("is_admin = ?", true).Find(&admins).Error; err != nil {
		return err
	}
	if len(admins) == 0 {
		return nil
	}
	notifications := make([]models.Notification, 0, len(admins))
	for _, admin := range admins {
		notifications = append(notifications, models.Notification{
			UserID:    admin.ID,
			TitleEn:   "New contact lead",
			TitleID:   "Lead kontak baru",
			MessageEn: lead.Name + " sent a message about " + lead.Subject + ".",
			MessageID: lead.Name + " mengirim pesan tentang " + lead.Subject + ".",
			Type:      "contact_lead",
			Link:      fmt.Sprintf("/admin/crm/leads?lead=%d", lead.ID),
		})
	}
	return h.db.WithContext(ctx).Create(&notifications).Error
}
