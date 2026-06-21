package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) GetStorageStatus(c *gin.Context) {
	status, err := services.GetStorageStatus(c.Request.Context(), h.db, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load storage status"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Storage status loaded", Data: status})
}

func (h *Controller) TestStorage(c *gin.Context) {
	if err := services.TestStorageConnection(c.Request.Context(), h.db, h.cfg); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Storage connection verified"})
}

func (h *Controller) SetActiveStorageProvider(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid storage provider payload"})
		return
	}
	if req.Provider != "local" && req.Provider != "r2" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Storage provider must be local or r2"})
		return
	}
	if req.Provider == "r2" {
		effective := services.EffectiveStorageConfig(c.Request.Context(), h.db, h.cfg)
		effective.StorageProvider = "r2"
		if _, _, err := services.NewConfiguredObjectStorage(c.Request.Context(), nil, effective); err != nil {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "R2 is not configured or cannot be used yet: " + err.Error()})
			return
		}
	}
	setting := models.Setting{Key: "storage_provider_active", Value: req.Provider}
	if err := h.db.WithContext(c.Request.Context()).Where(models.Setting{Key: setting.Key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save active storage provider"})
		return
	}
	status, _ := services.GetStorageStatus(c.Request.Context(), h.db, h.cfg)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Active storage provider saved", Data: status})
}

func (h *Controller) StartStorageMigration(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}
	_ = c.ShouldBindJSON(&req)
	job, err := services.StartStorageMigration(c.Request.Context(), h.db, h.cfg, req.DryRun)
	if err != nil {
		if errors.Is(err, services.ErrStorageBackupActive) {
			c.JSON(http.StatusConflict, structs.Response{Success: false, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	message := "Storage migration queued"
	if req.DryRun {
		message = fmt.Sprintf("Dry-run completed: %d files inventoried; no files were moved", job.TotalItems)
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: message, Data: job})
}

func (h *Controller) RunStorageMigrationBatch(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err := services.ProcessStorageMigrationBatch(c.Request.Context(), h.db, h.cfg, limit); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	status, _ := services.GetStorageStatus(c.Request.Context(), h.db, h.cfg)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Storage migration batch processed", Data: status})
}

func (h *Controller) PauseStorageMigration(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid migration ID"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.StorageMigrationJob{}).Where("id = ? AND status IN ?", id, []string{"queued", "running"}).Update("status", "paused").Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to pause migration"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Storage migration paused"})
}

func (h *Controller) ResumeStorageMigration(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid migration ID"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.StorageMigrationJob{}).Where("id = ? AND status = ?", id, "paused").Update("status", "queued").Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to resume migration"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Storage migration resumed"})
}

func (h *Controller) RunStorageBackup(c *gin.Context) {
	backup, err := services.QueueObjectBackup(c.Request.Context(), h.db)
	if err != nil {
		if errors.Is(err, services.ErrStorageMigrationActive) || errors.Is(err, services.ErrStorageBackupActive) {
			c.JSON(http.StatusConflict, structs.Response{Success: false, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, structs.Response{Success: true, Message: "Object backup queued and will continue in the background", Data: backup})
}

func (h *Controller) RetryStorageMigration(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid migration ID"})
		return
	}
	if err := services.RetryFailedStorageMigration(c.Request.Context(), h.db, uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Failed files queued for retry"})
}

func (h *Controller) CleanupGeneratedCache(c *gin.Context) {
	removed, err := services.CleanupExpiredGeneratedObjects(c.Request.Context(), h.db, h.cfg, 250)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Generated cache cleanup completed", Data: gin.H{"removed": removed}})
}
