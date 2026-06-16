package public

import (
	"net/http"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) GetTalentLanding(c *gin.Context) {
	var testimonials []models.Testimonial
	_ = h.db.WithContext(c.Request.Context()).Where("is_active = ?", true).Order("id desc").Limit(4).Find(&testimonials).Error
	var jobs []models.TalentJob
	_ = h.db.WithContext(c.Request.Context()).Where("status = ?", "open").Order("created_at desc").Limit(3).Find(&jobs).Error
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge data loaded", Data: gin.H{
		"testimonials": testimonials,
		"jobs":         jobs,
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
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Talent Bridge+ application submitted", Data: req})
}

func (h *Controller) ListPartners(c *gin.Context) {
	var partners []models.Partner
	if err := h.db.WithContext(c.Request.Context()).Where("is_active = ?", true).Order("created_at desc").Find(&partners).Error; err != nil {
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
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Application submitted", Data: application})
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
