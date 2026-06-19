package dashboard

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) AttendEvent(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	eventID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || eventID == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Event not found"})
		return
	}

	var event models.Event
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND is_active = ?", uint(eventID), true).First(&event).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Event not found"})
		return
	}

	now := time.Now().UTC()
	if now.Before(event.StartDate) {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Event check-in opens when the event starts"})
		return
	}

	attendance := models.EventAttendance{EventID: event.ID, UserID: user.ID, Status: "attended", AttendedAt: &now}
	result := h.db.WithContext(c.Request.Context()).
		Where(models.EventAttendance{EventID: event.ID, UserID: user.ID}).
		Attrs(attendance).
		FirstOrCreate(&attendance)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check in"})
		return
	}

	xpAwarded := false
	if result.RowsAffected > 0 {
		if err := services.AwardXP(c.Request.Context(), h.db, user.ID, services.XPEventEventAttended, "event", event.ID, services.XPEventAttendance, "Attended a Prometheus event", "Menghadiri event Prometheus"); err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Check-in saved, but XP could not be awarded"})
			return
		}
		xpAwarded = true
	} else if attendance.AttendedAt == nil {
		if err := h.db.WithContext(c.Request.Context()).Model(&attendance).Updates(map[string]any{"status": "attended", "attended_at": now}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update check-in"})
			return
		}
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Event check-in recorded", Data: gin.H{"attendance": attendance, "xp_awarded": xpAwarded}})
}
