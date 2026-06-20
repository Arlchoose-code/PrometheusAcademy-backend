package admin

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
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
	invitation, err := h.sendTalentReviewInvitation(c.Request.Context(), req.ApplicationType, req.ApplicationID, true)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "eligible") || strings.Contains(err.Error(), "already submitted") {
			status = http.StatusConflict
		}
		c.JSON(status, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Review invitation sent", Data: invitation})
}

func (h *Controller) sendTalentReviewInvitation(ctx context.Context, applicationType string, applicationID uint, force bool) (models.TalentReviewInvitation, error) {
	applicationType = strings.ToLower(strings.TrimSpace(applicationType))
	recipient, err := services.TalentReviewRecipientForApplication(h.db.WithContext(ctx), applicationType, applicationID)
	if err != nil {
		return models.TalentReviewInvitation{}, errors.New("Talent application not found")
	}
	if !services.TalentReviewStatusEligible(applicationType, recipient.Status) {
		return models.TalentReviewInvitation{}, errors.New("Application must be accepted, placed, or completed before inviting a review")
	}

	var invitation models.TalentReviewInvitation
	err = h.db.WithContext(ctx).Where("application_type = ? AND application_id = ?", applicationType, applicationID).First(&invitation).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.TalentReviewInvitation{}, errors.New("Failed to load review invitation")
	}
	if invitation.UsedAt != nil {
		return models.TalentReviewInvitation{}, errors.New("This application has already submitted a review")
	}
	if !force && invitation.SentAt != nil {
		return invitation, nil
	}

	rawToken, tokenHash, err := services.GenerateTalentReviewToken()
	if err != nil {
		return models.TalentReviewInvitation{}, errors.New("Failed to generate review invitation")
	}
	inviteHours := 168
	var inviteSetting models.Setting
	if err := h.db.WithContext(ctx).Where("`key` = ?", "talent_review_invite_hours").First(&inviteSetting).Error; err == nil {
		if configuredHours, parseErr := strconv.Atoi(strings.TrimSpace(inviteSetting.Value)); parseErr == nil && configuredHours > 0 {
			inviteHours = configuredHours
		}
	}
	expiresAt := time.Now().Add(time.Duration(inviteHours) * time.Hour)
	invitation.ApplicationType = applicationType
	invitation.ApplicationID = applicationID
	invitation.Name = recipient.Name
	invitation.Email = strings.ToLower(strings.TrimSpace(recipient.Email))
	invitation.TokenHash = tokenHash
	invitation.ExpiresAt = expiresAt
	invitation.SentAt = nil
	if err := h.db.WithContext(ctx).Save(&invitation).Error; err != nil {
		return models.TalentReviewInvitation{}, errors.New("Failed to save review invitation")
	}

	locale := "en"
	var user models.User
	if err := h.db.WithContext(ctx).Where("LOWER(email) = ?", invitation.Email).First(&user).Error; err == nil && user.Language == "id" {
		locale = "id"
	}
	reviewURL := strings.TrimRight(h.cfg.FrontendURL, "/") + "/" + locale + "/talent-bridge/review/" + url.PathEscape(rawToken)
	mailUser := models.User{Name: invitation.Name, Email: invitation.Email, Language: locale}
	if err := services.SendTransactionalTemplateEmail(ctx, h.db, services.EmailTemplateTalentReviewInvite, "talent_review_invitation", mailUser, map[string]string{
		"review_url": reviewURL,
		"expires_at": expiresAt.Format("02 Jan 2006 15:04 MST"),
	}); err != nil {
		return models.TalentReviewInvitation{}, err
	}

	now := time.Now()
	invitation.SentAt = &now
	if err := h.db.WithContext(ctx).Model(&invitation).Update("sent_at", now).Error; err != nil {
		return models.TalentReviewInvitation{}, errors.New("Review invitation sent but delivery status could not be saved")
	}
	return invitation, nil
}
