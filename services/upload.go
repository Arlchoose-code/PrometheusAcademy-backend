package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

type UploadService struct {
	db      *gorm.DB
	cfg     config.Config
	storage ObjectStorage
}

func NewUploadService(db *gorm.DB, cfg config.Config) *UploadService {
	storage, effectiveCfg, err := NewConfiguredObjectStorage(context.Background(), db, cfg)
	if err != nil {
		storage = &LocalStorage{Root: cfg.StoragePath}
		effectiveCfg = cfg
	}
	return &UploadService{db: db, cfg: effectiveCfg, storage: storage}
}

func (s *UploadService) SaveUserAvatar(ctx context.Context, userID uint, file *multipart.FileHeader) (string, error) {
	path, err := s.saveWebP(file, filepath.Join("uploads", "avatars"), "avatar")
	if err != nil {
		return "", err
	}

	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("avatar", path).Error; err != nil {
		return "", fmt.Errorf("update user avatar: %w", err)
	}

	return path, nil
}

func (s *UploadService) SaveSiteLogo(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "logos"), "logo")
}

func (s *UploadService) SaveFavicon(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "favicons"), "favicon")
}

func (s *UploadService) SaveSEOImage(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "seo"), "seo")
}

func (s *UploadService) SaveCourseThumbnail(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "courses"), "course")
}

func (s *UploadService) SaveProductThumbnail(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "products"), "product")
}

func (s *UploadService) SavePartnerLogo(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "partners"), "partner")
}

func (s *UploadService) SaveProductFile(file *multipart.FileHeader) (string, string, error) {
	return s.saveRawFile(file, filepath.Join("uploads", "product-files"), "product-file")
}

func (s *UploadService) SaveCourseAddonFile(file *multipart.FileHeader) (string, string, error) {
	return s.saveRawFile(file, filepath.Join("uploads", "course-addons"), "course-addon")
}

func (s *UploadService) SaveTalentCV(file *multipart.FileHeader) (string, string, error) {
	return s.saveRawFile(file, filepath.Join("uploads", "talent-cv"), "cv")
}

func (s *UploadService) SaveBannerImage(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "banners"), "banner")
}

func (s *UploadService) SavePageImage(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "pages"), "page")
}

func (s *UploadService) SaveTestimonialAvatar(file *multipart.FileHeader) (string, error) {
	return s.saveWebP(file, filepath.Join("uploads", "testimonials"), "testimonial")
}

func (s *UploadService) SaveMediaFile(ctx context.Context, userID uint, file *multipart.FileHeader) (models.MediaFile, error) {
	path, err := s.saveWebP(file, filepath.Join("uploads", "media"), "media")
	if err != nil {
		return models.MediaFile{}, err
	}

	media := models.MediaFile{
		FilePath:   path,
		FileName:   filepath.Base(path),
		FileType:   "image/webp",
		FileSize:   file.Size,
		UploadedBy: userID,
	}
	if err := s.db.WithContext(ctx).Create(&media).Error; err != nil {
		return models.MediaFile{}, fmt.Errorf("create media file: %w", err)
	}
	_ = s.db.WithContext(ctx).Model(&models.StoredObject{}).
		Where("legacy_path = ?", path).
		Updates(map[string]any{"owner_id": userID, "owner_scope": ownerScopeForUser(ctx, s.db, userID), "object_class": "media"}).Error

	return media, nil
}

// SaveTopicBlockFile stores an arbitrary file (pdf, zip, docx, image, etc.)
// for a topic content block without webp conversion. Returns the public path
// and the original file name.
func (s *UploadService) SaveTopicBlockFile(file *multipart.FileHeader) (string, string, error) {
	return s.saveRawFile(file, filepath.Join("uploads", "course-files"), "block")
}

func (s *UploadService) saveRawFile(file *multipart.FileHeader, relativeDir, prefix string) (string, string, error) {
	if file == nil {
		return "", "", fmt.Errorf("missing file")
	}
	limit := rawUploadLimit(relativeDir)
	if file.Size > limit {
		return "", "", fmt.Errorf("file too large")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !rawUploadExtensionAllowed(relativeDir, ext) {
		return "", "", fmt.Errorf("file type is not allowed")
	}

	source, err := file.Open()
	if err != nil {
		return "", "", fmt.Errorf("open upload: %w", err)
	}
	defer source.Close()

	header := make([]byte, 512)
	read, err := source.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", fmt.Errorf("read upload header: %w", err)
	}
	if !rawUploadContentAllowed(http.DetectContentType(header[:read])) {
		return "", "", fmt.Errorf("file content type is not allowed")
	}
	body := io.MultiReader(bytes.NewReader(header[:read]), source)
	filename := fmt.Sprintf("%s-%d%s", prefix, time.Now().UnixNano(), ext)
	key := filepath.ToSlash(filepath.Join(relativeDir, filename))
	storage, effectiveCfg := s.storageForWrite(context.Background())
	stored, err := storage.Put(context.Background(), PutObjectInput{Key: key, Body: body, ContentType: contentTypeForRawUpload(ext, file.Header.Get("Content-Type"))})
	if err != nil {
		return "", "", fmt.Errorf("store upload: %w", err)
	}

	publicPath := "/" + filepath.ToSlash(filepath.Join(relativeDir, filename))
	publicPath = strings.ReplaceAll(publicPath, "\\", "/")
	RegisterStoredObject(context.Background(), s.db, effectiveCfg, stored, publicPath, file.Filename, contentTypeForRawUpload(ext, file.Header.Get("Content-Type")), visibilityForDir(relativeDir), "admin", objectClassForDir(relativeDir), 0, nil)
	return publicPath, file.Filename, nil
}

func (s *UploadService) saveWebP(file *multipart.FileHeader, relativeDir, prefix string) (string, error) {
	if file == nil {
		return "", fmt.Errorf("missing file")
	}
	if file.Size > 5*1024*1024 {
		return "", fmt.Errorf("file too large")
	}

	source, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open upload: %w", err)
	}

	originalExt := strings.ToLower(filepath.Ext(file.Filename))
	if originalExt != ".webp" {
		if _, _, err := image.Decode(source); err != nil {
			_ = source.Close()
			return "", fmt.Errorf("decode image upload: %w", err)
		}
	} else if !isLikelyWebP(source) {
		_ = source.Close()
		return "", fmt.Errorf("decode image upload: invalid webp file")
	}
	if _, err := source.Seek(0, 0); err != nil {
		_ = source.Close()
		return "", fmt.Errorf("rewind image upload: %w", err)
	}

	if originalExt == "" {
		originalExt = ".upload"
	}
	tempFile, err := os.CreateTemp("", "prometheus-upload-*"+originalExt)
	if err != nil {
		_ = source.Close()
		return "", fmt.Errorf("create upload temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := io.Copy(tempFile, source); err != nil {
		_ = source.Close()
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("write upload temp file: %w", err)
	}
	_ = source.Close()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close upload temp file: %w", err)
	}
	defer os.Remove(tempPath)

	filename := fmt.Sprintf("%s-%d.webp", prefix, time.Now().UnixNano())
	target, err := os.CreateTemp("", "prometheus-webp-*.webp")
	if err != nil {
		return "", fmt.Errorf("create webp target: %w", err)
	}
	targetPath := target.Name()
	if err := target.Close(); err != nil {
		return "", fmt.Errorf("close webp target: %w", err)
	}
	defer os.Remove(targetPath)

	cwebpBin, err := safeToolBinary(s.cfg.CWebPBin, "cwebp")
	if err != nil {
		return "", err
	}

	if err := convertToWebP(cwebpBin, tempPath, targetPath); err != nil {
		return "", err
	}
	// #nosec G304 - targetPath is created by os.CreateTemp in this function.
	converted, err := os.Open(targetPath)
	if err != nil {
		return "", err
	}
	defer converted.Close()
	key := filepath.ToSlash(filepath.Join(relativeDir, filename))
	storage, effectiveCfg := s.storageForWrite(context.Background())
	stored, err := storage.Put(context.Background(), PutObjectInput{Key: key, Body: converted, ContentType: "image/webp", CacheControl: "public, max-age=31536000, immutable"})
	if err != nil {
		return "", fmt.Errorf("store webp: %w", err)
	}

	publicPath := "/" + filepath.ToSlash(filepath.Join(relativeDir, filename))
	publicPath = strings.ReplaceAll(publicPath, "\\", "/")
	RegisterStoredObject(context.Background(), s.db, effectiveCfg, stored, publicPath, file.Filename, "image/webp", "public", "admin", objectClassForDir(relativeDir), 0, nil)
	return publicPath, nil
}

func (s *UploadService) storageForWrite(ctx context.Context) (ObjectStorage, config.Config) {
	storage, effectiveCfg, err := NewConfiguredObjectStorage(ctx, s.db, s.cfg)
	if err != nil {
		return s.storage, s.cfg
	}
	return storage, effectiveCfg
}

func visibilityForDir(relativeDir string) string {
	dir := filepath.ToSlash(relativeDir)
	if strings.Contains(dir, "product-files") || strings.Contains(dir, "course-files") || strings.Contains(dir, "course-addons") || strings.Contains(dir, "talent-cv") || strings.Contains(dir, "invoices") || strings.Contains(dir, "certificates") {
		return "protected"
	}
	return "public"
}

func objectClassForDir(relativeDir string) string {
	dir := filepath.ToSlash(relativeDir)
	switch {
	case strings.Contains(dir, "product-files"):
		return "product_file"
	case strings.Contains(dir, "course-files"):
		return "course_material"
	case strings.Contains(dir, "course-addons"):
		return "course_addon"
	case strings.Contains(dir, "talent-cv"):
		return "talent_cv"
	case strings.Contains(dir, "media"):
		return "media"
	default:
		return "public"
	}
}

func rawUploadLimit(relativeDir string) int64 {
	dir := filepath.ToSlash(relativeDir)
	if strings.Contains(dir, "talent-cv") {
		return 10 * 1024 * 1024
	}
	return 50 * 1024 * 1024
}

func rawUploadExtensionAllowed(relativeDir string, ext string) bool {
	dir := filepath.ToSlash(relativeDir)
	if strings.Contains(dir, "talent-cv") {
		return ext == ".pdf" || ext == ".doc" || ext == ".docx"
	}
	switch ext {
	case ".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx", ".csv", ".txt", ".zip":
		return true
	default:
		return false
	}
}

func rawUploadContentAllowed(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch contentType {
	case "text/html", "image/svg+xml", "application/javascript", "text/javascript", "application/x-msdownload":
		return false
	default:
		return true
	}
}

func contentTypeForRawUpload(ext string, fallback string) string {
	switch strings.ToLower(ext) {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	case ".zip":
		return "application/zip"
	default:
		if strings.TrimSpace(fallback) == "" {
			return "application/octet-stream"
		}
		return fallback
	}
}

func isLikelyWebP(file multipart.File) bool {
	header := make([]byte, 12)
	n, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	if seeker, ok := file.(io.Seeker); ok {
		_, _ = seeker.Seek(0, 0)
	}
	return n == 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP"
}

func ownerScopeForUser(ctx context.Context, db *gorm.DB, userID uint) string {
	if db == nil || userID == 0 {
		return "admin"
	}
	var user models.User
	if err := db.WithContext(ctx).Select("id", "is_admin", "is_instructor").First(&user, userID).Error; err != nil {
		return "user"
	}
	if user.IsAdmin {
		return "admin"
	}
	if user.IsInstructor {
		return "instructor"
	}
	return "user"
}

func convertToWebP(cwebpBin, sourcePath, targetPath string) error {
	// #nosec G204 - binary is validated by safeToolBinary and paths are server-created temp files.
	cmd := exec.Command(cwebpBin, "-quiet", "-q", "85", sourcePath, "-o", targetPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if !errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("convert image to webp: %w%s", err, formatCommandOutput(output))
	}

	// #nosec G204 - fallback binary is constant and paths are server-created temp files.
	fallback := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", sourcePath, "-q:v", "80", targetPath)
	output, fallbackErr := fallback.CombinedOutput()
	if fallbackErr != nil {
		return fmt.Errorf("convert image to webp: cwebp not found and ffmpeg fallback failed: %w%s", fallbackErr, formatCommandOutput(output))
	}
	return nil
}

func safeToolBinary(value string, fallback string) (string, error) {
	binary := strings.TrimSpace(value)
	if binary == "" {
		binary = fallback
	}
	if strings.ContainsAny(binary, "\x00\r\n") || strings.ContainsAny(binary, `"';&|<>`) {
		return "", fmt.Errorf("invalid converter binary path")
	}
	if strings.ContainsAny(binary, `\/`) {
		clean := filepath.Clean(binary)
		if clean != binary {
			return "", fmt.Errorf("invalid converter binary path")
		}
		if _, err := os.Stat(clean); err != nil {
			return "", fmt.Errorf("converter binary is not available")
		}
		return clean, nil
	}
	if path, err := exec.LookPath(binary); err == nil {
		return path, nil
	}
	return binary, nil
}

func formatCommandOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}
	return ": " + strings.TrimSpace(string(output))
}
