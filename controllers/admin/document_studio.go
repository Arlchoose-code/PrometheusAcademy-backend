package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) ListDocumentTemplates(c *gin.Context) {
	docType := strings.TrimSpace(c.Query("type"))
	query := h.db.WithContext(c.Request.Context()).Model(&models.DocumentTemplate{}).Order("updated_at desc")
	if docType != "" {
		query = query.Where("document_type = ?", docType)
	}
	var rows []models.DocumentTemplate
	if err := query.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load document templates"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Document templates loaded", Data: rows})
}

func (h *Controller) GetDocumentTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template not found"})
		return
	}
	var tmpl models.DocumentTemplate
	if err := h.db.WithContext(c.Request.Context()).First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template not found"})
		return
	}
	var versions []models.DocumentTemplateVersion
	_ = h.db.WithContext(c.Request.Context()).Where("template_id = ?", tmpl.ID).Order("version desc").Find(&versions).Error
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Document template loaded", Data: gin.H{"template": tmpl, "versions": versions}})
}

func (h *Controller) CreateDocumentTemplate(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var req documentTemplatePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid template payload"})
		return
	}
	if err := normalizeDocumentTemplatePayload(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := validateDocumentTemplateHTML(req.DocumentType, req.HTMLEn); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	tmpl := models.DocumentTemplate{Name: req.Name, DocumentType: req.DocumentType, PaperSize: req.PaperSize, Orientation: req.Orientation, Status: "draft", CreatedBy: user.ID}
	if err := h.db.WithContext(c.Request.Context()).Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create template"})
		return
	}
	version := models.DocumentTemplateVersion{TemplateID: tmpl.ID, Version: 1, DesignJSONEn: req.DesignJSONEn, DesignJSONID: req.DesignJSONID, HTMLEn: req.HTMLEn, HTMLID: req.HTMLID, CSS: req.CSS, VariablesJSON: services.AllowedDocumentVariables(req.DocumentType), ChecksumSHA256: services.DocumentTemplateChecksum(req.HTMLEn, req.HTMLID, req.CSS, req.DesignJSONEn, req.DesignJSONID), RendererVersion: services.DocumentRendererVersion, CreatedBy: user.ID}
	if err := h.db.WithContext(c.Request.Context()).Create(&version).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create template version"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Document template created", Data: gin.H{"template": tmpl, "version": version}})
}

func (h *Controller) UpdateDocumentTemplate(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template not found"})
		return
	}
	var tmpl models.DocumentTemplate
	if err := h.db.WithContext(c.Request.Context()).First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template not found"})
		return
	}
	var req documentTemplatePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid template payload"})
		return
	}
	if req.DocumentType == "" {
		req.DocumentType = tmpl.DocumentType
	}
	if err := normalizeDocumentTemplatePayload(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if req.DocumentType != tmpl.DocumentType {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Document type cannot be changed after creation"})
		return
	}
	if err := validateDocumentTemplateHTML(req.DocumentType, req.HTMLEn); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	var latest models.DocumentTemplateVersion
	nextVersion := 1
	if err := h.db.WithContext(c.Request.Context()).Where("template_id = ?", tmpl.ID).Order("version desc").First(&latest).Error; err == nil {
		nextVersion = latest.Version + 1
	}
	var createdVersion models.DocumentTemplateVersion
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&tmpl).Updates(map[string]any{
			"name":        req.Name,
			"paper_size":  req.PaperSize,
			"orientation": req.Orientation,
			"status":      "draft",
		}).Error; err != nil {
			return err
		}
		version := models.DocumentTemplateVersion{TemplateID: tmpl.ID, Version: nextVersion, DesignJSONEn: req.DesignJSONEn, DesignJSONID: req.DesignJSONID, HTMLEn: req.HTMLEn, HTMLID: req.HTMLID, CSS: req.CSS, VariablesJSON: services.AllowedDocumentVariables(req.DocumentType), ChecksumSHA256: services.DocumentTemplateChecksum(req.HTMLEn, req.HTMLID, req.CSS, req.DesignJSONEn, req.DesignJSONID), RendererVersion: services.DocumentRendererVersion, CreatedBy: user.ID}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		createdVersion = version
		return tx.First(&tmpl, tmpl.ID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update template"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Document template updated", Data: gin.H{"template": tmpl, "version": createdVersion}})
}

func (h *Controller) PublishDocumentTemplate(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template not found"})
		return
	}
	var tmpl models.DocumentTemplate
	if err := h.db.WithContext(c.Request.Context()).First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template not found"})
		return
	}
	var version models.DocumentTemplateVersion
	if err := h.db.WithContext(c.Request.Context()).Where("template_id = ?", tmpl.ID).Order("version desc").First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Template version not found"})
		return
	}
	now := time.Now()
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.DocumentTemplate{}).Where("document_type = ?", tmpl.DocumentType).Update("is_default", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&tmpl).Updates(map[string]any{"status": "published", "is_default": true}).Error; err != nil {
			return err
		}
		return tx.Model(&version).Updates(map[string]any{"published_at": &now, "created_by": user.ID}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to publish template"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Document template published"})
}

func (h *Controller) PreviewDocumentTemplate(c *gin.Context) {
	var req struct {
		DocumentType string `json:"document_type"`
		HTML         string `json:"html"`
		Orientation  string `json:"orientation"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid preview payload"})
		return
	}
	vars := services.SampleDocumentVariables(req.DocumentType)
	pdf, err := services.RenderDocumentPDF(c.Request.Context(), h.cfg, req.HTML, vars, req.Orientation)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `inline; filename="document-preview.pdf"`)
	_, _ = c.Writer.Write(pdf)
}

type documentTemplatePayload struct {
	Name         string `json:"name"`
	DocumentType string `json:"document_type"`
	PaperSize    string `json:"paper_size"`
	Orientation  string `json:"orientation"`
	DesignJSONEn string `json:"design_json_en"`
	DesignJSONID string `json:"design_json_id"`
	HTMLEn       string `json:"html_en"`
	HTMLID       string `json:"html_id"`
	CSS          string `json:"css"`
}

func normalizeDocumentTemplatePayload(req *documentTemplatePayload) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return fmt.Errorf("Template name is required")
	}
	req.DocumentType = strings.TrimSpace(req.DocumentType)
	if req.DocumentType != "invoice" && req.DocumentType != "certificate" {
		return fmt.Errorf("Unsupported document type")
	}
	if req.PaperSize == "" {
		req.PaperSize = "A4"
	}
	if req.Orientation == "" {
		req.Orientation = "portrait"
		if req.DocumentType == "certificate" {
			req.Orientation = "landscape"
		}
	}
	if strings.TrimSpace(req.DesignJSONEn) == "" {
		req.DesignJSONEn = "[]"
	}
	if strings.TrimSpace(req.DesignJSONID) == "" {
		req.DesignJSONID = "[]"
	}
	return nil
}

func validateDocumentTemplateHTML(documentType string, html string) error {
	allowed := map[string]string{}
	var allowedList []string
	_ = json.Unmarshal([]byte(services.AllowedDocumentVariables(documentType)), &allowedList)
	for _, key := range allowedList {
		allowed[key] = "sample"
	}
	return services.ValidateDocumentVariables(html, allowed)
}
