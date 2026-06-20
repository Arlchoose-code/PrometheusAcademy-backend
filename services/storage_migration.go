package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

type StorageStatus struct {
	ActiveProvider     string                      `json:"active_provider"`
	LocalConfigured    bool                        `json:"local_configured"`
	R2Configured       bool                        `json:"r2_configured"`
	R2BucketConfigured bool                        `json:"r2_bucket_configured"`
	LocalObjects       int64                       `json:"local_objects"`
	R2Objects          int64                       `json:"r2_objects"`
	ProtectedObjects   int64                       `json:"protected_objects"`
	PublicObjects      int64                       `json:"public_objects"`
	GeneratedObjects   int64                       `json:"generated_objects"`
	LatestMigration    *models.StorageMigrationJob `json:"latest_migration,omitempty"`
	MigrationFailures  []StorageMigrationFailure   `json:"migration_failures,omitempty"`
	LatestBackup       *models.StorageBackup       `json:"latest_backup,omitempty"`
}

type StorageMigrationFailure struct {
	ObjectKey    string `json:"object_key"`
	OriginalName string `json:"original_name"`
	ErrorMessage string `json:"error_message"`
	Attempts     int    `json:"attempts"`
}

func GetStorageStatus(ctx context.Context, db *gorm.DB, cfg config.Config) (StorageStatus, error) {
	cfg = EffectiveStorageConfig(ctx, db, cfg)
	status := StorageStatus{ActiveProvider: fallbackString(cfg.StorageProvider, "local"), LocalConfigured: cfg.StoragePath != "", R2Configured: cfg.R2AccountID != "" && cfg.R2AccessKeyID != "" && cfg.R2SecretAccessKey != "", R2BucketConfigured: cfg.R2Bucket != ""}
	if db == nil {
		return status, nil
	}
	_ = db.WithContext(ctx).Model(&models.StoredObject{}).Where("storage_provider = ?", "local").Count(&status.LocalObjects).Error
	_ = db.WithContext(ctx).Model(&models.StoredObject{}).Where("storage_provider = ?", "r2").Count(&status.R2Objects).Error
	_ = db.WithContext(ctx).Model(&models.StoredObject{}).Where("visibility = ?", "protected").Count(&status.ProtectedObjects).Error
	_ = db.WithContext(ctx).Model(&models.StoredObject{}).Where("visibility = ?", "public").Count(&status.PublicObjects).Error
	_ = db.WithContext(ctx).Model(&models.StoredObject{}).Where("object_class = ?", "generated").Count(&status.GeneratedObjects).Error
	var job models.StorageMigrationJob
	if err := db.WithContext(ctx).Order("created_at desc").First(&job).Error; err == nil {
		status.LatestMigration = &job
		if job.FailedItems > 0 {
			_ = db.WithContext(ctx).Table("storage_migration_items AS item").
				Select("obj.object_key, obj.original_name, item.error_message, item.attempts").
				Joins("JOIN stored_objects AS obj ON obj.id = item.stored_object_id").
				Where("item.job_id = ? AND item.status = ?", job.ID, "failed").
				Order("item.id ASC").Limit(10).Scan(&status.MigrationFailures).Error
		}
	}
	var backup models.StorageBackup
	if err := db.WithContext(ctx).Order("created_at desc").First(&backup).Error; err == nil {
		status.LatestBackup = &backup
	}
	return status, nil
}

func TestStorageConnection(ctx context.Context, db *gorm.DB, cfg config.Config) error {
	storage, _, err := NewConfiguredObjectStorage(ctx, db, cfg)
	if err != nil {
		return err
	}
	return testObjectStorageConnection(ctx, storage)
}

func TestR2Connection(ctx context.Context, cfg config.Config) error {
	storage, err := NewR2Storage(ctx, cfg, cfg.R2Bucket, cfg.R2AccessKeyID, cfg.R2SecretAccessKey)
	if err != nil {
		return err
	}
	return testObjectStorageConnection(ctx, storage)
}

func testObjectStorageConnection(ctx context.Context, storage ObjectStorage) error {
	key := "generated/health/storage-test-" + time.Now().UTC().Format("20060102150405") + ".txt"
	if _, err := storage.Put(ctx, PutObjectInput{Key: key, Body: strings.NewReader("ok"), ContentType: "text/plain"}); err != nil {
		return err
	}
	ok, err := storage.Exists(ctx, key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("storage test object missing after write")
	}
	return storage.Delete(ctx, key)
}

func EnsureStoredObjectInventory(ctx context.Context, db *gorm.DB, cfg config.Config) error {
	if db == nil {
		return nil
	}
	collect := func(table, column, class, visibility string, ownerColumn string) error {
		rows, err := db.WithContext(ctx).Table(table).Select("id, " + column + " AS path" + optionalOwnerSelect(ownerColumn)).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, owner uint
			var p string
			if ownerColumn == "" {
				if err := rows.Scan(&id, &p); err != nil {
					return err
				}
			} else {
				if err := rows.Scan(&id, &p, &owner); err != nil {
					return err
				}
			}
			p = strings.TrimSpace(p)
			if p == "" || !strings.HasPrefix(p, "/uploads/") {
				continue
			}
			key := ObjectKeyFromPublicPath(p)
			full := StorageFilePath(cfg, p)
			st, err := os.Stat(full)
			if err != nil {
				continue
			}
			RegisterStoredObject(ctx, db, cfg, StoredObject{Key: key, Provider: "local", Size: st.Size()}, p, filepath.Base(p), "", visibility, ownerScopeGuess(db, owner), class, owner, nil)
		}
		return nil
	}
	for _, item := range []struct{ table, column, class, visibility, owner string }{
		{"media_files", "file_path", "media", "public", "uploaded_by"},
		{"product_files", "file_path", "product_file", "protected", ""},
		{"topic_blocks", "file_path", "course_material", "protected", ""},
		{"course_addons", "file_path", "course_addon", "protected", ""},
		{"talent_job_applications", "cv_path", "talent_cv", "protected", ""},
		{"invoices", "file_path", "generated", "protected", ""},
		{"certificates", "certificate_url", "generated", "protected", "user_id"},
	} {
		_ = collect(item.table, item.column, item.class, item.visibility, item.owner)
	}
	return nil
}

func optionalOwnerSelect(ownerColumn string) string {
	if ownerColumn == "" {
		return ""
	}
	return ", " + ownerColumn + " AS owner_id"
}
func ownerScopeGuess(db *gorm.DB, owner uint) string {
	if owner == 0 {
		return "admin"
	}
	return ownerScopeForUser(context.Background(), db, owner)
}

func StartStorageMigration(ctx context.Context, db *gorm.DB, cfg config.Config, dryRun bool) (models.StorageMigrationJob, error) {
	if !dryRun {
		if err := TestR2Connection(ctx, cfg); err != nil {
			return models.StorageMigrationJob{}, fmt.Errorf("R2 preflight failed: %w", err)
		}
	}
	if err := EnsureStoredObjectInventory(ctx, db, cfg); err != nil {
		return models.StorageMigrationJob{}, err
	}
	now := time.Now()
	job := models.StorageMigrationJob{Status: "queued", DryRun: dryRun, SourceProvider: "local", TargetProvider: "r2", StartedAt: &now}
	var objects []models.StoredObject
	if err := db.WithContext(ctx).Where("storage_provider = ?", "local").Find(&objects).Error; err != nil {
		return job, err
	}
	job.TotalItems = len(objects)
	for _, o := range objects {
		job.TotalBytes += o.SizeBytes
	}
	if dryRun {
		// A dry-run only inventories the migration plan. No background copy or
		// remote checksum verification is needed, so finish it synchronously.
		job.Status = "verified"
		job.ProcessedItems = job.TotalItems
		job.VerifiedItems = job.TotalItems
		job.CompletedAt = &now
	}
	if err := db.WithContext(ctx).Create(&job).Error; err != nil {
		return job, err
	}
	items := make([]models.StorageMigrationItem, 0, len(objects))
	for _, o := range objects {
		itemStatus := "pending"
		if dryRun {
			itemStatus = "verified"
		}
		items = append(items, models.StorageMigrationItem{JobID: job.ID, StoredObjectID: o.ID, Status: itemStatus, SourceChecksum: o.ChecksumSHA256})
	}
	if len(items) > 0 {
		if err := db.WithContext(ctx).Create(&items).Error; err != nil {
			return job, err
		}
	}
	return job, nil
}

// StartStorageMigrationWorker continuously drains queued migration batches.
// Jobs remain resumable in MySQL, so restarting the API resumes pending work.
func StartStorageMigrationWorker(ctx context.Context, db *gorm.DB, cfg config.Config) {
	run := func() {
		batchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		_ = ProcessStorageMigrationBatch(batchCtx, db, cfg, 25)
	}
	run()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			run()
		case <-ctx.Done():
			return
		}
	}
}

func ProcessStorageMigrationBatch(ctx context.Context, db *gorm.DB, cfg config.Config, limit int) error {
	if limit <= 0 {
		limit = 10
	}
	var job models.StorageMigrationJob
	if err := db.WithContext(ctx).Where("status IN ?", []string{"queued", "running"}).Order("created_at asc").First(&job).Error; err != nil {
		return nil
	}
	if job.DryRun {
		now := time.Now()
		return db.WithContext(ctx).Model(&job).Updates(map[string]any{"status": "verified", "processed_items": job.TotalItems, "verified_items": job.TotalItems, "completed_at": &now}).Error
	}
	job.Status = "running"
	_ = db.WithContext(ctx).Model(&job).Update("status", "running").Error
	target, err := NewR2Storage(ctx, cfg, cfg.R2Bucket, cfg.R2AccessKeyID, cfg.R2SecretAccessKey)
	if err != nil {
		return markMigrationFailed(ctx, db, &job, err)
	}
	local := &LocalStorage{Root: cfg.StoragePath}
	var items []models.StorageMigrationItem
	if err := db.WithContext(ctx).Where("job_id = ? AND status IN ?", job.ID, []string{"pending", "failed"}).Order("id asc").Limit(limit).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		var obj models.StoredObject
		if err := db.WithContext(ctx).First(&obj, item.StoredObjectID).Error; err != nil {
			continue
		}
		if err := db.WithContext(ctx).Model(&item).Updates(map[string]any{"status": "copying", "attempts": item.Attempts + 1}).Error; err != nil {
			return err
		}
		r, _, err := local.Open(ctx, obj.ObjectKey)
		if err != nil {
			_ = db.WithContext(ctx).Model(&item).Updates(map[string]any{"status": "failed", "error_message": err.Error()}).Error
			continue
		}
		stored, err := target.Put(ctx, PutObjectInput{Key: obj.ObjectKey, Body: r, ContentType: obj.MimeType})
		r.Close()
		if err != nil {
			_ = db.WithContext(ctx).Model(&item).Updates(map[string]any{"status": "failed", "error_message": err.Error()}).Error
			continue
		}
		checksum := stored.ChecksumSHA256
		if obj.ChecksumSHA256 != "" && checksum != obj.ChecksumSHA256 {
			_ = db.WithContext(ctx).Model(&item).Updates(map[string]any{"status": "failed", "error_message": "checksum mismatch", "target_checksum": checksum}).Error
			continue
		}
		_ = db.WithContext(ctx).Model(&obj).Updates(map[string]any{"storage_provider": "r2", "bucket": cfg.R2Bucket, "checksum_sha256": checksum}).Error
		_ = db.WithContext(ctx).Model(&item).Updates(map[string]any{"status": "verified", "target_checksum": checksum}).Error
	}
	var pending, failed, verified int64
	db.WithContext(ctx).Model(&models.StorageMigrationItem{}).Where("job_id = ? AND status IN ?", job.ID, []string{"pending", "copying"}).Count(&pending)
	db.WithContext(ctx).Model(&models.StorageMigrationItem{}).Where("job_id = ? AND status = ?", job.ID, "failed").Count(&failed)
	db.WithContext(ctx).Model(&models.StorageMigrationItem{}).Where("job_id = ? AND status = ?", job.ID, "verified").Count(&verified)
	updates := map[string]any{"processed_items": verified + failed, "verified_items": verified, "failed_items": failed}
	if pending == 0 {
		now := time.Now()
		if failed > 0 {
			updates["status"] = "failed"
			var firstFailure models.StorageMigrationItem
			if err := db.WithContext(ctx).Where("job_id = ? AND status = ?", job.ID, "failed").Order("id ASC").First(&firstFailure).Error; err == nil {
				updates["last_error"] = firstFailure.ErrorMessage
			}
		} else {
			updates["status"] = "verified"
		}
		updates["completed_at"] = &now
	}
	return db.WithContext(ctx).Model(&job).Updates(updates).Error
}

func RetryFailedStorageMigration(ctx context.Context, db *gorm.DB, jobID uint) error {
	var job models.StorageMigrationJob
	if err := db.WithContext(ctx).Where("id = ? AND status = ? AND dry_run = ?", jobID, "failed", false).First(&job).Error; err != nil {
		return fmt.Errorf("failed migration job not found")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.StorageMigrationItem{}).Where("job_id = ? AND status = ?", jobID, "failed").Updates(map[string]any{"status": "pending", "error_message": ""}).Error; err != nil {
			return err
		}
		return tx.Model(&job).Updates(map[string]any{"status": "queued", "failed_items": 0, "last_error": "", "completed_at": nil}).Error
	})
}

func markMigrationFailed(ctx context.Context, db *gorm.DB, job *models.StorageMigrationJob, err error) error {
	return db.WithContext(ctx).Model(job).Updates(map[string]any{"status": "failed", "last_error": err.Error()}).Error
}

func SHA256OfReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func CleanupExpiredGeneratedObjects(ctx context.Context, db *gorm.DB, cfg config.Config, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	var objects []models.StoredObject
	if err := db.WithContext(ctx).Where("object_class = ? AND expires_at IS NOT NULL AND expires_at < ?", "generated", time.Now()).Limit(limit).Find(&objects).Error; err != nil {
		return 0, err
	}
	storage, _, err := NewConfiguredObjectStorage(ctx, db, cfg)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, obj := range objects {
		_ = storage.Delete(ctx, obj.ObjectKey)
		if err := db.WithContext(ctx).Delete(&obj).Error; err == nil {
			removed++
		}
	}
	return removed, nil
}

func CreateObjectBackupManifest(ctx context.Context, db *gorm.DB, cfg config.Config) (models.StorageBackup, error) {
	var objects []models.StoredObject
	if err := db.WithContext(ctx).Order("object_key asc").Find(&objects).Error; err != nil {
		return models.StorageBackup{}, err
	}
	raw, _ := json.MarshalIndent(objects, "", "  ")
	sum := sha256.Sum256(raw)
	key := "backups/" + time.Now().UTC().Format("2006/01/02/150405") + "-object-manifest.json"
	backupCfg := cfg
	if cfg.BackupR2Bucket != "" {
		backupCfg.R2Bucket = cfg.BackupR2Bucket
		backupCfg.R2AccessKeyID = fallbackString(cfg.BackupR2AccessKeyID, cfg.R2AccessKeyID)
		backupCfg.R2SecretAccessKey = fallbackString(cfg.BackupR2SecretAccessKey, cfg.R2SecretAccessKey)
		backupCfg.StorageProvider = "r2"
	}
	storage, err := NewObjectStorage(ctx, backupCfg)
	if err != nil {
		return models.StorageBackup{}, err
	}
	stored, err := storage.Put(ctx, PutObjectInput{Key: key, Body: bytes.NewReader(raw), ContentType: "application/json"})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") || strings.Contains(err.Error(), "StatusCode: 401") {
			return models.StorageBackup{}, fmt.Errorf("backup R2 rejected its Access Key/Secret pair; use credentials created together, or leave both BACKUP_R2 credential fields empty to reuse the main R2 credentials")
		}
		return models.StorageBackup{}, err
	}
	var total int64
	for _, obj := range objects {
		total += obj.SizeBytes
	}
	now := time.Now()
	backup := models.StorageBackup{Status: "verified", ManifestObjectKey: stored.Key, ObjectCount: len(objects), TotalBytes: total, ChecksumSHA256: hex.EncodeToString(sum[:]), VerifiedAt: &now, RestoreStatus: "not_tested"}
	if err := db.WithContext(ctx).Create(&backup).Error; err != nil {
		return backup, err
	}
	return backup, nil
}
