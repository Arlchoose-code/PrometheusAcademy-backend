package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) ListAutomationWorkflows(c *gin.Context) {
	var workflows []models.AutomationWorkflow
	_ = h.db.WithContext(c.Request.Context()).Order("category asc, id asc").Find(&workflows).Error
	for index := range workflows {
		if strings.TrimSpace(workflows[index].TemplateKey) == "" {
			workflows[index].TemplateKey = defaultAutomationTemplateKey(workflows[index].Key)
			_ = h.db.WithContext(c.Request.Context()).Model(&models.AutomationWorkflow{}).Where("id = ?", workflows[index].ID).Update("template_key", workflows[index].TemplateKey).Error
		}
	}
	runPage, _ := strconv.Atoi(c.DefaultQuery("run_page", "1"))
	runPerPage, _ := strconv.Atoi(c.DefaultQuery("run_per_page", "12"))
	if runPage < 1 {
		runPage = 1
	}
	if runPerPage < 1 || runPerPage > 100 {
		runPerPage = 12
	}
	var runTotal int64
	h.db.WithContext(c.Request.Context()).Model(&models.AutomationRun{}).Count(&runTotal)
	var runs []models.AutomationRun
	_ = h.db.WithContext(c.Request.Context()).Order("created_at desc").Limit(runPerPage).Offset((runPage - 1) * runPerPage).Find(&runs).Error
	var stats struct{ Scheduled, Sent, Failed, Suppressed int }
	h.db.WithContext(c.Request.Context()).Raw(`SELECT SUM(status='scheduled') scheduled, SUM(status='sent') sent, SUM(status='failed') failed, SUM(status='suppressed') suppressed FROM automation_runs`).Scan(&stats)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Automations loaded", Data: gin.H{"workflows": workflows, "runs": runs, "stats": stats, "runs_pagination": gin.H{"page": runPage, "per_page": runPerPage, "total": runTotal, "total_pages": (runTotal + int64(runPerPage) - 1) / int64(runPerPage)}}})
}

func defaultAutomationTemplateKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	return "automation_" + strings.NewReplacer("-", "_").Replace(key)
}

func (h *Controller) UpdateAutomationWorkflow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, structs.Response{Success: false, Message: "Invalid workflow id"})
		return
	}
	var req struct {
		Name         string `json:"name"`
		DelayMinutes int    `json:"delay_minutes"`
		TemplateKey  string `json:"template_key"`
		SubjectEn    string `json:"subject_en"`
		SubjectID    string `json:"subject_id"`
		BodyEn       string `json:"body_en"`
		BodyID       string `json:"body_id"`
		IsEnabled    bool   `json:"is_enabled"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, structs.Response{Success: false, Message: "Invalid workflow payload"})
		return
	}
	if req.DelayMinutes < 0 {
		req.DelayMinutes = 0
	}
	err = h.db.WithContext(c.Request.Context()).Model(&models.AutomationWorkflow{}).Where("id = ?", uint(id)).Updates(map[string]any{"name": strings.TrimSpace(req.Name), "delay_minutes": req.DelayMinutes, "template_key": strings.TrimSpace(req.TemplateKey), "subject_en": req.SubjectEn, "subject_id": req.SubjectID, "body_en": req.BodyEn, "body_id": req.BodyID, "is_enabled": req.IsEnabled}).Error
	if err != nil {
		c.JSON(500, structs.Response{Success: false, Message: "Failed to save workflow"})
		return
	}
	c.JSON(200, structs.Response{Success: true, Message: "Workflow saved"})
}

func (h *Controller) ListSuppressions(c *gin.Context) {
	var rows []models.EmailSuppression
	h.db.Order("created_at desc").Find(&rows)
	c.JSON(200, structs.Response{Success: true, Message: "Suppressions loaded", Data: rows})
}
func (h *Controller) CreateSuppression(c *gin.Context) {
	var row models.EmailSuppression
	if c.ShouldBindJSON(&row) != nil || !strings.Contains(row.Email, "@") {
		c.JSON(400, structs.Response{Success: false, Message: "Valid email is required"})
		return
	}
	row.Email = strings.ToLower(strings.TrimSpace(row.Email))
	if row.Reason == "" {
		row.Reason = "manual"
	}
	if h.db.Where("email = ?", row.Email).FirstOrCreate(&row).Error != nil {
		c.JSON(500, structs.Response{Success: false, Message: "Failed to suppress email"})
		return
	}
	c.JSON(200, structs.Response{Success: true, Message: "Email suppressed", Data: row})
}
func (h *Controller) DeleteSuppression(c *gin.Context) {
	h.db.Delete(&models.EmailSuppression{}, c.Param("id"))
	c.JSON(200, structs.Response{Success: true, Message: "Suppression removed"})
}

func (h *Controller) GetAutomationAnalytics(c *gin.Context) {
	var events []struct {
		EventType string `json:"event_type"`
		Count     int    `json:"count"`
		Revenue   int    `json:"revenue"`
	}
	h.db.WithContext(c.Request.Context()).Raw(`SELECT event_type, COUNT(*) count, COALESCE(SUM(revenue),0) revenue FROM email_events GROUP BY event_type`).Scan(&events)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "12"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 12
	}
	var total int64
	h.db.WithContext(c.Request.Context()).Model(&models.EmailCampaign{}).Count(&total)
	var campaignSummary struct {
		Sent   int `json:"sent"`
		Failed int `json:"failed"`
	}
	h.db.WithContext(c.Request.Context()).Model(&models.EmailCampaign{}).Select("COALESCE(SUM(sent_count),0) sent, COALESCE(SUM(failed_count),0) failed").Scan(&campaignSummary)
	var campaigns []models.EmailCampaign
	h.db.Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&campaigns)
	c.JSON(200, structs.Response{Success: true, Message: "Automation analytics loaded", Data: gin.H{"events": events, "campaigns": campaigns, "campaign_summary": campaignSummary, "campaigns_pagination": gin.H{"page": page, "per_page": perPage, "total": total, "total_pages": (total + int64(perPage) - 1) / int64(perPage)}}})
}

type pipelineLead struct {
	ID        uint      `json:"id"`
	LeadType  string    `json:"lead_type"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Controller) ListCRMPipeline(c *gin.Context) {
	var rows []pipelineLead
	h.db.WithContext(c.Request.Context()).Raw(`
		SELECT id,'contact' lead_type,name,email,'General inquiry' source,status,created_at FROM contact_leads
		UNION ALL SELECT id,'hiring',CONCAT(first_name,' ',last_name),work_email,'Hiring' source,status,created_at FROM hiring_inquiries
		UNION ALL SELECT id,'talent',name,email,'Talent applicant' source,status,created_at FROM talent_job_applications
		UNION ALL SELECT id,'talent_plus',CONCAT(first_name,' ',last_name),email,'Talent Bridge+' source,status,created_at FROM talent_plus_applications
		UNION ALL SELECT id,'partner',contact_person,email,'Partner' source,status,created_at FROM partner_applications
		ORDER BY created_at DESC`).Scan(&rows)
	c.JSON(200, structs.Response{Success: true, Message: "CRM pipeline loaded", Data: rows})
}
func (h *Controller) UpdateCRMPipelineStage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, structs.Response{Success: false, Message: "Invalid lead id"})
		return
	}
	leadType := c.Param("type")
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, structs.Response{Success: false, Message: "Invalid stage"})
		return
	}
	allowed := map[string]bool{"new_lead": true, "contacted": true, "qualified": true, "won": true, "lost": true}
	if !allowed[req.Status] {
		c.JSON(400, structs.Response{Success: false, Message: "Unsupported CRM stage"})
		return
	}
	modelsByType := map[string]any{"contact": &models.ContactLead{}, "hiring": &models.HiringInquiry{}, "talent": &models.TalentJobApplication{}, "talent_plus": &models.TalentPlusApplication{}, "partner": &models.PartnerApplication{}}
	model, ok := modelsByType[leadType]
	if !ok {
		c.JSON(400, structs.Response{Success: false, Message: "Unsupported lead type"})
		return
	}
	if h.db.Model(model).Where("id = ?", uint(id)).Update("status", req.Status).Error != nil {
		c.JSON(500, structs.Response{Success: false, Message: "Failed to move lead"})
		return
	}
	c.JSON(200, structs.Response{Success: true, Message: "Lead stage updated"})
}
