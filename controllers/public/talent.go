package public

import (
	"net/http"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) GetTalentLanding(c *gin.Context) {
	var testimonials []models.Testimonial
	for _, source := range []string{"google", "student"} {
		var sourceReviews []models.Testimonial
		_ = h.db.WithContext(c.Request.Context()).
			Where("is_active = ? AND review_status = ? AND display_context IN ? AND review_source = ?", true, "approved", []string{"talent_bridge", "all"}, source).
			Order("created_at desc").
			Limit(2).
			Find(&sourceReviews).Error
		testimonials = append(testimonials, sourceReviews...)
	}
	if remaining := 4 - len(testimonials); remaining > 0 {
		selectedIDs := make([]uint, 0, len(testimonials))
		for _, testimonial := range testimonials {
			selectedIDs = append(selectedIDs, testimonial.ID)
		}
		query := h.db.WithContext(c.Request.Context()).
			Where("is_active = ? AND review_status = ? AND display_context IN ?", true, "approved", []string{"talent_bridge", "all"})
		if len(selectedIDs) > 0 {
			query = query.Where("id NOT IN ?", selectedIDs)
		}
		var fallbackReviews []models.Testimonial
		_ = query.Order("created_at desc").Limit(remaining).Find(&fallbackReviews).Error
		testimonials = append(testimonials, fallbackReviews...)
	}
	var reviewStats struct {
		Average float64 `json:"average"`
		Count   int64   `json:"count"`
	}
	_ = h.db.WithContext(c.Request.Context()).
		Model(&models.Testimonial{}).
		Select("COALESCE(AVG(rating), 0) AS average, COUNT(*) AS count").
		Where("is_active = ? AND review_status = ? AND display_context IN ?", true, "approved", []string{"talent_bridge", "all"}).
		Scan(&reviewStats).Error
	var googleCount int64
	var studentCount int64
	_ = h.db.WithContext(c.Request.Context()).Model(&models.Testimonial{}).Where("is_active = ? AND review_status = ? AND display_context IN ? AND review_source = ?", true, "approved", []string{"talent_bridge", "all"}, "google").Count(&googleCount).Error
	_ = h.db.WithContext(c.Request.Context()).Model(&models.Testimonial{}).Where("is_active = ? AND review_status = ? AND display_context IN ? AND review_source = ?", true, "approved", []string{"talent_bridge", "all"}, "student").Count(&studentCount).Error
	var jobs []models.TalentJob
	_ = h.db.WithContext(c.Request.Context()).Where("status = ?", "open").Order("created_at desc").Limit(3).Find(&jobs).Error
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge data loaded", Data: gin.H{
		"testimonials": testimonials,
		"review_summary": gin.H{
			"average": reviewStats.Average,
			"count":   reviewStats.Count,
			"sources": gin.H{
				"google":  googleCount,
				"student": studentCount,
			},
		},
		"jobs": jobs,
	}})
}

func (h *Controller) CreateHiringInquiry(c *gin.Context) {
	var req models.HiringInquiry
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid hiring inquiry"})
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.WorkEmail = strings.ToLower(strings.TrimSpace(req.WorkEmail))
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.RolesNeeded = strings.TrimSpace(req.RolesNeeded)
	if req.Headcount <= 0 {
		req.Headcount = 1
	}
	if req.FirstName == "" || req.LastName == "" || req.WorkEmail == "" || !strings.Contains(req.WorkEmail, "@") || req.CompanyName == "" || req.RolesNeeded == "" || !req.GDPRConsent {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Required fields and GDPR consent are needed"})
		return
	}
	req.Status = "new"
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit hiring inquiry"})
		return
	}
	h.sendTalentConfirmation(c, services.EmailTemplateHiringInquiry, "hiring_inquiry_received", req.FirstName+" "+req.LastName, req.WorkEmail, map[string]string{
		"company_name": req.CompanyName,
		"roles_needed": req.RolesNeeded,
	})
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Hiring inquiry submitted", Data: req})
}

func (h *Controller) CreateTalentPlusApplication(c *gin.Context) {
	var req models.TalentPlusApplication
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid Talent Bridge+ application"})
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)
	req.Country = strings.TrimSpace(req.Country)
	req.CurrentStatus = strings.TrimSpace(req.CurrentStatus)
	req.JobField = strings.TrimSpace(req.JobField)
	req.ProgrammeInterest = strings.TrimSpace(req.ProgrammeInterest)
	req.TargetCountries = strings.TrimSpace(req.TargetCountries)
	if req.FirstName == "" || req.LastName == "" || req.Email == "" || !strings.Contains(req.Email, "@") || req.Phone == "" || req.Country == "" || req.CurrentStatus == "" || req.JobField == "" || req.ProgrammeInterest == "" || req.TargetCountries == "" || !req.GDPRConsent {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Required fields and GDPR consent are needed"})
		return
	}
	if !validPhone(req.Phone, true) {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid phone number"})
		return
	}
	req.Status = "new"
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit Talent Bridge+ application"})
		return
	}
	h.sendTalentConfirmation(c, services.EmailTemplateTalentApplication, "talent_application_received", req.FirstName+" "+req.LastName, req.Email, map[string]string{
		"application_type": "Talent Bridge+",
	})
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge+ application submitted", Data: req})
}

func (h *Controller) ListPartners(c *gin.Context) {
	var partners []models.Partner
	query := h.db.WithContext(c.Request.Context()).Where("is_active = ? AND status = ?", true, "active")
	if partnerType := strings.TrimSpace(c.Query("type")); partnerType != "" {
		query = query.Where("partner_type = ?", partnerType)
	}
	if err := query.Order("partner_type desc, created_at desc").Find(&partners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load partners"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Partners loaded", Data: partners})
}

func (h *Controller) CreatePartnerApplication(c *gin.Context) {
	var req models.PartnerApplication
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid partner application"})
		return
	}
	req.UniversityName = strings.TrimSpace(req.UniversityName)
	req.Country = strings.TrimSpace(req.Country)
	req.ContactPerson = strings.TrimSpace(req.ContactPerson)
	req.RolePosition = strings.TrimSpace(req.RolePosition)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)
	req.CurrentQSRanking = strings.TrimSpace(req.CurrentQSRanking)
	req.PartnershipGoals = strings.TrimSpace(req.PartnershipGoals)
	if req.UniversityName == "" || req.Country == "" || req.ContactPerson == "" || req.RolePosition == "" || req.Email == "" || !strings.Contains(req.Email, "@") || req.PartnershipGoals == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Required partner application fields are missing"})
		return
	}
	if !validPhone(req.Phone, false) {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid phone number"})
		return
	}
	req.Status = "new"
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit partner application"})
		return
	}
	h.sendTalentConfirmation(c, services.EmailTemplatePartnerApplication, "partner_application_received", req.ContactPerson, req.Email, map[string]string{
		"university_name": req.UniversityName,
	})
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Partner application submitted", Data: req})
}

func (h *Controller) ListEvents(c *gin.Context) {
	var events []models.Event
	if err := h.db.WithContext(c.Request.Context()).Where("is_active = ?", true).Order("start_date asc").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load events"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Events loaded", Data: events})
}

func (h *Controller) ListTalentJobs(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	perPage := positiveInt(c.Query("per_page"), 9)
	if perPage > 24 {
		perPage = 24
	}
	query := h.db.WithContext(c.Request.Context()).Model(&models.TalentJob{}).Where("status = ?", "open")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count jobs"})
		return
	}
	var jobs []models.TalentJob
	if err := query.Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load jobs"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Jobs loaded", Data: gin.H{"items": jobs, "page": page, "per_page": perPage, "total": total, "total_pages": totalPages(total, perPage)}})
}

func (h *Controller) GetTalentJob(c *gin.Context) {
	var job models.TalentJob
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status = ?", c.Param("slug"), "open").First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Job not found"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Job loaded", Data: job})
}

func (h *Controller) CreateTalentJobApplication(c *gin.Context) {
	var job models.TalentJob
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status = ?", c.Param("slug"), "open").First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Job not found"})
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	file, err := c.FormFile("cv")
	if name == "" || email == "" || !strings.Contains(email, "@") || err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Name, email, and CV are required"})
		return
	}
	path, _, err := h.uploadService.SaveTalentCV(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	application := models.TalentJobApplication{JobID: job.ID, Name: name, Email: email, CVPath: path, Status: "new", AppliedAt: time.Now()}
	if err := h.db.WithContext(c.Request.Context()).Create(&application).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit application"})
		return
	}
	h.sendTalentConfirmation(c, services.EmailTemplateTalentApplication, "talent_application_received", application.Name, application.Email, map[string]string{
		"application_type": job.TitleEn,
	})
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Application submitted", Data: application})
}

func (h *Controller) sendTalentConfirmation(c *gin.Context, settingKey string, fallbackKey string, name string, email string, variables map[string]string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}
	locale := "en"
	var user models.User
	if err := h.db.WithContext(c.Request.Context()).Where("LOWER(email) = ?", email).First(&user).Error; err == nil {
		if user.Language == "id" {
			locale = "id"
		}
		if strings.TrimSpace(name) == "" {
			name = user.Name
		}
	}
	if strings.TrimSpace(name) == "" {
		name = email
	}
	baseURL := strings.TrimRight(h.cfg.FrontendURL, "/")
	values := map[string]string{
		"dashboard_url": baseURL + "/" + locale + "/dashboard",
		"login_url":     baseURL + "/" + locale + "/login",
		"register_url":  baseURL + "/" + locale + "/register",
	}
	for key, value := range variables {
		values[key] = value
	}
	_ = services.SendTransactionalTemplateEmail(c.Request.Context(), h.db, settingKey, fallbackKey, models.User{Name: strings.TrimSpace(name), Email: email, Language: locale}, values)
}

func validPhone(value string, required bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return !required
	}
	digits := 0
	for index, char := range value {
		if char >= '0' && char <= '9' {
			digits++
			continue
		}
		if char == '+' && index == 0 {
			continue
		}
		if char == ' ' || char == '-' || char == '.' || char == '(' || char == ')' {
			continue
		}
		return false
	}
	return digits >= 6 && digits <= 20 && len(value) <= 32
}
