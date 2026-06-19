package admin

import (
	"net/http"
	"strconv"

	"academyprometheus/backend/models"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) ListCommunications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	data, err := h.communicationService.AdminHub(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load communications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Communications loaded", Data: data})
}

func (h *Controller) ListCommunicationMessages(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := adminCommunicationID(c)
	if !ok {
		return
	}
	messages, err := h.communicationService.Messages(c.Request.Context(), user, id)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Messages loaded", Data: messages})
}

func (h *Controller) ReplyCommunication(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := adminCommunicationID(c)
	if !ok {
		return
	}
	var input structs.CreateMessageRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Message is required"})
		return
	}
	if err := h.communicationService.Reply(c.Request.Context(), user, id, input.Message); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Reply sent"})
}

func (h *Controller) UpdateCommunicationStatus(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := adminCommunicationID(c)
	if !ok {
		return
	}
	var input structs.UpdateConversationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Status must be open, resolved, or closed"})
		return
	}
	if err := h.communicationService.UpdateStatus(c.Request.Context(), user, id, input.Status); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Conversation status updated"})
}

func adminCommunicationID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid conversation ID"})
		return 0, false
	}
	return uint(value), true
}
