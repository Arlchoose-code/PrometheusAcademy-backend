package dashboard

import (
	"net/http"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) ListNotifications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	rows, unread, err := services.NotificationInbox(c.Request.Context(), h.db, user.ID, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications loaded", Data: gin.H{"items": rows, "unread_count": unread}})
}

func (h *Controller) MarkAllNotificationsRead(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if err := services.MarkAllNotificationsRead(c.Request.Context(), h.db, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications marked as read"})
}
