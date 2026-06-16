package dashboard

import (
	"net/http"
	"strconv"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) ListConsultationSlots(c *gin.Context) {
	rows, err := services.ListConsultationSlots(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load consultation slots"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slots loaded", Data: rows})
}

func (h *Controller) ListConsultationBookings(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	bookings, err := services.ConsultationBookingRows(c.Request.Context(), h.db, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load bookings"})
		return
	}
	services.ApplyConsultationRescheduleLimit(bookings, services.ConsultationRescheduleLimit(c.Request.Context(), h.db))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation bookings loaded", Data: bookings})
}

func (h *Controller) BookConsultationSlot(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	slotID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || slotID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Slot not found"})
		return
	}
	var req structs.BookConsultationSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Successful booking-time product order is required"})
		return
	}
	booking, err := services.BookConsultationSlot(c.Request.Context(), h.db, user, uint(slotID), req.OrderID, req.Notes)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation booking confirmed", Data: booking})
}

func (h *Controller) UpdateConsultationBooking(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	bookingID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || bookingID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Booking not found"})
		return
	}
	var req structs.UpdateConsultationBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid booking payload"})
		return
	}
	if err := services.UpdateConsultationBooking(c.Request.Context(), h.db, user.ID, uint(bookingID), req.SlotID, req.Status, req.Notes); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation booking saved"})
}
