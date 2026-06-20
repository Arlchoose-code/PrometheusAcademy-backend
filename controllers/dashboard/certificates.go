package dashboard

import (
	"fmt"
	"net/http"
	"strings"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) DownloadCertificate(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	certificateUUID := strings.TrimSpace(c.Param("uuid"))
	if !services.LooksLikeUUID(certificateUUID) {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Certificate not found"})
		return
	}
	var certificate models.Certificate
	if err := h.db.WithContext(c.Request.Context()).Where("uuid = ? OR certificate_code = ?", certificateUUID, strings.ToUpper(certificateUUID)).First(&certificate).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Certificate not found"})
		return
	}
	if err := services.EnsureCertificateUUID(c.Request.Context(), h.db, &certificate); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to prepare certificate ID"})
		return
	}
	if !user.IsAdmin && certificate.UserID != user.ID {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
		return
	}
	renderLocale := strings.ToLower(strings.TrimSpace(c.Query("locale")))
	if renderLocale == "en" || renderLocale == "id" {
		certificate.Locale = renderLocale
	}
	pdf, err := services.EnsureCertificatePDFBytes(c.Request.Context(), h.db, h.cfg, certificate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load certificate"})
		return
	}
	c.Header("X-Certificate-Code", services.CertificateDisplayCode(certificate))
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="prometheus-certificate-%s.pdf"`, services.CertificateDisplayCode(certificate)))
	_, _ = c.Writer.Write(pdf)
}
