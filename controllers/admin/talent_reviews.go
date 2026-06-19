package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) SendTalentReviewInvitation(c *gin.Context) {
	var req structs.TalentReviewInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ApplicationID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid review invitation payload"})
		return
	}
	req.ApplicationType = strings.ToLower(strings.TrimSpace(req.ApplicationType))
	recipient, err := services.TalentReviewRecipientForApplication(h.db.WithContext(c.Request.Context()), req.ApplicationType, req.ApplicationID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, structs.Response{Success: false, Message: "Talent application not found"})
		return
	}
	if !services.TalentReviewStatusEligible(req.ApplicationType, recipient.Status) {
		c.JSON(http.StatusConflict, structs.Response{Success: false, Message: "Application must be accepted, placed, or completed before inviting a review"})
		return
	}

	var invitation models.TalentReviewInvitation
	err = h.db.WithContext(c.Request.Context()).Where("application_type = ? AND application_id = ?", req.ApplicationType, req.ApplicationID).First(&invitation).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load review invitation"})
		return
	}
	if invitation.UsedAt != nil {
		c.JSON(http.StatusConflict, structs.Response{Success: false, Message: "This application has already submitted a review"})
		return
	}

	rawToken, tokenHash, err := services.GenerateTalentReviewToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate review invitation"})
		return
	}
	expiresAt := time.Now().Add(time.Duration(h.cfg.TalentReviewInviteHours) * time.Hour)
	invitation.ApplicationType = req.ApplicationType
	invitation.ApplicationID = req.ApplicationID
	invitation.Name = recipient.Name
	invitation.Email = strings.ToLower(strings.TrimSpace(recipient.Email))
	invitation.TokenHash = tokenHash
	invitation.ExpiresAt = expiresAt
	invitation.SentAt = nil
	if err := h.db.WithContext(c.Request.Context()).Save(&invitation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save review invitation"})
		return
	}

	locale := "en"
	var user models.User
	if err := h.db.WithContext(c.Request.Context()).Where("LOWER(email) = ?", invitation.Email).First(&user).Error; err == nil && user.Language == "id" {
		locale = "id"
	}
	reviewURL := strings.TrimRight(h.cfg.FrontendURL, "/") + "/" + locale + "/talent-bridge/review/" + url.PathEscape(rawToken)
	mailUser := models.User{Name: invitation.Name, Email: invitation.Email, Language: locale}
	if err := services.SendTransactionalTemplateEmail(c.Request.Context(), h.db, services.EmailTemplateTalentReviewInvite, "talent_review_invitation", mailUser, map[string]string{
		"review_url": reviewURL,
		"expires_at": expiresAt.Format("02 Jan 2006 15:04 MST"),
	}); err != nil {
		c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: err.Error()})
		return
	}

	now := time.Now()
	invitation.SentAt = &now
	if err := h.db.WithContext(c.Request.Context()).Model(&invitation).Update("sent_at", now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Review invitation sent but delivery status could not be saved"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Review invitation sent", Data: invitation})
}
