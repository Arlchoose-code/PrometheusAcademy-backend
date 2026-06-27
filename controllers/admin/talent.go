package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
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

func (h *Controller) ListTalentJobs(c *gin.Context) {
	listRows[models.TalentJob](h.db, "created_at desc", "Talent jobs loaded")(c)
}

func (h *Controller) CreateTalentJob(c *gin.Context) {
	var job models.TalentJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid job payload"})
		return
	}
	if strings.TrimSpace(job.Slug) == "" {
		job.Slug = services.GenerateSlug(job.TitleEn)
	}
	if strings.TrimSpace(job.Status) == "" {
		job.Status = "draft"
	}
	if job.Status != "draft" && job.Status != "open" && job.Status != "closed" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Job status must be draft, open, or closed"})
		return
	}
	if job.OpenPositions <= 0 {
		job.OpenPositions = 1
	}
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if job.HiringInquiryID != 0 {
			var inquiry models.HiringInquiry
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inquiry, job.HiringInquiryID).Error; err != nil {
				return err
			}
			if inquiry.Status != "approved" {
				return fmt.Errorf("hiring inquiry must be approved before creating a job")
			}
			var existing int64
			if err := tx.Model(&models.TalentJob{}).Where("hiring_inquiry_id = ?", inquiry.ID).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				return fmt.Errorf("this hiring inquiry already has a job listing")
			}
			job.OpenPositions = inquiry.Headcount
			job.Status = "draft"
		}
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		if job.HiringInquiryID != 0 {
			return tx.Model(&models.HiringInquiry{}).Where("id = ?", job.HiringInquiryID).Update("status", "drafted").Error
		}
		return nil
	}); err != nil {
		message := "Failed to create job"
		statusCode := http.StatusInternalServerError
		if err.Error() == "hiring inquiry must be approved before creating a job" || err.Error() == "this hiring inquiry already has a job listing" {
			message = err.Error()
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, structs.Response{Success: false, Message: message})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent job created", Data: job})
}

func (h *Controller) UpdateTalentJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid job id"})
		return
	}
	var job models.TalentJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid job payload"})
		return
	}
	if strings.TrimSpace(job.Slug) == "" {
		job.Slug = services.GenerateSlug(job.TitleEn)
	}
	if job.OpenPositions <= 0 {
		job.OpenPositions = 1
	}
	if job.Status != "draft" && job.Status != "open" && job.Status != "closed" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Job status must be draft, open, or closed"})
		return
	}
	var existing models.TalentJob
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&existing, uint(id)).Error; err != nil {
			return err
		}
		if err := tx.Model(&existing).Select("title_en", "title_id", "slug", "description_en", "description_id", "open_positions", "status").Updates(job).Error; err != nil {
			return err
		}
		if existing.HiringInquiryID != 0 && job.Status != "closed" {
			inquiryStatus := "drafted"
			if job.Status == "open" {
				inquiryStatus = "posted"
			}
			return tx.Model(&models.HiringInquiry{}).Where("id = ? AND status <> ?", existing.HiringInquiryID, "resolved").Update("status", inquiryStatus).Error
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save job"})
		return
	}
	job.ID = uint(id)
	job.HiringInquiryID = existing.HiringInquiryID
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent job saved", Data: job})
}

func (h *Controller) DeleteTalentJob(c *gin.Context) {
	deleteRow[models.TalentJob](h.db, "Talent job deleted")(c)
}

func (h *Controller) ListTalentTrustPhotos(c *gin.Context) {
	listRows[models.TalentTrustPhoto](h.db, "`order` asc, created_at desc", "Talent Bridge trust photos loaded")(c)
}

func (h *Controller) CreateTalentTrustPhoto(c *gin.Context) {
	var photo models.TalentTrustPhoto
	if err := c.ShouldBindJSON(&photo); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid trust photo payload"})
		return
	}
	normalizeTalentTrustPhoto(&photo)
	if photo.TitleEn == "" || photo.ImagePath == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Title EN and image are required"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&photo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create trust photo"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge trust photo created", Data: photo})
}

func (h *Controller) UpdateTalentTrustPhoto(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid trust photo id"})
		return
	}
	var photo models.TalentTrustPhoto
	if err := c.ShouldBindJSON(&photo); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid trust photo payload"})
		return
	}
	normalizeTalentTrustPhoto(&photo)
	if photo.TitleEn == "" || photo.ImagePath == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Title EN and image are required"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.TalentTrustPhoto{}).
		Where("id = ?", uint(id)).
		Select("title_en", "title_id", "category", "image_path", "`order`", "is_active").
		Updates(photo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save trust photo"})
		return
	}
	photo.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge trust photo saved", Data: photo})
}

func (h *Controller) DeleteTalentTrustPhoto(c *gin.Context) {
	deleteRow[models.TalentTrustPhoto](h.db, "Talent Bridge trust photo deleted")(c)
}

func normalizeTalentTrustPhoto(photo *models.TalentTrustPhoto) {
	photo.TitleEn = strings.TrimSpace(photo.TitleEn)
	photo.TitleID = strings.TrimSpace(photo.TitleID)
	photo.Category = strings.ToLower(strings.TrimSpace(photo.Category))
	if photo.Category == "" {
		photo.Category = "general"
	}
	photo.ImagePath = strings.TrimSpace(photo.ImagePath)
}

func (h *Controller) ListHiringInquiries(c *gin.Context) {
	type row struct {
		models.HiringInquiry
		JobID              uint  `json:"job_id"`
		HiredCount         int64 `json:"hired_count"`
		RemainingPositions int   `json:"remaining_positions"`
	}
	var rows []row
	if err := h.db.WithContext(c.Request.Context()).Raw(`
		SELECT h.*, COALESCE(MAX(j.id), 0) AS job_id,
			COUNT(DISTINCT CASE WHEN a.status = 'hired' THEN a.id END) AS hired_count
		FROM hiring_inquiries h
		LEFT JOIN talent_jobs j ON j.hiring_inquiry_id = h.id
		LEFT JOIN talent_job_applications a ON a.job_id = j.id
		GROUP BY h.id
		ORDER BY h.created_at DESC
	`).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load hiring inquiries"})
		return
	}
	for index := range rows {
		remaining := rows[index].Headcount - int(rows[index].HiredCount)
		if remaining < 0 {
			remaining = 0
		}
		rows[index].RemainingPositions = remaining
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Hiring inquiries loaded", Data: rows})
}

func (h *Controller) UpdateHiringInquiry(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid hiring inquiry id"})
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid status"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	allowed := map[string]bool{"new": true, "contacted": true, "approved": true, "drafted": true, "rejected": true}
	if !allowed[status] {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Unsupported hiring status"})
		return
	}
	var createdJob *models.TalentJob
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var inquiry models.HiringInquiry
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inquiry, uint(id)).Error; err != nil {
			return err
		}
		if status == "drafted" {
			var job models.TalentJob
			err := tx.Where("hiring_inquiry_id = ?", inquiry.ID).First(&job).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				job = models.TalentJob{
					HiringInquiryID: inquiry.ID,
					TitleEn:         inquiry.RolesNeeded,
					TitleID:         inquiry.RolesNeeded,
					Slug:            fmt.Sprintf("%s-%d", services.GenerateSlug(inquiry.RolesNeeded), inquiry.ID),
					DescriptionEn:   inquiry.Challenge,
					DescriptionID:   inquiry.Challenge,
					OpenPositions:   inquiry.Headcount,
					Status:          "draft",
				}
				if err := tx.Create(&job).Error; err != nil {
					return err
				}
				createdJob = &job
			} else if err != nil {
				return err
			} else {
				if err := tx.Model(&job).Updates(map[string]any{"open_positions": inquiry.Headcount, "status": "draft"}).Error; err != nil {
					return err
				}
				createdJob = &job
			}
		}
		return tx.Model(&inquiry).Update("status", status).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update hiring inquiry"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Hiring inquiry saved", Data: gin.H{"job": createdJob}})
}

func (h *Controller) ListTalentPlusApplications(c *gin.Context) {
	listRows[models.TalentPlusApplication](h.db, "created_at desc", "Talent Bridge+ applications loaded")(c)
}

func (h *Controller) UpdateTalentPlusApplication(c *gin.Context) {
	h.updateTalentApplicationStatus(c, "plus", "Talent Bridge+ application saved")
}

func (h *Controller) ListTalentApplications(c *gin.Context) {
	applications, err := talentApplicationRows(c.Request.Context(), h.db, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load applications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent applications loaded", Data: applications})
}

func (h *Controller) ListTalentJobApplications(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid job id"})
		return
	}
	applications, err := talentApplicationRows(c.Request.Context(), h.db, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load applications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent applications loaded", Data: applications})
}

func (h *Controller) UpdateTalentApplication(c *gin.Context) {
	h.updateTalentApplicationStatus(c, "job", "Talent application saved")
}

func (h *Controller) updateTalentApplicationStatus(c *gin.Context, applicationType string, message string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Status) == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid status"})
		return
	}
	status := strings.TrimSpace(req.Status)
	if applicationType == "plus" {
		if err := h.db.WithContext(c.Request.Context()).Model(&models.TalentPlusApplication{}).Where("id = ?", uint(id)).Update("status", status).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update status"})
			return
		}
	} else {
		if err := h.updateJobApplicationStatus(c.Request.Context(), uint(id), status); err != nil {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
			return
		}
	}
	if services.TalentReviewStatusEligible(applicationType, status) {
		if _, err := h.sendTalentReviewInvitation(c.Request.Context(), applicationType, uint(id), false); err != nil {
			c.JSON(http.StatusOK, structs.Response{Success: true, Message: message + ", but automatic review invitation failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message + " and review invitation queued"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: message})
}

func (h *Controller) updateJobApplicationStatus(ctx context.Context, applicationID uint, status string) error {
	allowed := map[string]bool{"new": true, "screening": true, "proposed": true, "rejected": true}
	if !allowed[status] {
		return fmt.Errorf("unsupported application status")
	}
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var application models.TalentJobApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&application, applicationID).Error; err != nil {
			return fmt.Errorf("application not found")
		}
		var job models.TalentJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, application.JobID).Error; err != nil {
			return fmt.Errorf("job not found")
		}
		if err := tx.Model(&application).Update("status", status).Error; err != nil {
			return err
		}
		if status == "proposed" && job.HiringInquiryID != 0 {
			return tx.Model(&models.HiringInquiry{}).Where("id = ? AND status <> ?", job.HiringInquiryID, "resolved").Update("status", "in_progress").Error
		}
		return nil
	})
}

func (h *Controller) DownloadTalentApplicationCV(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid application id"})
		return
	}
	var application models.TalentJobApplication
	if err := h.db.WithContext(c.Request.Context()).First(&application, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Application not found"})
		return
	}
	extension := strings.ToLower(filepath.Ext(application.CVPath))
	if extension == "" {
		extension = ".bin"
	}
	if contentType := mime.TypeByExtension(extension); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	reader, info, err := services.OpenStoredUploadPath(c.Request.Context(), h.db, h.cfg, application.CVPath)
	if err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "CV file not found"})
		return
	}
	defer reader.Close()
	if info.ContentType != "" {
		c.Header("Content-Type", info.ContentType)
	}
	c.Header("Content-Disposition", `attachment; filename="candidate-cv`+extension+`"`)
	_, _ = io.Copy(c.Writer, reader)
}

func (h *Controller) ListPartnerApplications(c *gin.Context) {
	listRows[models.PartnerApplication](h.db, "created_at desc", "Partner applications loaded")(c)
}

func (h *Controller) UpdatePartnerApplication(c *gin.Context) {
	updateLeadStatus[models.PartnerApplication](h.db, "Partner application saved")(c)
}

func (h *Controller) ListPartners(c *gin.Context) {
	listRows[models.Partner](h.db, "created_at desc", "Partners loaded")(c)
}

func (h *Controller) CreatePartner(c *gin.Context) {
	var partner models.Partner
	if err := c.ShouldBindJSON(&partner); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid partner payload"})
		return
	}
	normalizePartner(&partner)
	if partner.Name == "" || partner.Country == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Partner name and country are required"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&partner).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create partner"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Partner created", Data: partner})
}

func (h *Controller) UpdatePartner(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid partner id"})
		return
	}
	var partner models.Partner
	if err := c.ShouldBindJSON(&partner); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid partner payload"})
		return
	}
	normalizePartner(&partner)
	if partner.Name == "" || partner.Country == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Partner name and country are required"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.Partner{}).
		Where("id = ?", uint(id)).
		Select("partner_type", "name", "country", "website", "contact_info", "description_en", "description_id", "status", "notes", "is_active").
		Updates(partner).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save partner"})
		return
	}
	partner.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Partner saved", Data: partner})
}

func (h *Controller) DeletePartner(c *gin.Context) {
	deleteRow[models.Partner](h.db, "Partner deleted")(c)
}

func (h *Controller) UpdatePartnerLogo(c *gin.Context) {
	h.updateUploadField(c, h.uploadService.SavePartnerLogo, &models.Partner{}, "logo", "Partner logo uploaded")
}

func normalizePartner(partner *models.Partner) {
	partner.PartnerType = strings.ToLower(strings.TrimSpace(partner.PartnerType))
	if partner.PartnerType != "company" {
		partner.PartnerType = "university"
	}
	partner.Name = strings.TrimSpace(partner.Name)
	partner.Country = strings.TrimSpace(partner.Country)
	partner.Website = strings.TrimSpace(partner.Website)
	partner.ContactInfo = strings.TrimSpace(partner.ContactInfo)
	partner.DescriptionEn = strings.TrimSpace(partner.DescriptionEn)
	partner.DescriptionID = strings.TrimSpace(partner.DescriptionID)
	partner.Notes = strings.TrimSpace(partner.Notes)
	partner.Status = strings.ToLower(strings.TrimSpace(partner.Status))
	if partner.Status != "inactive" {
		partner.Status = "active"
	}
	partner.IsActive = partner.Status == "active"
}

func (h *Controller) ListLeadNotes(c *gin.Context) {
	leadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	leadType := strings.TrimSpace(c.Param("type"))
	if err != nil || leadID == 0 || leadType == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid lead note target"})
		return
	}
	var notes []models.LeadNote
	if err := h.db.WithContext(c.Request.Context()).Where("lead_id = ? AND lead_type = ?", uint(leadID), leadType).Order("created_at desc").Find(&notes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load notes"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Lead notes loaded", Data: notes})
}

func (h *Controller) CreateLeadNote(c *gin.Context) {
	leadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	leadType := strings.TrimSpace(c.Param("type"))
	if err != nil || leadID == 0 || leadType == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid lead note target"})
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Note) == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Note is required"})
		return
	}
	adminID := uint(0)
	if value, exists := c.Get("user_id"); exists {
		if id, ok := value.(uint); ok {
			adminID = id
		}
	}
	note := models.LeadNote{LeadID: uint(leadID), LeadType: leadType, Note: strings.TrimSpace(req.Note), CreatedBy: adminID}
	if err := h.db.WithContext(c.Request.Context()).Create(&note).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save note"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Lead note saved", Data: note})
}

func (h *Controller) ListEvents(c *gin.Context) {
	listRows[models.Event](h.db, "start_date desc", "Events loaded")(c)
}

func (h *Controller) CreateEvent(c *gin.Context) {
	createRow[models.Event](h.db, "Event created")(c)
}

func (h *Controller) UpdateEvent(c *gin.Context) {
	updateRow[models.Event](h.db, "Event saved")(c)
}

func (h *Controller) DeleteEvent(c *gin.Context) {
	deleteRow[models.Event](h.db, "Event deleted")(c)
}

func updateLeadStatus[T any](db *gorm.DB, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
			return
		}
		var req struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Status) == "" {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Status is required"})
			return
		}
		if err := db.WithContext(c.Request.Context()).Model(new(T)).Where("id = ?", uint(id)).Update("status", strings.TrimSpace(req.Status)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update status"})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message})
	}
}

type talentApplicationRow struct {
	ID              uint      `json:"id"`
	JobID           uint      `json:"job_id"`
	JobTitleEn      string    `json:"job_title_en"`
	JobTitleID      string    `json:"job_title_id"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	CVPath          string    `json:"cv_path"`
	Status          string    `json:"status"`
	AppliedAt       time.Time `json:"applied_at"`
	CreatedAt       time.Time `json:"created_at"`
	OpenPositions   int       `json:"open_positions"`
	HiringInquiryID uint      `json:"hiring_inquiry_id"`
	HiredCount      int64     `json:"hired_count"`
}

func talentApplicationRows(ctx context.Context, db *gorm.DB, jobID uint) ([]talentApplicationRow, error) {
	query := db.WithContext(ctx).Table("talent_job_applications a").
		Select(`a.id, a.job_id, j.title_en AS job_title_en, j.title_id AS job_title_id, a.name, a.email, a.cv_path, a.status, a.applied_at, a.created_at, j.open_positions, j.hiring_inquiry_id,
			(SELECT COUNT(*) FROM talent_job_applications hired WHERE hired.job_id = j.id AND hired.status = 'hired') AS hired_count`).
		Joins("JOIN talent_jobs j ON j.id = a.job_id").
		Order("a.applied_at desc, a.id desc")
	if jobID != 0 {
		query = query.Where("a.job_id = ?", jobID)
	}
	var rows []talentApplicationRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
