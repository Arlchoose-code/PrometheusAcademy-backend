package admin

import (
	"net/http"
	"strconv"
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
	db            *gorm.DB
	cfg           config.Config
	uploadService *services.UploadService
}

func NewController(db *gorm.DB, cfg config.Config, uploadService *services.UploadService) *Controller {
	return &Controller{db: db, cfg: cfg, uploadService: uploadService}
}

func (h *Controller) GetOverview(c *gin.Context) {
	ctx := c.Request.Context()
	var totalStudents int64
	var revenue int64
	var activeCourses int64
	var contactLeads int64
	var hiringLeads int64
	var partnerLeads int64
	if err := h.db.WithContext(ctx).Model(&models.User{}).Where("is_student = ?", true).Count(&totalStudents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count students"})
		return
	}
	if err := h.db.WithContext(ctx).Model(&models.Order{}).Where("status = ?", "success").Select("COALESCE(SUM(total_amount), 0)").Scan(&revenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count revenue"})
		return
	}
	if err := h.db.WithContext(ctx).Model(&models.Course{}).Where("status = ?", "published").Count(&activeCourses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count courses"})
		return
	}
	if err := h.db.WithContext(ctx).Model(&models.ContactLead{}).Where("status = ?", "new").Count(&contactLeads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count contact leads"})
		return
	}
	if err := h.db.WithContext(ctx).Model(&models.HiringInquiry{}).Where("status = ?", "new").Count(&hiringLeads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count hiring leads"})
		return
	}
	if err := h.db.WithContext(ctx).Model(&models.PartnerApplication{}).Where("status = ?", "new").Count(&partnerLeads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count partner leads"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{
		Success: true,
		Message: "Admin overview loaded",
		Data: gin.H{
			"total_students": totalStudents,
			"revenue":        revenue,
			"active_courses": activeCourses,
			"new_leads":      contactLeads + hiringLeads + partnerLeads,
			"trends": gin.H{
				"total_students": 12,
				"revenue":        18,
				"active_courses": 4,
				"new_leads":      -6,
			},
		},
	})
}

func (h *Controller) ListContactLeads(c *gin.Context) {
	listRows[models.ContactLead](h.db, "created_at desc", "Contact leads loaded")(c)
}

func (h *Controller) UpdateContactLead(c *gin.Context) {
	updateLeadStatus[models.ContactLead](h.db, "Contact lead saved")(c)
}

func (h *Controller) ListNewsletterSubscribers(c *gin.Context) {
	listRows[models.NewsletterSubscriber](h.db, "subscribed_at desc", "Newsletter subscribers loaded")(c)
}

func (h *Controller) ListUsers(c *gin.Context) {
	type row struct {
		ID              uint       `json:"id"`
		Name            string     `json:"name"`
		Email           string     `json:"email"`
		Avatar          string     `json:"avatar"`
		Phone           string     `json:"phone"`
		IsStudent       bool       `json:"is_student"`
		IsAdmin         bool       `json:"is_admin"`
		Language        string     `json:"language"`
		CreatedAt       time.Time  `json:"created_at"`
		LastActiveAt    *time.Time `json:"last_active_at"`
		EnrolledCourses int        `json:"enrolled_courses"`
		Transactions    int        `json:"transactions"`
		TotalSpent      int        `json:"total_spent"`
	}
	var rows []row
	if err := h.db.WithContext(c.Request.Context()).Raw(`
		SELECT u.id, u.name, u.email, u.avatar, u.phone, u.is_student, u.is_admin, u.language, u.created_at,
			MAX(COALESCE(o.created_at, e.created_at, u.updated_at)) AS last_active_at,
			COUNT(DISTINCT e.id) AS enrolled_courses,
			COUNT(DISTINCT o.id) AS transactions,
			COALESCE(SUM(CASE WHEN o.status = 'success' THEN o.total_amount ELSE 0 END), 0) AS total_spent
		FROM users u
		LEFT JOIN course_enrollments e ON e.user_id = u.id
		LEFT JOIN orders o ON o.user_id = u.id
		GROUP BY u.id, u.name, u.email, u.avatar, u.phone, u.is_student, u.is_admin, u.language, u.created_at
		ORDER BY u.created_at DESC
	`).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load users"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Users loaded", Data: rows})
}

func (h *Controller) ListNotifications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var rows []models.Notification
	if err := h.db.WithContext(c.Request.Context()).Where("user_id = ?", user.ID).Order("created_at desc").Limit(20).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load notifications"})
		return
	}
	var unread int64
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", user.ID, false).Count(&unread).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications loaded", Data: gin.H{"items": rows, "unread_count": unread}})
}

func (h *Controller) MarkAllNotificationsRead(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Notification{}).Where("user_id = ?", user.ID).Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications marked as read"})
}

func (h *Controller) ResetUserPassword(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || userID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid user id"})
		return
	}

	var req structs.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid password"})
		return
	}

	hash, err := services.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to hash password"})
		return
	}

	if err := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("id = ?", uint(userID)).Update("password", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Password reset"})
}

func (h *Controller) UpdateUserRole(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || userID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid user id"})
		return
	}

	var req structs.UserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid role"})
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != "admin" && role != "student" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Role must be admin or student"})
		return
	}

	current, _ := c.Get("user")
	currentUser, _ := current.(models.User)
	if role == "student" && currentUser.ID == uint(userID) {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "You cannot remove your own admin role"})
		return
	}

	if role == "student" {
		var adminCount int64
		if err := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("is_admin = ?", true).Count(&adminCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check admins"})
			return
		}
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "At least one admin is required"})
			return
		}
	}

	updates := map[string]any{"is_student": true}
	if role == "admin" {
		updates["is_admin"] = true
	} else {
		updates["is_admin"] = false
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("id = ?", uint(userID)).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update role"})
		return
	}

	var user models.User
	if err := h.db.WithContext(c.Request.Context()).First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reload user"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Role updated", Data: user})
}

func (h *Controller) UpdateSiteLogo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Logo file is required"})
		return
	}

	path, err := h.uploadService.SaveSiteLogo(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}

	setting := models.Setting{Key: "logo_path", Value: path}
	if err := h.db.WithContext(c.Request.Context()).Where(models.Setting{Key: setting.Key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update logo setting"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Logo uploaded", Data: gin.H{"logo": path}})
}

func (h *Controller) ListSettings(c *gin.Context) {
	var rows []models.Setting
	if err := h.db.WithContext(c.Request.Context()).Order("`key` asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load settings"})
		return
	}
	settings := map[string]string{}
	for _, row := range rows {
		settings[row.Key] = row.Value
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Settings loaded", Data: settings})
}

func (h *Controller) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid settings payload"})
		return
	}
	for key, value := range req {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if key == "phone" && !services.ValidPhone(value, false) {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid phone number"})
			return
		}
		setting := models.Setting{Key: key, Value: value}
		if err := h.db.WithContext(c.Request.Context()).Where(models.Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save settings"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Settings saved"})
}

func (h *Controller) UpdateFavicon(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Favicon file is required"})
		return
	}
	path, err := h.uploadService.SaveFavicon(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	setting := models.Setting{Key: "favicon_path", Value: path}
	if err := h.db.WithContext(c.Request.Context()).Where(models.Setting{Key: setting.Key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update favicon setting"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Favicon uploaded", Data: gin.H{"favicon": path}})
}

func (h *Controller) ListSEO(c *gin.Context) {
	for _, item := range defaultSEOMetaRows() {
		seo := item
		if err := h.db.WithContext(c.Request.Context()).Where(models.SEOMeta{PageSlug: seo.PageSlug}).Attrs(seo).FirstOrCreate(&seo).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to prepare SEO meta"})
			return
		}
	}
	var rows []models.SEOMeta
	if err := h.db.WithContext(c.Request.Context()).Order("page_slug asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load SEO meta"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "SEO meta loaded", Data: rows})
}

func (h *Controller) UpdateSEO(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	var req models.SEOMeta
	if err := c.ShouldBindJSON(&req); err != nil || slug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid SEO payload"})
		return
	}
	seo := models.SEOMeta{
		PageSlug:      slug,
		TitleEn:       req.TitleEn,
		TitleID:       req.TitleID,
		DescriptionEn: req.DescriptionEn,
		DescriptionID: req.DescriptionID,
		OGImage:       req.OGImage,
	}
	if err := h.db.WithContext(c.Request.Context()).Where(models.SEOMeta{PageSlug: slug}).Assign(seo).FirstOrCreate(&seo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save SEO meta"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "SEO meta saved", Data: seo})
}

func (h *Controller) UpdateSEOImage(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	file, err := c.FormFile("file")
	if err != nil || slug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "OG image file is required"})
		return
	}
	path, err := h.uploadService.SaveSEOImage(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.SEOMeta{}).Where("page_slug = ?", slug).Update("og_image", path).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update OG image"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "OG image uploaded", Data: gin.H{"og_image": path}})
}

func (h *Controller) ListEmailTemplates(c *gin.Context) {
	var rows []models.EmailTemplate
	if err := h.db.WithContext(c.Request.Context()).Order("`key` asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load email templates"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email templates loaded", Data: rows})
}

func (h *Controller) UpdateEmailTemplate(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	var req models.EmailTemplate
	if err := c.ShouldBindJSON(&req); err != nil || key == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email template payload"})
		return
	}
	template := models.EmailTemplate{
		Key:             key,
		SubjectEn:       req.SubjectEn,
		SubjectID:       req.SubjectID,
		PreheaderEn:     req.PreheaderEn,
		PreheaderID:     req.PreheaderID,
		BodyEn:          req.BodyEn,
		BodyID:          req.BodyID,
		FooterEn:        req.FooterEn,
		FooterID:        req.FooterID,
		BackgroundColor: req.BackgroundColor,
		AccentColor:     req.AccentColor,
	}
	if err := h.db.WithContext(c.Request.Context()).Where(models.EmailTemplate{Key: key}).Assign(template).FirstOrCreate(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save email template"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email template saved", Data: template})
}

func defaultSEOMetaRows() []models.SEOMeta {
	return []models.SEOMeta{
		{PageSlug: "about", TitleEn: "About", TitleID: "Tentang", DescriptionEn: "Learn about Prometheus Academy, our mission, and the Europe x Asia education bridge.", DescriptionID: "Kenali Prometheus Academy, misi kami, dan jembatan edukasi Eropa x Asia."},
		{PageSlug: "become-a-partner", TitleEn: "Become a Partner", TitleID: "Jadi Mitra", DescriptionEn: "Partner with Prometheus Academy for university programs and global academic collaboration.", DescriptionID: "Bermitra dengan Prometheus Academy untuk program universitas dan kolaborasi akademik global."},
		{PageSlug: "contact", TitleEn: "Contact", TitleID: "Kontak", DescriptionEn: "Contact Prometheus Academy for courses, services, Talent Bridge, and partnership inquiries.", DescriptionID: "Hubungi Prometheus Academy untuk kursus, layanan, Talent Bridge, dan kemitraan."},
		{PageSlug: "courses", TitleEn: "Courses", TitleID: "Kursus", DescriptionEn: "Browse Prometheus Academy online courses in UI/UX, digital marketing, financial literacy, AI, and career preparation.", DescriptionID: "Jelajahi kursus online Prometheus Academy di UI/UX, digital marketing, literasi finansial, AI, dan persiapan karier."},
		{PageSlug: "home", TitleEn: "Prometheus Academy - Europe Asia Learning Bridge", TitleID: "Prometheus Academy - Jembatan Belajar Eropa Asia", DescriptionEn: "Courses, digital products, Talent Bridge, and university partnerships across Europe and Asia.", DescriptionID: "Kursus, produk digital, Talent Bridge, dan partner universitas di Eropa dan Asia.", OGImage: "/uploads/seo/home-og.webp"},
		{PageSlug: "privacy-policy", TitleEn: "Privacy Policy", TitleID: "Kebijakan Privasi", DescriptionEn: "How Prometheus Academy handles personal data.", DescriptionID: "Cara Prometheus Academy mengelola data pribadi."},
		{PageSlug: "services", TitleEn: "Services", TitleID: "Layanan", DescriptionEn: "Explore Prometheus Academy digital products, scholarship blueprints, e-books, and consultation services.", DescriptionID: "Jelajahi produk digital, blueprint beasiswa, e-book, dan layanan konsultasi Prometheus Academy."},
		{PageSlug: "talent-bridge", TitleEn: "Talent Bridge", TitleID: "Jembatan Talenta", DescriptionEn: "Managed staffing services connecting Asia-based talent with European companies.", DescriptionID: "Layanan staffing terkelola yang menghubungkan talenta Asia dengan perusahaan Eropa."},
		{PageSlug: "terms", TitleEn: "Terms of Service", TitleID: "Syarat Layanan", DescriptionEn: "Terms for using Prometheus Academy services, courses, and digital products.", DescriptionID: "Syarat penggunaan layanan, kursus, dan produk digital Prometheus Academy."},
	}
}
