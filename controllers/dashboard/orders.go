package dashboard

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) CancelOrder(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Order not found"})
		return
	}
	var order models.Order
	if err := h.db.WithContext(c.Request.Context()).First(&order, uint(orderID)).Error; err != nil || (!user.IsAdmin && order.UserID != user.ID) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
		return
	}
	if order.Status != "pending" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Only pending orders can be cancelled"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&order).Update("status", "cancelled").Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to cancel order"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Order cancelled", Data: gin.H{"id": order.ID, "status": "cancelled"}})
}

func (h *Controller) PayOrder(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	_ = services.CancelExpiredPendingOrders(c.Request.Context(), h.db)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Order not found"})
		return
	}
	var order models.Order
	if err := h.db.WithContext(c.Request.Context()).First(&order, uint(orderID)).Error; err != nil || (!user.IsAdmin && order.UserID != user.ID) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
		return
	}
	if order.Status != "pending" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Order is not pending"})
		return
	}
	if order.PaymentExpiresAt != nil && order.PaymentExpiresAt.Before(time.Now()) {
		if err := h.db.WithContext(c.Request.Context()).Model(&order).Update("status", "cancelled").Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to expire order"})
			return
		}
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Payment window expired"})
		return
	}
	itemID, itemName, err := services.OrderPaymentItem(c.Request.Context(), h.db, order.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Order item not found"})
		return
	}
	token, redirectURL, err := services.EnsureOrderPaymentToken(c.Request.Context(), h.db, h.cfg, &order, itemID, itemName, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	data := services.OrderPaymentResponse(order, false)
	data["snap_token"] = token
	data["redirect_url"] = redirectURL
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Payment token loaded", Data: data})
}

func (h *Controller) SyncOrder(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Order not found"})
		return
	}
	var order models.Order
	if err := h.db.WithContext(c.Request.Context()).First(&order, uint(orderID)).Error; err != nil || (!user.IsAdmin && order.UserID != user.ID) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
		return
	}
	if order.Status == "success" {
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Order already synced", Data: services.OrderPaymentResponse(order, false)})
		return
	}
	if err := services.SyncOrderPaymentStatus(c.Request.Context(), h.db, h.cfg, &order); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Order synced", Data: services.OrderPaymentResponse(order, false)})
}

func (h *Controller) GetOrderInvoice(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Invoice not found"})
		return
	}
	var order models.Order
	if err := h.db.WithContext(c.Request.Context()).First(&order, uint(orderID)).Error; err != nil || (!user.IsAdmin && order.UserID != user.ID) || order.Status != "success" {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
		return
	}
	invoice, err := services.EnsureInvoice(c.Request.Context(), h.db, h.cfg, order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load invoice"})
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.pdf"`, invoice.InvoiceNumber))
	c.File(services.StorageFilePath(h.cfg, invoice.FilePath))
}

func (h *Controller) DownloadProductFile(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	productID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || productID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "File not found"})
		return
	}
	var count int64
	if err := h.db.WithContext(c.Request.Context()).
		Table("orders").
		Joins("JOIN order_items oi ON oi.order_id = orders.id").
		Where("orders.user_id = ? AND orders.status = ? AND oi.item_type = ? AND oi.item_id = ?", user.ID, "success", "product", uint(productID)).
		Count(&count).Error; err != nil || count == 0 {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
		return
	}
	var file models.ProductFile
	if err := h.db.WithContext(c.Request.Context()).Where("product_id = ?", uint(productID)).Order("id asc").First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "File not found"})
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.FileName))
	c.File(services.StorageFilePath(h.cfg, file.FilePath))
}
