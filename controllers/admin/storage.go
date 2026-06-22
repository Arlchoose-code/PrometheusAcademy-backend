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

func (h *Controller) ScanR2Objects(c *gin.Context) {
	created, err := services.EnsureR2ObjectInventory(c.Request.Context(), h.db, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	status, _ := services.GetStorageStatus(c.Request.Context(), h.db, h.cfg)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: fmt.Sprintf("R2 scan completed: %d new objects registered", created), Data: status})
}

func (h *Controller) RepairBrokenPaths(c *gin.Context) {
	results, err := services.RepairBrokenPaths(c.Request.Context(), h.db, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: err.Error()})
		return
	}
	totalFixed := 0
	for _, r := range results {
		totalFixed += r.Fixed
	}
	status, _ := services.GetStorageStatus(c.Request.Context(), h.db, h.cfg)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: fmt.Sprintf("Repair completed: %d paths fixed", totalFixed), Data: gin.H{"results": results, "status": status}})
}

func (h *Controller) DiagnoseStorage(c *gin.Context) {
	ctx := c.Request.Context()
	effective := services.EffectiveStorageConfig(ctx, h.db, h.cfg)

	diag := gin.H{
		"active_provider":  effective.StorageProvider,
		"r2_account_id":    effective.R2AccountID,
		"r2_bucket":        effective.R2Bucket,
		"r2_key_configured": effective.R2AccessKeyID != "",
		"r2_secret_configured": effective.R2SecretAccessKey != "",
		"storage_path":     effective.StoragePath,
	}

	var localCount, r2Count int64
	h.db.WithContext(ctx).Model(&models.StoredObject{}).Where("storage_provider = ?", "local").Count(&localCount)
	h.db.WithContext(ctx).Model(&models.StoredObject{}).Where("storage_provider = ?", "r2").Count(&r2Count)
	diag["stored_local_count"] = localCount
	diag["stored_r2_count"] = r2Count

	var sample []models.StoredObject
	h.db.WithContext(ctx).Order("id desc").Limit(5).Find(&sample)
	sampleKeys := make([]gin.H, 0, len(sample))
	for _, s := range sample {
		sampleKeys = append(sampleKeys, gin.H{"object_key": s.ObjectKey, "legacy_path": s.LegacyPath, "provider": s.StorageProvider, "bucket": s.Bucket})
	}
	diag["sample_stored_objects"] = sampleKeys

	if effective.R2Bucket != "" && effective.R2AccessKeyID != "" && effective.R2SecretAccessKey != "" && effective.R2AccountID != "" {
		r2, err := services.NewR2Storage(ctx, effective, effective.R2Bucket, effective.R2AccessKeyID, effective.R2SecretAccessKey)
		if err != nil {
			diag["r2_connect_error"] = err.Error()
		} else {
			objects, listErr := r2.List(ctx, "")
			if listErr != nil {
				diag["r2_list_error"] = listErr.Error()
			} else {
				diag["r2_total_objects"] = len(objects)
				r2Sample := make([]gin.H, 0, 5)
				for i, obj := range objects {
					if i >= 5 {
						break
					}
					r2Sample = append(r2Sample, gin.H{"key": obj.Key, "size": obj.Size})
				}
				diag["r2_sample_keys"] = r2Sample

				if len(sample) > 0 && len(objects) > 0 {
					testKey := sample[0].ObjectKey
					exists, _ := r2.Exists(ctx, testKey)
					diag["test_key"] = testKey
					diag["test_key_exists_in_r2"] = exists
				}
			}
		}
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Storage diagnostic", Data: diag})
}
