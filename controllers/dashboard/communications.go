package dashboard

import (
	"errors"
	"net/http"
	"strconv"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) ListCommunications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	data, err := h.communicationService.Hub(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load communication hub"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Communication hub loaded", Data: data})
}

func (h *Controller) CreateCommunication(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var input structs.CreateConversationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Course, subject, and message are required"})
		return
	}
	conversation, err := h.communicationService.CreateConversation(c.Request.Context(), user, input)
	if errors.Is(err, services.ErrCourseEnrollmentNeeded) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "You must be enrolled in this course"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to start conversation"})
		return
	}
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Conversation started", Data: gin.H{"id": conversation.ID}})
}

func (h *Controller) ListCommunicationMessages(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := communicationID(c)
	if !ok {
		return
	}
	messages, err := h.communicationService.Messages(c.Request.Context(), user, id)
	if writeCommunicationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Messages loaded", Data: messages})
}

func (h *Controller) ReplyCommunication(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := communicationID(c)
	if !ok {
		return
	}
	var input structs.CreateMessageRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Message is required"})
		return
	}
	if writeCommunicationError(c, h.communicationService.Reply(c.Request.Context(), user, id, input.Message)) {
		return
	}
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Reply sent"})
}

func communicationID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid conversation ID"})
		return 0, false
	}
	return uint(value), true
}

func writeCommunicationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, services.ErrConversationNotFound) {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Conversation not found"})
	} else if errors.Is(err, services.ErrConversationForbidden) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "You cannot access this conversation"})
	} else {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
	}
	return true
}
