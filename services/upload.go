package services

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
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
	storage, err := NewObjectStorage(context.Background(), cfg)
	if err != nil {
		storage = &LocalStorage{Root: cfg.StoragePath}
	}
	return &UploadService{db: db, cfg: cfg, storage: storage}
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
	if file.Size > 50*1024*1024 {
		return "", "", fmt.Errorf("file too large")
	}

	source, err := file.Open()
	if err != nil {
		return "", "", fmt.Errorf("open upload: %w", err)
	}
	defer source.Close()

	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("%s-%d%s", prefix, time.Now().UnixNano(), ext)
	key := filepath.ToSlash(filepath.Join(relativeDir, filename))
	if _, err := s.storage.Put(context.Background(), PutObjectInput{Key: key, Body: source, ContentType: file.Header.Get("Content-Type")}); err != nil {
		return "", "", fmt.Errorf("store upload: %w", err)
	}

	publicPath := "/" + filepath.ToSlash(filepath.Join(relativeDir, filename))
	return strings.ReplaceAll(publicPath, "\\", "/"), file.Filename, nil
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
			source.Close()
			return "", fmt.Errorf("decode image upload: %w", err)
		}
	}
	if _, err := source.Seek(0, 0); err != nil {
		source.Close()
		return "", fmt.Errorf("rewind image upload: %w", err)
	}

	if originalExt == "" {
		originalExt = ".upload"
	}
	tempFile, err := os.CreateTemp("", "prometheus-upload-*"+originalExt)
	if err != nil {
		source.Close()
		return "", fmt.Errorf("create upload temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := io.Copy(tempFile, source); err != nil {
		source.Close()
		tempFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("write upload temp file: %w", err)
	}
	source.Close()
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
	target.Close()
	defer os.Remove(targetPath)

	cwebpBin := s.cfg.CWebPBin
	if cwebpBin == "" {
		cwebpBin = "cwebp"
	}

	if err := convertToWebP(cwebpBin, tempPath, targetPath); err != nil {
		return "", err
	}
	converted, err := os.Open(targetPath)
	if err != nil {
		return "", err
	}
	defer converted.Close()
	key := filepath.ToSlash(filepath.Join(relativeDir, filename))
	if _, err := s.storage.Put(context.Background(), PutObjectInput{Key: key, Body: converted, ContentType: "image/webp", CacheControl: "public, max-age=31536000, immutable"}); err != nil {
		return "", fmt.Errorf("store webp: %w", err)
	}

	publicPath := "/" + filepath.ToSlash(filepath.Join(relativeDir, filename))
	return strings.ReplaceAll(publicPath, "\\", "/"), nil
}

func convertToWebP(cwebpBin, sourcePath, targetPath string) error {
	cmd := exec.Command(cwebpBin, "-quiet", "-q", "85", sourcePath, "-o", targetPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if !errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("convert image to webp: %w%s", err, formatCommandOutput(output))
	}

	fallback := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", sourcePath, "-q:v", "80", targetPath)
	output, fallbackErr := fallback.CombinedOutput()
	if fallbackErr != nil {
		return fmt.Errorf("convert image to webp: cwebp not found and ffmpeg fallback failed: %w%s", fallbackErr, formatCommandOutput(output))
	}
	return nil
}

func formatCommandOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}
	return ": " + strings.TrimSpace(string(output))
}
