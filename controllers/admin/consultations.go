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
	var slots []models.ConsultationSlot
	if err := h.db.WithContext(c.Request.Context()).Order("date asc, time_start asc").Find(&slots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load slots"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slots loaded", Data: slots})
}

func (h *Controller) CreateConsultationSlot(c *gin.Context) {
	var slot models.ConsultationSlot
	if err := c.ShouldBindJSON(&slot); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot payload"})
		return
	}
	if strings.TrimSpace(slot.TimeStart) == "" || strings.TrimSpace(slot.TimeEnd) == "" || slot.Date.IsZero() {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Date, start time, and end time are required"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&slot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create slot"})
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
	var slot models.ConsultationSlot
	if err := c.ShouldBindJSON(&slot); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot payload"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.ConsultationSlot{}).Where("id = ?", uint(id)).Select("date", "time_start", "time_end", "is_available").Updates(slot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save slot"})
		return
	}
	slot.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot saved", Data: slot})
}

func (h *Controller) DeleteConsultationSlot(c *gin.Context) {
	deleteRow[models.ConsultationSlot](h.db, "Consultation slot deleted")(c)
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
	updates := map[string]any{"notes": req.Notes}
	if strings.TrimSpace(req.Status) != "" {
		updates["status"] = strings.TrimSpace(req.Status)
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.ConsultationBooking{}).Where("id = ?", uint(id)).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save booking"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation booking saved"})
}
