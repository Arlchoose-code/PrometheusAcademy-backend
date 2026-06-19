package public

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *Controller) GetTalentReviewInvitation(c *gin.Context) {
	invitation, status, message := h.validTalentReviewInvitation(c.Param("token"))
	if status != http.StatusOK {
		c.JSON(status, structs.Response{Success: false, Message: message})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Review invitation loaded", Data: gin.H{
		"name":       invitation.Name,
		"expires_at": invitation.ExpiresAt,
	}})
}

func (h *Controller) SubmitTalentReviewInvitation(c *gin.Context) {
	var req structs.TalentReviewSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid review payload"})
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Rating < 1 || req.Rating > 5 || len(req.Content) < 10 || len(req.Content) > 2000 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Rating and a review between 10 and 2000 characters are required"})
		return
	}
	tokenHash := services.HashTalentReviewToken(c.Param("token"))
	if tokenHash == services.HashTalentReviewToken("") {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Review invitation not found"})
		return
	}

	var testimonial models.Testimonial
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var invitation models.TalentReviewInvitation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(&invitation).Error; err != nil {
			return err
		}
		if invitation.UsedAt != nil {
			return fmt.Errorf("invitation already used")
		}
		if time.Now().After(invitation.ExpiresAt) {
			return fmt.Errorf("invitation expired")
		}
		testimonial = models.Testimonial{
			Name:           invitation.Name,
			Role:           "Talent Bridge Participant",
			ContentEn:      req.Content,
			ContentID:      req.Content,
			Rating:         req.Rating,
			ReviewSource:   "student",
			DisplayContext: "talent_bridge",
			ReviewStatus:   "pending",
			ExternalID:     fmt.Sprintf("talent-review-invitation-%d", invitation.ID),
			IsActive:       false,
		}
		if err := tx.Create(&testimonial).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&invitation).Updates(map[string]any{"used_at": now, "testimonial_id": testimonial.ID}).Error
	})
	if err != nil {
		message := "Review invitation is invalid or expired"
		status := http.StatusGone
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
			message = "Review invitation not found"
		}
		c.JSON(status, structs.Response{Success: false, Message: message})
		return
	}
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Review submitted for approval"})
}

func (h *Controller) validTalentReviewInvitation(token string) (models.TalentReviewInvitation, int, string) {
	var invitation models.TalentReviewInvitation
	token = strings.TrimSpace(token)
	if token == "" {
		return invitation, http.StatusNotFound, "Review invitation not found"
	}
	if err := h.db.Where("token_hash = ?", services.HashTalentReviewToken(token)).First(&invitation).Error; err != nil {
		return invitation, http.StatusNotFound, "Review invitation not found"
	}
	if invitation.UsedAt != nil {
		return invitation, http.StatusGone, "Review invitation has already been used"
	}
	if time.Now().After(invitation.ExpiresAt) {
		return invitation, http.StatusGone, "Review invitation has expired"
	}
	return invitation, http.StatusOK, ""
}
