package models

import "time"

type StoredObject struct {
	BaseModel
	StorageProvider string     `gorm:"size:20;not null;index" json:"storage_provider"`
	Bucket          string     `gorm:"size:191" json:"bucket"`
	ObjectKey       string     `gorm:"size:700;not null;uniqueIndex" json:"object_key"`
	LegacyPath      string     `gorm:"size:700;index" json:"legacy_path"`
	OriginalName    string     `gorm:"size:255" json:"original_name"`
	MimeType        string     `gorm:"size:120" json:"mime_type"`
	SizeBytes       int64      `gorm:"not null;default:0" json:"size_bytes"`
	ChecksumSHA256  string     `gorm:"size:64;index" json:"checksum_sha256"`
	Visibility      string     `gorm:"size:20;not null;index" json:"visibility"`
	OwnerID         uint       `gorm:"not null;default:0;index:idx_object_owner" json:"owner_id"`
	OwnerScope      string     `gorm:"size:20;not null;index:idx_object_owner" json:"owner_scope"`
	ObjectClass     string     `gorm:"size:30;not null;index" json:"object_class"`
	ExpiresAt       *time.Time `gorm:"index" json:"expires_at"`
}

type StorageMigrationJob struct {
	BaseModel
	Status         string     `gorm:"size:30;not null;index" json:"status"`
	DryRun         bool       `json:"dry_run"`
	SourceProvider string     `gorm:"size:20" json:"source_provider"`
	TargetProvider string     `gorm:"size:20" json:"target_provider"`
	TotalItems     int        `json:"total_items"`
	ProcessedItems int        `json:"processed_items"`
	VerifiedItems  int        `json:"verified_items"`
	FailedItems    int        `json:"failed_items"`
	TotalBytes     int64      `json:"total_bytes"`
	ProcessedBytes int64      `json:"processed_bytes"`
	LastError      string     `gorm:"type:text" json:"last_error"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}
type StorageMigrationItem struct {
	BaseModel
	JobID          uint   `gorm:"not null;index:idx_migration_item" json:"job_id"`
	StoredObjectID uint   `gorm:"not null;index:idx_migration_item" json:"stored_object_id"`
	Status         string `gorm:"size:30;not null;index" json:"status"`
	Attempts       int    `json:"attempts"`
	ErrorMessage   string `gorm:"type:text" json:"error_message"`
	SourceChecksum string `gorm:"size:64" json:"source_checksum"`
	TargetChecksum string `gorm:"size:64" json:"target_checksum"`
}
type StorageBackup struct {
	BaseModel
	Status            string     `gorm:"size:30;not null;index" json:"status"`
	DatabaseObjectKey string     `gorm:"size:700" json:"database_object_key"`
	ManifestObjectKey string     `gorm:"size:700" json:"manifest_object_key"`
	ObjectCount       int        `json:"object_count"`
	TotalBytes        int64      `json:"total_bytes"`
	ChecksumSHA256    string     `gorm:"size:64" json:"checksum_sha256"`
	VerifiedAt        *time.Time `json:"verified_at"`
	RestoreStatus     string     `gorm:"size:30" json:"restore_status"`
	LastError         string     `gorm:"type:text" json:"last_error"`
}

type DocumentTemplate struct {
	BaseModel
	Name         string `gorm:"size:191;not null" json:"name"`
	DocumentType string `gorm:"size:30;not null;index" json:"document_type"`
	PaperSize    string `gorm:"size:20;not null" json:"paper_size"`
	Orientation  string `gorm:"size:20;not null" json:"orientation"`
	IsDefault    bool   `gorm:"not null;default:false;index" json:"is_default"`
	Status       string `gorm:"size:30;not null;index" json:"status"`
	CreatedBy    uint   `gorm:"not null;index" json:"created_by"`
}
type DocumentTemplateVersion struct {
	BaseModel
	TemplateID      uint       `gorm:"not null;index;uniqueIndex:idx_document_version" json:"template_id"`
	Version         int        `gorm:"not null;uniqueIndex:idx_document_version" json:"version"`
	DesignJSONEn    string     `gorm:"type:longtext" json:"design_json_en"`
	DesignJSONID    string     `gorm:"type:longtext" json:"design_json_id"`
	HTMLEn          string     `gorm:"type:longtext" json:"html_en"`
	HTMLID          string     `gorm:"type:longtext" json:"html_id"`
	CSS             string     `gorm:"type:longtext" json:"css"`
	VariablesJSON   string     `gorm:"type:text" json:"variables_json"`
	ChecksumSHA256  string     `gorm:"size:64;not null" json:"checksum_sha256"`
	RendererVersion string     `gorm:"size:50" json:"renderer_version"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedBy       uint       `gorm:"not null;index" json:"created_by"`
}
