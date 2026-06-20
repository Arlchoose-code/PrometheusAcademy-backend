package public

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) HandleEmailEventWebhook(c *gin.Context) {
	secret := strings.TrimSpace(os.Getenv("EMAIL_EVENT_WEBHOOK_SECRET"))
	provided := strings.TrimSpace(c.GetHeader("X-Webhook-Secret"))
	if secret == "" {
		c.JSON(http.StatusServiceUnavailable, structs.Response{Success: false, Message: "Email event webhook is not configured"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(secret), []byte(provided)) != 1 {
		c.JSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Invalid webhook signature"})
		return
	}
	var req struct {
		Event      string     `json:"event"`
		Email      string     `json:"email"`
		MessageID  string     `json:"message_id"`
		CampaignID uint       `json:"campaign_id"`
		RunID      uint       `json:"run_id"`
		Revenue    int        `json:"revenue"`
		OccurredAt *time.Time `json:"occurred_at"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, structs.Response{Success: false, Message: "Invalid email event"})
		return
	}
	eventType := strings.ToLower(strings.TrimSpace(req.Event))
	allowed := map[string]bool{"delivered": true, "open": true, "click": true, "bounce": true, "complaint": true, "unsubscribe": true, "conversion": true}
	if !allowed[eventType] {
		c.JSON(400, structs.Response{Success: false, Message: "Unsupported email event"})
		return
	}
	occurred := time.Now()
	if req.OccurredAt != nil {
		occurred = *req.OccurredAt
	}
	event := models.EmailEvent{CampaignID: req.CampaignID, RunID: req.RunID, Email: strings.ToLower(strings.TrimSpace(req.Email)), MessageID: req.MessageID, EventType: eventType, Revenue: req.Revenue, OccurredAt: occurred}
	if h.db.WithContext(c.Request.Context()).Create(&event).Error != nil {
		c.JSON(500, structs.Response{Success: false, Message: "Failed to store email event"})
		return
	}
	if eventType == "bounce" || eventType == "complaint" || eventType == "unsubscribe" {
		suppression := models.EmailSuppression{Email: event.Email, Reason: eventType, Source: "email_webhook"}
		h.db.Where("email = ?", event.Email).FirstOrCreate(&suppression)
	}
	c.JSON(200, structs.Response{Success: true, Message: "Email event recorded"})
}
