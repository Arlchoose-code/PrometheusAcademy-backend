package public

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) HandleMidtransWebhook(c *gin.Context) {
	if strings.TrimSpace(h.cfg.MidtransServerKey) == "" {
		c.JSON(http.StatusServiceUnavailable, structs.Response{Success: false, Message: "Payment webhook is not configured"})
		return
	}
	var req structs.MidtransWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.OrderID) == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid webhook payload"})
		return
	}
	expected := services.MidtransSignature(req.OrderID, req.StatusCode, req.GrossAmount, h.cfg.MidtransServerKey)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(req.SignatureKey)) != 1 {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Invalid signature"})
		return
	}
	status := services.MapMidtransStatus(req.Status)
	var order models.Order
	if err := h.db.WithContext(c.Request.Context()).Where("midtrans_order_id = ?", req.OrderID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Order not found"})
		return
	}
	raw, _ := json.Marshal(req)
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": status}
		if status == "success" {
			now := time.Now()
			updates["paid_at"] = &now
		}
		if err := tx.Model(&order).Updates(updates).Error; err != nil {
			return err
		}
		order.Status = status
		trx := models.Transaction{OrderID: order.ID, MidtransTransactionID: req.TransactionID, PaymentType: req.PaymentType, Status: status, RawResponse: string(raw)}
		if trx.MidtransTransactionID == "" {
			trx.MidtransTransactionID = "TRX-" + order.MidtransOrderID
		}
		if err := tx.Where(models.Transaction{MidtransTransactionID: trx.MidtransTransactionID}).Assign(trx).FirstOrCreate(&trx).Error; err != nil {
			return err
		}
		if status == "success" {
			if err := services.FulfillSuccessfulOrder(c.Request.Context(), tx, order); err != nil {
				return err
			}
			_, err := services.EnsureInvoice(c.Request.Context(), tx, h.cfg, order)
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to process webhook"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Webhook processed"})
}
