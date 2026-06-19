package dashboard

import (
	"net/http"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Controller struct {
	db                   *gorm.DB
	cfg                  config.Config
	dashboardService     *services.DashboardService
	communicationService *services.CommunicationService
}

func NewController(db *gorm.DB, cfg config.Config) *Controller {
	return &Controller{
		db:                   db,
		cfg:                  cfg,
		dashboardService:     services.NewDashboardService(db),
		communicationService: services.NewCommunicationService(db),
	}
}

func (h *Controller) GetOverview(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	data, err := h.dashboardService.GetStudentDashboard(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load dashboard"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Dashboard loaded", Data: data})
}
