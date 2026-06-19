package admin

import (
	"context"
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
		job.Status = "open"
	}
	if job.OpenPositions <= 0 {
		job.OpenPositions = 1
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create job"})
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
	if err := h.db.WithContext(c.Request.Context()).Model(&models.TalentJob{}).Where("id = ?", uint(id)).Select("title_en", "title_id", "slug", "description_en", "description_id", "open_positions", "status").Updates(job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save job"})
		return
	}
	job.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent job saved", Data: job})
}

func (h *Controller) DeleteTalentJob(c *gin.Context) {
	deleteRow[models.TalentJob](h.db, "Talent job deleted")(c)
}

func (h *Controller) ListHiringInquiries(c *gin.Context) {
	listRows[models.HiringInquiry](h.db, "created_at desc", "Hiring inquiries loaded")(c)
}

func (h *Controller) UpdateHiringInquiry(c *gin.Context) {
	updateLeadStatus[models.HiringInquiry](h.db, "Hiring inquiry saved")(c)
}

func (h *Controller) ListTalentPlusApplications(c *gin.Context) {
	listRows[models.TalentPlusApplication](h.db, "created_at desc", "Talent Bridge+ applications loaded")(c)
}

func (h *Controller) UpdateTalentPlusApplication(c *gin.Context) {
	updateLeadStatus[models.TalentPlusApplication](h.db, "Talent Bridge+ application saved")(c)
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
	updateLeadStatus[models.TalentJobApplication](h.db, "Talent application saved")(c)
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
	c.FileAttachment(services.StorageFilePath(h.cfg, application.CVPath), "candidate-cv"+extension)
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
	ID            uint      `json:"id"`
	JobID         uint      `json:"job_id"`
	JobTitleEn    string    `json:"job_title_en"`
	JobTitleID    string    `json:"job_title_id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	CVPath        string    `json:"cv_path"`
	Status        string    `json:"status"`
	AppliedAt     time.Time `json:"applied_at"`
	CreatedAt     time.Time `json:"created_at"`
	OpenPositions int       `json:"open_positions"`
}

func talentApplicationRows(ctx context.Context, db *gorm.DB, jobID uint) ([]talentApplicationRow, error) {
	query := db.WithContext(ctx).Table("talent_job_applications a").
		Select(`a.id, a.job_id, j.title_en AS job_title_en, j.title_id AS job_title_id, a.name, a.email, a.cv_path, a.status, a.applied_at, a.created_at, j.open_positions`).
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
