package public

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *Controller) GetTalentLanding(c *gin.Context) {
	var testimonials []models.Testimonial
	for _, source := range []string{"google", "student"} {
		var sourceReviews []models.Testimonial
		_ = h.db.WithContext(c.Request.Context()).
			Where("is_active = ? AND review_status = ? AND display_context = ? AND review_source = ?", true, "approved", "talent_bridge", source).
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
			Where("is_active = ? AND review_status = ? AND display_context = ?", true, "approved", "talent_bridge")
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
		Where("is_active = ? AND review_status = ? AND display_context = ?", true, "approved", "talent_bridge").
		Scan(&reviewStats).Error
	var googleCount int64
	var studentCount int64
	_ = h.db.WithContext(c.Request.Context()).Model(&models.Testimonial{}).Where("is_active = ? AND review_status = ? AND display_context = ? AND review_source = ?", true, "approved", "talent_bridge", "google").Count(&googleCount).Error
	_ = h.db.WithContext(c.Request.Context()).Model(&models.Testimonial{}).Where("is_active = ? AND review_status = ? AND display_context = ? AND review_source = ?", true, "approved", "talent_bridge", "student").Count(&studentCount).Error
	var jobs []models.TalentJob
	_ = h.db.WithContext(c.Request.Context()).Where("status = ?", "open").Order("created_at desc").Limit(3).Find(&jobs).Error
	var trustPhotos []models.TalentTrustPhoto
	_ = h.db.WithContext(c.Request.Context()).
		Where("is_active = ? AND image_path <> ?", true, "").
		Order("`order` asc, created_at desc").
		Limit(6).
		Find(&trustPhotos).Error
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
		"jobs":         jobs,
		"trust_photos": trustPhotos,
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
	if req.Language != "id" {
		req.Language = "en"
	}
	if req.Headcount <= 0 {
		req.Headcount = 1
	}
	if req.FirstName == "" || req.LastName == "" || req.WorkEmail == "" || !strings.Contains(req.WorkEmail, "@") || req.CompanyName == "" || req.RolesNeeded == "" || !req.GDPRConsent {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Required fields and GDPR consent are needed"})
		return
	}
	req.Status = "new"
	var existingUser models.User
	if err := h.db.WithContext(c.Request.Context()).Where("LOWER(email) = ?", req.WorkEmail).First(&existingUser).Error; err == nil {
		if !existingUser.IsAdmin && !existingUser.IsCompany {
			c.JSON(http.StatusConflict, structs.Response{Success: false, Message: "Use a dedicated company email or ask an admin to change this account to Company", Code: "company_account_required"})
			return
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to validate company account"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit hiring inquiry"})
		return
	}
	h.ensureCompanyPortalInvite(c.Request.Context(), req)
	h.sendTalentConfirmation(c, services.EmailTemplateHiringInquiry, "hiring_inquiry_received", req.FirstName+" "+req.LastName, req.WorkEmail, map[string]string{
		"company_name": req.CompanyName,
		"roles_needed": req.RolesNeeded,
	})
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Hiring inquiry submitted", Data: req})
}

func (h *Controller) ensureCompanyPortalInvite(ctx context.Context, inquiry models.HiringInquiry) {
	email := strings.ToLower(strings.TrimSpace(inquiry.WorkEmail))
	if email == "" {
		return
	}
	now := time.Now()
	var user models.User
	if err := h.db.WithContext(ctx).Where("LOWER(email) = ?", email).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		password, err := randomPortalPassword()
		if err != nil {
			return
		}
		hash, err := services.HashPassword(password)
		if err != nil {
			return
		}
		name := strings.TrimSpace(inquiry.FirstName + " " + inquiry.LastName)
		if name == "" {
			name = inquiry.CompanyName
		}
		user = models.User{
			Name:            name,
			Email:           email,
			Password:        hash,
			IsStudent:       false,
			IsCompany:       true,
			Language:        inquiry.Language,
			EmailVerifiedAt: &now,
		}
		if err := h.db.WithContext(ctx).Create(&user).Error; err != nil {
			return
		}
	} else if !user.IsCompany {
		if !user.IsAdmin {
			return
		}
		updates := map[string]any{"is_company": true}
		if err := h.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
			return
		}
		user.IsCompany = true
		if !user.IsAdmin {
			user.IsStudent = false
		}
	}
	locale := inquiry.Language
	if locale != "id" {
		locale = user.Language
	}
	if locale != "id" {
		locale = "en"
	}
	inviteUser := user
	inviteUser.Language = locale
	companyDashboardURL := strings.TrimRight(h.cfg.FrontendURL, "/") + "/" + locale + "/talent-bridge/company-dashboard"
	_, _ = services.CreatePasswordResetInvitation(ctx, h.db, h.cfg, inviteUser, services.EmailTemplateCompanyPortalInvite, "company_portal_invite", map[string]string{
		"company_name":          inquiry.CompanyName,
		"roles_needed":          inquiry.RolesNeeded,
		"company_dashboard_url": companyDashboardURL,
	})
}

func randomPortalPassword() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
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
	if req.Language != "id" {
		req.Language = "en"
	}
	if req.FirstName == "" || req.LastName == "" || req.Email == "" || !strings.Contains(req.Email, "@") || req.Phone == "" || req.Country == "" || req.CurrentStatus == "" || req.JobField == "" || req.ProgrammeInterest == "" || req.TargetCountries == "" || !req.GDPRConsent {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Required fields and GDPR consent are needed"})
		return
	}
	if !validPhone(req.Phone, true) {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid phone number"})
		return
	}
	req.Status = "new"
	var authenticatedUser *models.User
	if user, exists := c.Get("user"); exists {
		if authUser, ok := user.(models.User); ok && strings.EqualFold(authUser.Email, req.Email) {
			authenticatedUser = &authUser
		}
	}
	var inviteUser *models.User
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var owner models.User
		lookupErr := tx.Where("LOWER(email) = ?", req.Email).First(&owner).Error
		if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		if lookupErr == nil {
			if !owner.IsAdmin && !owner.IsStudent {
				return errors.New("talent_student_account_required")
			}
			if authenticatedUser != nil && authenticatedUser.ID == owner.ID {
				req.UserID = owner.ID
			}
		} else {
			password, passwordErr := randomPortalPassword()
			if passwordErr != nil {
				return passwordErr
			}
			hash, hashErr := services.HashPassword(password)
			if hashErr != nil {
				return hashErr
			}
			now := time.Now()
			owner = models.User{
				Name:            strings.TrimSpace(req.FirstName + " " + req.LastName),
				Email:           req.Email,
				Password:        hash,
				IsStudent:       true,
				Language:        req.Language,
				EmailVerifiedAt: &now,
			}
			if createErr := tx.Create(&owner).Error; createErr != nil {
				return createErr
			}
			req.UserID = owner.ID
			inviteCopy := owner
			inviteUser = &inviteCopy
		}
		return tx.Create(&req).Error
	})
	if err != nil {
		if err.Error() == "talent_student_account_required" {
			c.JSON(http.StatusConflict, structs.Response{Success: false, Message: "This email belongs to a different account type. Use a student email or ask an admin to change the role", Code: "talent_student_account_required"})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit Talent Bridge+ application"})
		return
	}
	if inviteUser != nil {
		dashboardURL := strings.TrimRight(h.cfg.FrontendURL, "/") + "/" + req.Language + "/dashboard"
		_, _ = services.CreatePasswordResetInvitation(c.Request.Context(), h.db, h.cfg, *inviteUser, services.EmailTemplateTalentPortalInvite, "talent_portal_invite", map[string]string{
			"application_type": "Talent Bridge+",
			"dashboard_url":    dashboardURL,
		})
	} else {
		h.sendTalentConfirmation(c, services.EmailTemplateTalentApplication, "talent_application_received", req.FirstName+" "+req.LastName, req.Email, map[string]string{
			"application_type": "Talent Bridge+",
		})
	}
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

func (h *Controller) GetCompanyTalentDashboard(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	email := strings.ToLower(strings.TrimSpace(user.Email))
	inquiries := make([]models.HiringInquiry, 0)
	if err := h.db.WithContext(c.Request.Context()).
		Where("LOWER(work_email) = ?", email).
		Order("created_at desc").
		Find(&inquiries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load company dashboard"})
		return
	}
	var hiredCount int64
	var requestedHeadcount int
	activeRequests := 0
	for _, inquiry := range inquiries {
		requestedHeadcount += inquiry.Headcount
		switch strings.ToLower(inquiry.Status) {
		case "new", "contacted", "approved", "posted", "in_progress", "qualified":
			activeRequests++
		}
	}
	if err := h.db.WithContext(c.Request.Context()).Table("talent_job_applications a").
		Joins("JOIN talent_jobs j ON j.id = a.job_id").
		Joins("JOIN hiring_inquiries h ON h.id = j.hiring_inquiry_id").
		Where("LOWER(h.work_email) = ? AND a.status = ?", email, "hired").
		Count(&hiredCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count company placements"})
		return
	}
	type candidateRow struct {
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		Status     string `json:"status"`
		JobTitleEn string `json:"job_title_en"`
		JobTitleID string `json:"job_title_id"`
		AppliedAt  string `json:"applied_at"`
	}
	candidates := make([]candidateRow, 0)
	if err := h.db.WithContext(c.Request.Context()).Table("talent_job_applications a").
		Select("a.id, a.name, a.email, a.status, j.title_en AS job_title_en, j.title_id AS job_title_id, a.applied_at").
		Joins("JOIN talent_jobs j ON j.id = a.job_id").
		Joins("JOIN hiring_inquiries h ON h.id = j.hiring_inquiry_id").
		Where("LOWER(h.work_email) = ? AND a.status IN ?", email, []string{"proposed", "interview", "hired", "declined"}).
		Order("a.applied_at DESC").Scan(&candidates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load proposed candidates"})
		return
	}
	placements := make([]candidateRow, 0)
	for _, candidate := range candidates {
		if candidate.Status == "hired" {
			placements = append(placements, candidate)
		}
	}
	companyName := companyDashboardName(inquiries)
	if companyName == "" {
		companyName = user.Name
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Company dashboard loaded", Data: gin.H{
		"company_name":        companyName,
		"hired_count":         hiredCount,
		"requested_headcount": requestedHeadcount,
		"active_requests":     activeRequests,
		"inquiries":           inquiries,
		"candidates":          candidates,
		"placements":          placements,
	}})
}

func (h *Controller) UpdateCompanyCandidateDecision(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	applicationID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || applicationID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid candidate id"})
		return
	}
	var req struct {
		Decision string `json:"decision"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid decision"})
		return
	}
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	allowed := map[string]bool{"interview": true, "hired": true, "declined": true}
	if !allowed[decision] {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Decision must be interview, hired, or declined"})
		return
	}
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var application models.TalentJobApplication
		if err := tx.Raw(`
			SELECT a.* FROM talent_job_applications a
			JOIN talent_jobs j ON j.id = a.job_id
			JOIN hiring_inquiries h ON h.id = j.hiring_inquiry_id
			WHERE a.id = ? AND LOWER(h.work_email) = ? FOR UPDATE
		`, uint(applicationID), strings.ToLower(strings.TrimSpace(user.Email))).Scan(&application).Error; err != nil {
			return err
		}
		if application.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		if application.Status != "proposed" && application.Status != "interview" && application.Status != "hired" && application.Status != "declined" {
			return fmt.Errorf("candidate has not been proposed to your company")
		}
		var job models.TalentJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, application.JobID).Error; err != nil {
			return err
		}
		if decision == "hired" && application.Status != "hired" {
			var hired int64
			if err := tx.Model(&models.TalentJobApplication{}).Where("job_id = ? AND status = ?", job.ID, "hired").Count(&hired).Error; err != nil {
				return err
			}
			if hired >= int64(job.OpenPositions) {
				return fmt.Errorf("all requested positions are already filled")
			}
		}
		if err := tx.Model(&application).Update("status", decision).Error; err != nil {
			return err
		}
		var hired int64
		if err := tx.Model(&models.TalentJobApplication{}).Where("job_id = ? AND status = ?", job.ID, "hired").Count(&hired).Error; err != nil {
			return err
		}
		jobStatus, inquiryStatus := "open", "in_progress"
		if hired >= int64(job.OpenPositions) {
			jobStatus, inquiryStatus = "closed", "resolved"
		}
		if err := tx.Model(&job).Update("status", jobStatus).Error; err != nil {
			return err
		}
		return tx.Model(&models.HiringInquiry{}).Where("id = ?", job.HiringInquiryID).Update("status", inquiryStatus).Error
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := "Failed to save candidate decision"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			statusCode = http.StatusNotFound
			message = "Candidate not found"
		} else if err.Error() == "candidate has not been proposed to your company" || err.Error() == "all requested positions are already filled" {
			statusCode = http.StatusBadRequest
			message = err.Error()
		}
		c.JSON(statusCode, structs.Response{Success: false, Message: message})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Candidate decision saved"})
}

func (h *Controller) DownloadCompanyCandidateCV(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	applicationID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || applicationID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid candidate id"})
		return
	}
	var application models.TalentJobApplication
	if err := h.db.WithContext(c.Request.Context()).Raw(`
		SELECT a.* FROM talent_job_applications a
		JOIN talent_jobs j ON j.id = a.job_id
		JOIN hiring_inquiries h ON h.id = j.hiring_inquiry_id
		WHERE a.id = ? AND LOWER(h.work_email) = ? AND a.status IN ('proposed','interview','hired','declined')
	`, uint(applicationID), strings.ToLower(strings.TrimSpace(user.Email))).Scan(&application).Error; err != nil || application.ID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Candidate not found"})
		return
	}
	reader, info, err := services.OpenStoredUploadPath(c.Request.Context(), h.db, h.cfg, application.CVPath)
	if err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "CV file not found"})
		return
	}
	defer reader.Close()
	c.Header("Content-Type", info.ContentType)
	c.Header("Content-Disposition", `attachment; filename="candidate-cv`+filepath.Ext(application.CVPath)+`"`)
	_, _ = io.Copy(c.Writer, reader)
}

func (h *Controller) GetTalentApplyEligibility(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	enrolledCount, completedCount, eligible, err := h.talentApplyEligibility(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load Talent Bridge eligibility"})
		return
	}
	alreadyApplied := false
	if slug := strings.TrimSpace(c.Query("job_slug")); slug != "" {
		var count int64
		if err := h.db.WithContext(c.Request.Context()).Table("talent_job_applications a").
			Joins("JOIN talent_jobs j ON j.id = a.job_id").
			Where("j.slug = ? AND a.user_id = ?", slug, user.ID).
			Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check existing application"})
			return
		}
		alreadyApplied = count > 0
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge eligibility loaded", Data: gin.H{
		"eligible":        eligible || user.IsAdmin,
		"enrolled_count":  enrolledCount,
		"completed_count": completedCount,
		"already_applied": alreadyApplied,
	}})
}

func (h *Controller) CreateTalentJobApplication(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if !user.IsAdmin {
		_, _, eligible, err := h.talentApplyEligibility(c.Request.Context(), user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check Talent Bridge eligibility"})
			return
		}
		if !eligible {
			c.JSON(http.StatusForbidden, structs.Response{Success: false, Code: "talent_course_required", Message: "Enroll in a Prometheus course before applying to Talent Bridge jobs"})
			return
		}
	}
	var job models.TalentJob
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status = ?", c.Param("slug"), "open").First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Job not found"})
		return
	}
	var duplicateCount int64
	if err := h.db.WithContext(c.Request.Context()).Model(&models.TalentJobApplication{}).
		Where("job_id = ? AND (user_id = ? OR LOWER(email) = ?)", job.ID, user.ID, strings.ToLower(strings.TrimSpace(user.Email))).
		Count(&duplicateCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check existing application"})
		return
	}
	if duplicateCount > 0 {
		c.JSON(http.StatusConflict, structs.Response{Success: false, Code: "talent_already_applied", Message: "You have already applied for this position"})
		return
	}
	name := strings.TrimSpace(user.Name)
	email := strings.ToLower(strings.TrimSpace(user.Email))
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
	application := models.TalentJobApplication{UserID: user.ID, JobID: job.ID, Name: name, Email: email, CVPath: path, Status: "new", AppliedAt: time.Now()}
	if err := h.db.WithContext(c.Request.Context()).Create(&application).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit application"})
		return
	}
	h.sendTalentConfirmation(c, services.EmailTemplateTalentApplication, "talent_application_received", application.Name, application.Email, map[string]string{
		"application_type": job.TitleEn,
	})
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Application submitted", Data: application})
}

func (h *Controller) talentApplyEligibility(ctx context.Context, userID uint) (int64, int64, bool, error) {
	var enrolledCount int64
	if err := h.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("user_id = ?", userID).Count(&enrolledCount).Error; err != nil {
		return 0, 0, false, err
	}
	var completedCount int64
	if err := h.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("user_id = ? AND completed_at IS NOT NULL", userID).Count(&completedCount).Error; err != nil {
		return 0, 0, false, err
	}
	return enrolledCount, completedCount, enrolledCount > 0, nil
}

func companyDashboardName(inquiries []models.HiringInquiry) string {
	for _, inquiry := range inquiries {
		if strings.TrimSpace(inquiry.CompanyName) != "" {
			return inquiry.CompanyName
		}
	}
	return ""
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
