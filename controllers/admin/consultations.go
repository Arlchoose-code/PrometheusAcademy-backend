package admin

import (
	"net/http"
	"strconv"
	"strings"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) GetConsultationSettings(c *gin.Context) {
	c.JSON(http.StatusOK, structs.Response{
		Success: true,
		Message: "Consultation settings loaded",
		Data: gin.H{
			"reschedule_limit": services.ConsultationRescheduleLimit(c.Request.Context(), h.db),
		},
	})
}

func (h *Controller) UpdateConsultationSettings(c *gin.Context) {
	var req struct {
		RescheduleLimit int `json:"reschedule_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid consultation settings payload"})
		return
	}
	limit := req.RescheduleLimit
	if limit < 0 {
		limit = 0
	}
	if limit > 10 {
		limit = 10
	}
	setting := models.Setting{Key: services.ConsultationRescheduleLimitKey, Value: strconv.Itoa(limit)}
	if err := h.db.WithContext(c.Request.Context()).Where(models.Setting{Key: setting.Key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save consultation settings"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation settings saved", Data: gin.H{"reschedule_limit": limit}})
}

func (h *Controller) ListConsultationSlots(c *gin.Context) {
	slots, err := services.ListConsultationSlots(c.Request.Context(), h.db, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load slots"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slots loaded", Data: slots})
}

func (h *Controller) CreateConsultationSlot(c *gin.Context) {
	var req services.ConsultationSlotPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot payload"})
		return
	}
	slot, err := services.ConsultationSlotFromPayload(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Date, start time, and end time are required"})
		return
	}
	if !h.validConsultationSlotOwner(c, slot.OwnerID) {
		return
	}
	if slot.OwnerID != 0 {
		slot.Capacity = 1
	}
	if err := services.CreateConsultationSlotRecord(c.Request.Context(), h.db, &slot); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot created", Data: slot})
}

func (h *Controller) UpdateConsultationSlot(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot id"})
		return
	}
	var req services.ConsultationSlotPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot payload"})
		return
	}
	slot, err := services.ConsultationSlotFromPayload(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Date, start time, and end time are required"})
		return
	}
	if !h.validConsultationSlotOwner(c, slot.OwnerID) {
		return
	}
	if slot.OwnerID != 0 {
		slot.Capacity = 1
	}
	if err := services.UpdateConsultationSlotRecord(c.Request.Context(), h.db, uint(id), nil, slot); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	slot.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot saved", Data: slot})
}

func (h *Controller) DeleteConsultationSlot(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot id"})
		return
	}
	if err := services.DeleteConsultationSlot(c.Request.Context(), h.db, uint(id), nil); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot deleted"})
}

func (h *Controller) ListConsultationBookings(c *gin.Context) {
	bookings, err := services.ConsultationBookingRows(c.Request.Context(), h.db, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load bookings"})
		return
	}
	services.ApplyConsultationRescheduleLimit(bookings, services.ConsultationRescheduleLimit(c.Request.Context(), h.db))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation bookings loaded", Data: bookings})
}

func (h *Controller) UpdateConsultationBooking(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid booking id"})
		return
	}
	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid booking payload"})
		return
	}
	if err := services.UpdateConsultationBookingByProvider(c.Request.Context(), h.db, uint(id), nil, strings.TrimSpace(req.Status), req.Notes); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation booking saved"})
}

func (h *Controller) validConsultationSlotOwner(c *gin.Context, ownerID uint) bool {
	if ownerID == 0 {
		return true
	}
	var owner models.User
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND (is_instructor = ? OR is_admin = ?)", ownerID, true, true).First(&owner).Error; err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Slot owner must be an instructor or admin"})
		return false
	}
	return true
}
