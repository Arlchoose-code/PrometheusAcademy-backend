package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	db                   *gorm.DB
	cfg                  config.Config
	uploadService        *services.UploadService
	communicationService *services.CommunicationService
}

func NewController(db *gorm.DB, cfg config.Config, uploadService *services.UploadService) *Controller {
	return &Controller{db: db, cfg: cfg, uploadService: uploadService, communicationService: services.NewCommunicationService(db)}
}

func (h *Controller) GetOverview(c *gin.Context) {
	ctx := c.Request.Context()
	funnelPeriod := crmFunnelPeriod(c.Query("funnel_period"), time.Now().UTC())
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
			"crm_funnel":     h.crmFunnel(ctx, funnelPeriod),
			"trends": gin.H{
				"total_students": 12,
				"revenue":        18,
				"active_courses": 4,
				"new_leads":      -6,
			},
		},
	})
}

type funnelPeriod struct {
	Key   string
	Label string
	From  *time.Time
}

func crmFunnelPeriod(value string, now time.Time) funnelPeriod {
	switch value {
	case "month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return funnelPeriod{Key: "month", Label: "This month", From: &start}
	case "30d":
		start := now.AddDate(0, 0, -30)
		return funnelPeriod{Key: "30d", Label: "Last 30 days", From: &start}
	case "90d":
		start := now.AddDate(0, 0, -90)
		return funnelPeriod{Key: "90d", Label: "Last 90 days", From: &start}
	default:
		return funnelPeriod{Key: "all", Label: "All time"}
	}
}

func (h *Controller) crmFunnel(ctx context.Context, period funnelPeriod) gin.H {
	registeredQuery := h.db.Model(&models.User{}).Where("is_student = ?", true)
	bookedCallsQuery := h.db.Model(&models.ConsultationBooking{}).Distinct("user_id")
	jobApplicationsQuery := h.db.Model(&models.TalentJobApplication{})
	plusApplicationsQuery := h.db.Model(&models.TalentPlusApplication{})
	partnerApplicationsQuery := h.db.Model(&models.PartnerApplication{})
	enrolledQuery := h.db.Model(&models.CourseEnrollment{}).Distinct("user_id")
	activeStudentsQuery := h.db.Model(&models.TopicProgress{}).Distinct("user_id")
	if period.From != nil {
		registeredQuery = registeredQuery.Where("created_at >= ?", *period.From)
		bookedCallsQuery = bookedCallsQuery.Where("created_at >= ?", *period.From)
		jobApplicationsQuery = jobApplicationsQuery.Where("created_at >= ?", *period.From)
		plusApplicationsQuery = plusApplicationsQuery.Where("created_at >= ?", *period.From)
		partnerApplicationsQuery = partnerApplicationsQuery.Where("created_at >= ?", *period.From)
		enrolledQuery = enrolledQuery.Where("enrolled_at >= ?", *period.From)
		activeStudentsQuery = activeStudentsQuery.Where("completed_at >= ?", *period.From)
	}

	registeredIDs := userIDSet(ctx, registeredQuery.Select("id"), "id")
	bookedIDs := userIDSet(ctx, bookedCallsQuery, "user_id")
	applicationIDs := h.crmApplicationUserIDs(ctx, period)
	enrolledIDs := userIDSet(ctx, enrolledQuery, "user_id")
	activeIDs := userIDSet(ctx, activeStudentsQuery, "user_id")

	registered := int64(len(registeredIDs))
	bookedCohort := intersectUserSets(registeredIDs, bookedIDs)
	applicationCohort := intersectUserSets(bookedCohort, applicationIDs)
	enrolledCohort := intersectUserSets(applicationCohort, enrolledIDs)
	activeCohort := intersectUserSets(enrolledCohort, activeIDs)
	jobApplications := countRows(ctx, jobApplicationsQuery)
	plusApplications := countRows(ctx, plusApplicationsQuery)
	partnerApplications := countRows(ctx, partnerApplicationsQuery)
	stages := []gin.H{
		funnelStage("Registered", registered, registered),
		funnelStage("Booked a Call", int64(len(bookedCohort)), registered),
		funnelStage("Application Submitted", int64(len(applicationCohort)), int64(len(bookedCohort))),
		funnelStage("Enrolled", int64(len(enrolledCohort)), int64(len(applicationCohort))),
		funnelStage("Active Student", int64(len(activeCohort)), int64(len(enrolledCohort))),
	}
	return gin.H{
		"period":       period.Key,
		"period_label": period.Label,
		"from":         period.From,
		"stages":       stages,
		"method":       "strict_user_cohort",
		"application_segments": []gin.H{
			{"key": "talent_jobs", "label": "Job Applications", "count": jobApplications},
			{"key": "talent_plus", "label": "Talent Bridge+", "count": plusApplications},
			{"key": "partners", "label": "Partner Applications", "count": partnerApplications},
		},
	}
}

func (h *Controller) crmApplicationUserIDs(ctx context.Context, period funnelPeriod) map[uint]struct{} {
	parts := []string{
		"SELECT LOWER(email) AS email FROM talent_job_applications",
		"SELECT LOWER(email) AS email FROM talent_plus_applications",
		"SELECT LOWER(email) AS email FROM partner_applications",
	}
	args := []any{}
	if period.From != nil {
		for index, part := range parts {
			parts[index] = part + " WHERE created_at >= ?"
			args = append(args, *period.From)
		}
	}
	args = append(args, true)
	query := fmt.Sprintf(`
		SELECT DISTINCT users.id
		FROM users
		JOIN (%s) applications ON LOWER(users.email) = applications.email
		WHERE users.is_student = ?
	`, strings.Join(parts, " UNION ALL "))
	var ids []uint
	_ = h.db.WithContext(ctx).Raw(query, args...).Scan(&ids).Error
	return uintSet(ids)
}

func userIDSet(ctx context.Context, query *gorm.DB, column string) map[uint]struct{} {
	var ids []uint
	_ = query.WithContext(ctx).Pluck(column, &ids).Error
	return uintSet(ids)
}

func uintSet(ids []uint) map[uint]struct{} {
	result := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id != 0 {
			result[id] = struct{}{}
		}
	}
	return result
}

func intersectUserSets(base map[uint]struct{}, next map[uint]struct{}) map[uint]struct{} {
	result := make(map[uint]struct{})
	for id := range base {
		if _, ok := next[id]; ok {
			result[id] = struct{}{}
		}
	}
	return result
}

func countRows(ctx context.Context, query *gorm.DB) int64 {
	var total int64
	_ = query.WithContext(ctx).Count(&total).Error
	return total
}

func funnelStage(label string, count int64, previous int64) gin.H {
	conversion := 0.0
	if previous > 0 {
		conversion = float64(count) / float64(previous) * 100
	}
	return gin.H{"label": label, "count": count, "conversion": conversion}
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
		ID                  uint       `json:"id"`
		Name                string     `json:"name"`
		Email               string     `json:"email"`
		Avatar              string     `json:"avatar"`
		Phone               string     `json:"phone"`
		IsStudent           bool       `json:"is_student"`
		IsAdmin             bool       `json:"is_admin"`
		IsInstructor        bool       `json:"is_instructor"`
		InstructorGrantedAt *time.Time `json:"instructor_granted_at"`
		InstructorGrantedBy *uint      `json:"instructor_granted_by"`
		Language            string     `json:"language"`
		CreatedAt           time.Time  `json:"created_at"`
		LastActiveAt        *time.Time `json:"last_active_at"`
		EnrolledCourses     int        `json:"enrolled_courses"`
		Transactions        int        `json:"transactions"`
		TotalSpent          int        `json:"total_spent"`
	}
	var rows []row
	if err := h.db.WithContext(c.Request.Context()).Raw(`
		SELECT u.id, u.name, u.email, u.avatar, u.phone, u.is_student, u.is_admin, u.is_instructor,
			u.instructor_granted_at, u.instructor_granted_by, u.language, u.created_at,
			MAX(COALESCE(o.created_at, e.created_at, u.updated_at)) AS last_active_at,
			COUNT(DISTINCT e.id) AS enrolled_courses,
			COUNT(DISTINCT o.id) AS transactions,
			COALESCE(SUM(CASE WHEN o.status = 'success' THEN o.total_amount ELSE 0 END), 0) AS total_spent
		FROM users u
		LEFT JOIN course_enrollments e ON e.user_id = u.id
		LEFT JOIN orders o ON o.user_id = u.id
		GROUP BY u.id, u.name, u.email, u.avatar, u.phone, u.is_student, u.is_admin, u.is_instructor,
			u.instructor_granted_at, u.instructor_granted_by, u.language, u.created_at
		ORDER BY u.created_at DESC
	`).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load users"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Users loaded", Data: rows})
}

func (h *Controller) ListNotifications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	rows, unread, err := services.NotificationInbox(c.Request.Context(), h.db, user.ID, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications loaded", Data: gin.H{"items": rows, "unread_count": unread}})
}

func (h *Controller) MarkAllNotificationsRead(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if err := services.MarkAllNotificationsRead(c.Request.Context(), h.db, user.ID); err != nil {
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
	if role != "admin" && role != "student" && role != "instructor" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Role must be admin, instructor, or student"})
		return
	}

	current, _ := c.Get("user")
	currentUser, _ := current.(models.User)

	var user models.User
	if err := h.db.WithContext(c.Request.Context()).First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "User not found"})
		return
	}

	if role == "student" && currentUser.ID == user.ID {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "You cannot remove your own admin role"})
		return
	}

	if role == "student" && user.IsAdmin {
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

	updates := map[string]any{}
	if role == "instructor" {
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		if !enabled {
			var assignedCourses int64
			if err := h.db.WithContext(c.Request.Context()).Model(&models.Course{}).Where("instructor_id = ?", uint(userID)).Count(&assignedCourses).Error; err != nil {
				c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check assigned courses"})
				return
			}
			if assignedCourses > 0 {
				c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Reassign this instructor's courses before removing the role"})
				return
			}
			updates["is_instructor"] = false
			updates["instructor_granted_at"] = nil
			updates["instructor_granted_by"] = nil
			if !user.IsAdmin {
				updates["is_student"] = true
			}
		} else {
			now := time.Now().UTC()
			updates["is_instructor"] = true
			updates["instructor_granted_at"] = &now
			updates["instructor_granted_by"] = currentUser.ID
			if !user.IsAdmin {
				updates["is_student"] = false
			}
		}
	} else if role == "admin" {
		updates["is_admin"] = true
		updates["is_student"] = true
	} else {
		updates["is_admin"] = false
		updates["is_student"] = !user.IsInstructor
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("id = ?", uint(userID)).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update role"})
		return
	}

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
		if row.Key == "ghl_access_token" || row.Key == "brevo_api_key" {
			settings[row.Key] = ""
			settings[row.Key+"_configured"] = strconv.FormatBool(strings.TrimSpace(row.Value) != "")
			continue
		}
		settings[row.Key] = row.Value
	}
	limit, _ := strconv.Atoi(settings["monthly_enrollment_limit"])
	start := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	var enrolled int64
	if err := h.db.WithContext(c.Request.Context()).Model(&models.CourseEnrollment{}).Where("enrolled_at >= ?", start).Distinct("user_id").Count(&enrolled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count monthly enrollments"})
		return
	}
	remaining := limit - int(enrolled)
	if remaining < 0 {
		remaining = 0
	}
	settings["monthly_enrollment_count"] = strconv.FormatInt(enrolled, 10)
	settings["monthly_enrollment_remaining"] = strconv.Itoa(remaining)
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
		if (key == "ghl_access_token" || key == "brevo_api_key") && value == "" {
			continue
		}
		if key == "mailer_provider" && value != "gohighlevel" && value != "brevo" {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Mailer provider must be gohighlevel or brevo"})
			return
		}
		if key == "phone" && !services.ValidPhone(value, false) {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid phone number"})
			return
		}
		if key == "monthly_enrollment_limit" {
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 0 || limit > 1000000 {
				c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Monthly enrollment limit must be between 0 and 1,000,000"})
				return
			}
		}
		if key == "monthly_enrollment_banner_enabled" && value != "true" && value != "false" {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Enrollment banner enabled must be true or false"})
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

func (h *Controller) ListEmailDesigns(c *gin.Context) {
	var rows []models.EmailDesign
	if err := h.db.WithContext(c.Request.Context()).Order("is_default desc, created_at desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load email designs"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email designs loaded", Data: rows})
}

func (h *Controller) CreateEmailDesign(c *gin.Context) {
	var req structs.EmailDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email design payload"})
		return
	}
	design := emailDesignFromRequest(req)
	if err := h.saveEmailDesign(c, &design); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save email design"})
		return
	}
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Email design created", Data: design})
}

func (h *Controller) UpdateEmailDesign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email design id"})
		return
	}
	var req structs.EmailDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email design payload"})
		return
	}
	var design models.EmailDesign
	if err := h.db.WithContext(c.Request.Context()).First(&design, id).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Email design not found"})
		return
	}
	patch := emailDesignFromRequest(req)
	design.Name = patch.Name
	design.Description = patch.Description
	design.BackgroundColor = patch.BackgroundColor
	design.ContentColor = patch.ContentColor
	design.AccentColor = patch.AccentColor
	design.TextColor = patch.TextColor
	design.Width = patch.Width
	design.BlocksJSON = patch.BlocksJSON
	design.IsDefault = patch.IsDefault
	if err := h.saveEmailDesign(c, &design); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save email design"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email design saved", Data: design})
}

func (h *Controller) DeleteEmailDesign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email design id"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.EmailDesign{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete email design"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email design deleted"})
}

func (h *Controller) saveEmailDesign(c *gin.Context, design *models.EmailDesign) error {
	return h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if design.IsDefault {
			if err := tx.Model(&models.EmailDesign{}).Where("id <> ?", design.ID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(design).Error
	})
}

func emailDesignFromRequest(req structs.EmailDesignRequest) models.EmailDesign {
	width := req.Width
	if width < 480 {
		width = 620
	}
	if width > 760 {
		width = 760
	}
	design := models.EmailDesign{
		Name:            strings.TrimSpace(req.Name),
		Description:     strings.TrimSpace(req.Description),
		BackgroundColor: strings.TrimSpace(req.BackgroundColor),
		ContentColor:    strings.TrimSpace(req.ContentColor),
		AccentColor:     strings.TrimSpace(req.AccentColor),
		TextColor:       strings.TrimSpace(req.TextColor),
		Width:           width,
		BlocksJSON:      strings.TrimSpace(req.BlocksJSON),
		IsDefault:       req.IsDefault,
	}
	if design.BackgroundColor == "" {
		design.BackgroundColor = "#F8F9FA"
	}
	if design.ContentColor == "" {
		design.ContentColor = "#FFFFFF"
	}
	if design.AccentColor == "" {
		design.AccentColor = "#C9A84C"
	}
	if design.TextColor == "" {
		design.TextColor = "#212529"
	}
	if design.BlocksJSON == "" {
		design.BlocksJSON = `[{"id":"logo","type":"logo","content":"Prometheus Academy"},{"id":"heading","type":"heading","content":"{{subject}}"},{"id":"body","type":"body","content":"{{content}}"},{"id":"footer","type":"footer","content":"Prometheus Academy<br/>Europe x Asia learning bridge."}]`
	}
	return design
}

func (h *Controller) UpdateEmailTemplate(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	var req models.EmailTemplate
	if err := c.ShouldBindJSON(&req); err != nil || key == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email template payload"})
		return
	}
	template := models.EmailTemplate{
		DesignID:        req.DesignID,
		Key:             key,
		SubjectEn:       req.SubjectEn,
		SubjectID:       req.SubjectID,
		PreheaderEn:     req.PreheaderEn,
		PreheaderID:     req.PreheaderID,
		BodyEn:          req.BodyEn,
		BodyID:          req.BodyID,
		DesignJSON:      req.DesignJSON,
		DesignJSONEn:    req.DesignJSONEn,
		DesignJSONID:    req.DesignJSONID,
		FooterEn:        req.FooterEn,
		FooterID:        req.FooterID,
		SenderName:      req.SenderName,
		SenderEmail:     req.SenderEmail,
		BackgroundColor: req.BackgroundColor,
		AccentColor:     req.AccentColor,
	}
	if err := h.db.WithContext(c.Request.Context()).Where(models.EmailTemplate{Key: key}).Assign(template).FirstOrCreate(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save email template"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email template saved", Data: template})
}

func (h *Controller) DeleteEmailTemplate(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid email template key"})
		return
	}
	builtIn := map[string]bool{
		"campaign_simple": true, "campaign_newsletter": true, "campaign_announcement": true,
		"welcome": true, "email_verification": true, "login_notification": true, "otp_login": true,
		"password_reset": true, "invoice": true, "payment_success": true, "deposit_confirmation": true,
		"certificate": true, "booking_confirmation": true, "talent_review_invitation": true,
	}
	if builtIn[key] || strings.HasPrefix(key, "automation_") {
		c.JSON(http.StatusConflict, structs.Response{Success: false, Message: "Built-in templates cannot be deleted. You can edit them or assign a different template."})
		return
	}
	var template models.EmailTemplate
	if err := h.db.WithContext(c.Request.Context()).Where("`key` = ?", key).First(&template).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Email template not found"})
		return
	}
	var settingRefs, workflowRefs, activeCampaignRefs int64
	h.db.WithContext(c.Request.Context()).Model(&models.Setting{}).Where("value = ? AND `key` LIKE ?", key, "email_template_%").Count(&settingRefs)
	h.db.WithContext(c.Request.Context()).Model(&models.AutomationWorkflow{}).Where("template_key = ?", key).Count(&workflowRefs)
	h.db.WithContext(c.Request.Context()).Model(&models.EmailCampaign{}).Where("template_key = ? AND status IN ?", key, []string{"draft", "queued", "processing"}).Count(&activeCampaignRefs)
	if settingRefs+workflowRefs+activeCampaignRefs > 0 {
		c.JSON(http.StatusConflict, structs.Response{Success: false, Message: "This template is still assigned to a system email, automation, or active campaign. Change that assignment first."})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete email template"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email template deleted"})
}

func (h *Controller) TestMailer(c *gin.Context) {
	var req structs.MailerTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid test email payload"})
		return
	}

	settings, err := services.LoadMailerSettings(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load mailer settings"})
		return
	}

	messageID, err := services.SendMailerEmail(c.Request.Context(), settings, services.MailMessage{
		ToEmail: req.ToEmail,
		ToName:  req.ToName,
		Subject: req.Subject,
		HTML:    req.HTML,
		Text:    req.Text,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, structs.Response{
		Success: true,
		Message: "Test email sent",
		Data:    gin.H{"message_id": messageID},
	})
}

func (h *Controller) ListMailerAudienceUsers(c *gin.Context) {
	type row struct {
		ID        uint      `json:"id"`
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		IsStudent bool      `json:"is_student"`
		IsAdmin   bool      `json:"is_admin"`
		CreatedAt time.Time `json:"created_at"`
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	query := strings.TrimSpace(c.Query("search"))
	role := strings.TrimSpace(c.Query("role"))
	db := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("email <> ''")
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("name LIKE ? OR email LIKE ?", like, like)
	}
	switch role {
	case "students":
		db = db.Where("is_student = ?", true)
	case "admins":
		db = db.Where("is_admin = ?", true)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count users"})
		return
	}
	if c.Query("ids_only") == "1" {
		var ids []uint
		if err := db.Order("created_at desc").Pluck("id", &ids).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load user IDs"})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Mailer user IDs loaded", Data: gin.H{"ids": ids, "total": total}})
		return
	}
	var rows []row
	if err := db.Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load users"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Mailer users loaded", Data: gin.H{
		"items": rows,
		"pagination": gin.H{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": (total + int64(perPage) - 1) / int64(perPage),
		},
	}})
}

func (h *Controller) ListMailerCampaigns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "12"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 12
	}
	var total int64
	if err := h.db.WithContext(c.Request.Context()).Model(&models.EmailCampaign{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count campaigns"})
		return
	}
	var campaigns []models.EmailCampaign
	if err := h.db.WithContext(c.Request.Context()).Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&campaigns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load campaigns"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Email campaigns loaded", Data: gin.H{
		"items":      campaigns,
		"pagination": gin.H{"page": page, "per_page": perPage, "total": total, "total_pages": (total + int64(perPage) - 1) / int64(perPage)},
	}})
}

func (h *Controller) QueueMailerCampaign(c *gin.Context) {
	var req structs.MailerCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid campaign payload"})
		return
	}
	campaign, err := h.upsertMailerCampaign(c, req, "queued")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, structs.Response{Success: true, Message: "Campaign queued", Data: campaign})
}

func (h *Controller) upsertMailerCampaign(c *gin.Context, req structs.MailerCampaignRequest, status string) (models.EmailCampaign, error) {
	subjectEn := strings.TrimSpace(req.SubjectEn)
	subjectID := strings.TrimSpace(req.SubjectID)
	htmlEn := req.HTMLEn
	htmlID := req.HTMLID
	textEn := req.TextEn
	textID := req.TextID
	if templateKey := strings.TrimSpace(req.TemplateKey); templateKey != "" {
		var template models.EmailTemplate
		if err := h.db.WithContext(c.Request.Context()).Where("`key` = ?", templateKey).First(&template).Error; err != nil {
			return models.EmailCampaign{}, errors.New("Selected email template was not found")
		}
		subjectEn = template.SubjectEn
		subjectID = template.SubjectID
		if strings.TrimSpace(subjectID) == "" {
			subjectID = template.SubjectEn
		}
		htmlEn = template.BodyEn
		htmlID = template.BodyID
		if strings.TrimSpace(htmlID) == "" {
			htmlID = template.BodyEn
		}
		textEn = ""
		textID = ""
	}
	if subjectEn == "" {
		subjectEn = strings.TrimSpace(req.Subject)
	}
	if subjectID == "" {
		subjectID = subjectEn
	}
	if subjectEn == "" {
		subjectEn = subjectID
	}
	if strings.TrimSpace(htmlEn) == "" {
		htmlEn = req.HTML
	}
	if strings.TrimSpace(htmlID) == "" {
		htmlID = htmlEn
	}
	if strings.TrimSpace(htmlEn) == "" {
		htmlEn = htmlID
	}
	if strings.TrimSpace(textEn) == "" {
		textEn = req.Text
	}
	if strings.TrimSpace(textID) == "" {
		textID = textEn
	}
	if strings.TrimSpace(req.TemplateKey) != "" && (strings.TrimSpace(subjectEn) == "" || strings.TrimSpace(htmlEn) == "") {
		var template models.EmailTemplate
		if err := h.db.WithContext(c.Request.Context()).Where("`key` = ?", strings.TrimSpace(req.TemplateKey)).First(&template).Error; err != nil {
			return models.EmailCampaign{}, errors.New("Email template not found")
		}
		subjectEn = firstNonEmpty(subjectEn, template.SubjectEn, template.SubjectID, template.Key)
		subjectID = firstNonEmpty(subjectID, template.SubjectID, template.SubjectEn, subjectEn)
		htmlEn = firstNonEmpty(htmlEn, template.BodyEn, template.BodyID)
		htmlID = firstNonEmpty(htmlID, template.BodyID, template.BodyEn, htmlEn)
		textEn = firstNonEmpty(textEn, stripHTMLForEmailLocal(htmlEn))
		textID = firstNonEmpty(textID, stripHTMLForEmailLocal(htmlID), textEn)
	}
	if strings.TrimSpace(subjectEn) == "" && strings.TrimSpace(subjectID) == "" {
		return models.EmailCampaign{}, errors.New("Subject is required")
	}
	if strings.TrimSpace(htmlEn) == "" && strings.TrimSpace(htmlID) == "" {
		return models.EmailCampaign{}, errors.New("Email body is required")
	}
	rateLimit := req.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = 30
	}
	if rateLimit > 300 {
		rateLimit = 300
	}
	recipients, err := services.ResolveCampaignRecipients(c.Request.Context(), h.db, req.Target, req.UserIDs)
	if err != nil {
		return models.EmailCampaign{}, err
	}
	userIDsJSON, _ := json.Marshal(req.UserIDs)
	currentUser, _ := c.Get("user")
	adminUser, _ := currentUser.(models.User)
	now := time.Now()
	campaign := models.EmailCampaign{
		DesignID:           req.DesignID,
		TemplateKey:        strings.TrimSpace(req.TemplateKey),
		Name:               strings.TrimSpace(req.Name),
		Subject:            strings.TrimSpace(subjectEn),
		SubjectEn:          strings.TrimSpace(subjectEn),
		SubjectID:          strings.TrimSpace(subjectID),
		HTML:               htmlEn,
		HTMLEn:             htmlEn,
		HTMLID:             htmlID,
		Text:               textEn,
		TextEn:             textEn,
		TextID:             textID,
		Target:             strings.TrimSpace(req.Target),
		UserIDsJSON:        string(userIDsJSON),
		SenderName:         strings.TrimSpace(req.SenderName),
		SenderEmail:        strings.TrimSpace(req.SenderEmail),
		RateLimitPerMinute: rateLimit,
		Status:             status,
		RecipientCount:     len(recipients),
		CreatedBy:          adminUser.ID,
	}
	if status == "queued" {
		campaign.QueuedAt = &now
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&campaign).Error; err != nil {
		return campaign, err
	}
	return campaign, nil
}

func (h *Controller) ListMailerSenders(c *gin.Context) {
	settings, err := services.LoadMailerSettings(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load mailer settings"})
		return
	}
	activeProvider := strings.ToLower(settings.Provider)
	if activeProvider == "brevo" && strings.TrimSpace(settings.BrevoAPIKey) != "" {
		if brevoSenders, err := services.ListBrevoSenders(c.Request.Context(), settings); err == nil {
			for _, item := range brevoSenders {
				sender := models.MailerSender{
					Provider:       "brevo",
					Name:           fallbackString(item.Name, item.Email),
					Email:          item.Email,
					Status:         "synced",
					ExternalID:     item.ID,
					ExternalStatus: item.Status,
				}
				_ = h.db.WithContext(c.Request.Context()).Where("email = ?", sender.Email).Assign(sender).FirstOrCreate(&sender).Error
			}
		}
	}
	if strings.TrimSpace(settings.FromEmail) != "" {
		sender := models.MailerSender{Provider: activeProvider, Name: settings.FromName, Email: strings.ToLower(settings.FromEmail), IsDefault: true, Status: "local", ExternalStatus: "default"}
		_ = h.db.WithContext(c.Request.Context()).Where("email = ?", sender.Email).Attrs(sender).FirstOrCreate(&sender).Error
	}
	var rows []models.MailerSender
	if err := h.db.WithContext(c.Request.Context()).Where("provider IN ?", []string{"all", activeProvider}).Order("is_default desc, name asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load sender identities"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Sender identities loaded", Data: rows})
}

func (h *Controller) SaveMailerSender(c *gin.Context) {
	var req structs.MailerSenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid sender payload"})
		return
	}
	settings, _ := services.LoadMailerSettings(c.Request.Context(), h.db)
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = strings.ToLower(settings.Provider)
	}
	if provider != "brevo" && provider != "gohighlevel" && provider != "all" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Sender provider must be all, brevo, or gohighlevel"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	sender := models.MailerSender{Provider: provider, Name: strings.TrimSpace(req.Name), Email: email, IsDefault: req.IsDefault, Status: "local"}
	if provider == "brevo" && strings.EqualFold(settings.Provider, "brevo") {
		if externalID, externalStatus, err := services.CreateBrevoSender(c.Request.Context(), settings, sender.Name, sender.Email); err == nil {
			sender.ExternalID = strings.TrimSpace(externalID)
			sender.ExternalStatus = externalStatus
			sender.Status = "synced"
		}
	}
	if sender.IsDefault {
		if err := h.db.WithContext(c.Request.Context()).Model(&models.MailerSender{}).Where("provider IN ?", []string{"all", provider}).Update("is_default", false).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update default sender"})
			return
		}
		for key, value := range map[string]string{"mailer_from_name": sender.Name, "mailer_from_email": sender.Email} {
			setting := models.Setting{Key: key}
			if err := h.db.WithContext(c.Request.Context()).Where(models.Setting{Key: key}).Assign(models.Setting{Value: value}).FirstOrCreate(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save sender"})
				return
			}
		}
	}
	id, _ := strconv.Atoi(c.Param("id"))
	query := h.db.WithContext(c.Request.Context())
	if id > 0 {
		if err := query.Model(&models.MailerSender{}).Where("id = ?", uint(id)).Updates(sender).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save sender"})
			return
		}
		_ = query.First(&sender, uint(id)).Error
	} else if err := query.Where("email = ?", sender.Email).Assign(sender).FirstOrCreate(&sender).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save sender"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Sender identity saved", Data: sender})
}

func (h *Controller) DeleteMailerSender(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid sender id"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.MailerSender{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete sender"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Sender identity deleted"})
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fallbackString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func stripHTMLForEmailLocal(value string) string {
	var builder strings.Builder
	inTag := false
	for _, char := range value {
		switch char {
		case '<':
			inTag = true
		case '>':
			inTag = false
			builder.WriteRune(' ')
		default:
			if !inTag {
				builder.WriteRune(char)
			}
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}
