package services

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

func EnsureCertificateUUID(ctx context.Context, db *gorm.DB, certificate *models.Certificate) error {
	if db == nil || certificate == nil || certificate.ID == 0 {
		return nil
	}
	if certificate.CertificateCode != nil && strings.TrimSpace(*certificate.CertificateCode) != "" {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		code, err := newCertificateCode()
		if err != nil {
			return err
		}
		var count int64
		if err := db.WithContext(ctx).Model(&models.Certificate{}).Where("certificate_code = ?", code).Count(&count).Error; err != nil || count > 0 {
			lastErr = err
			continue
		}
		updates := map[string]any{"certificate_code": code}
		if strings.TrimSpace(certificate.UUID) == "" {
			updates["uuid"] = code
		}
		if err := db.WithContext(ctx).Model(certificate).Updates(updates).Error; err != nil {
			lastErr = err
			continue
		}
		certificate.CertificateCode = &code
		if certificate.UUID == "" {
			certificate.UUID = code
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("generate certificate uuid: %w", lastErr)
	}
	return fmt.Errorf("generate certificate uuid: retries exhausted")
}

func CertificateDownloadURL(certificate models.Certificate) string {
	code := CertificateDisplayCode(certificate)
	if code == "" {
		return ""
	}
	return "/certificates/" + code
}

func CertificateFilePublicPath(certificate models.Certificate) string {
	code := CertificateDisplayCode(certificate)
	if code == "" {
		return "/uploads/certificates/certificate-pending.pdf"
	}
	return "/uploads/certificates/certificate-" + code + ".pdf"
}

func EnsureCertificatePDF(ctx context.Context, db *gorm.DB, cfg config.Config, certificate models.Certificate) (string, error) {
	publicPath := CertificateFilePublicPath(certificate)
	targetPath := StorageFilePath(cfg, publicPath)
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath, nil
	}
	var course models.Course
	if err := db.WithContext(ctx).First(&course, certificate.CourseID).Error; err != nil {
		return "", err
	}
	if err := WriteCertificatePDF(ctx, db, cfg, course, certificate.UserID, publicPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func EnsureCertificatePDFBytes(ctx context.Context, db *gorm.DB, cfg config.Config, certificate models.Certificate) ([]byte, error) {
	var version models.DocumentTemplateVersion
	if certificate.TemplateVersionID == 0 {
		_, fallbackVersion, err := SelectDocumentTemplateVersion(ctx, db, "certificate", 0)
		if err != nil {
			return nil, err
		}
		version = fallbackVersion
	} else if err := db.WithContext(ctx).First(&version, certificate.TemplateVersionID).Error; err != nil {
		return nil, err
	}
	var variables map[string]string
	if err := json.Unmarshal([]byte(certificate.SnapshotJSON), &variables); err != nil {
		var generic map[string]any
		_ = json.Unmarshal([]byte(certificate.SnapshotJSON), &generic)
		variables = mapStringAny(generic)
	}
	if len(variables) == 0 {
		variables = certificateFallbackVariables(ctx, db, certificate)
	}
	for key, value := range certificateFallbackVariables(ctx, db, certificate) {
		if strings.TrimSpace(variables[key]) == "" {
			variables[key] = value
		}
	}
	variables["certificate_number"] = CertificateDisplayCode(certificate)
	variables["document_number"] = CertificateDisplayCode(certificate)
	variables["verification_url"] = CertificateDownloadURL(certificate)
	if certificate.Locale == "id" {
		variables["course_name"] = fallbackString(variables["course_name_id"], variables["course_name"])
	} else {
		variables["course_name"] = fallbackString(variables["course_name_en"], variables["course_name"])
	}
	template := version.HTMLEn
	if certificate.Locale == "id" && strings.TrimSpace(version.HTMLID) != "" {
		template = version.HTMLID
	}
	orientation := "landscape"
	pdf, err := RenderDocumentPDF(ctx, cfg, template, variables, orientation)
	if err != nil {
		return nil, err
	}
	if cacheDays := DocumentPDFCacheDays(ctx, db); cacheDays > 0 {
		key := fmt.Sprintf("generated/certificates/%s/%d-%s.pdf", CertificateDisplayCode(certificate), version.ID, certificate.SnapshotChecksum)
		_ = StoreGeneratedPDF(ctx, db, cfg, key, pdf, CertificateFilePublicPath(certificate), cacheDays)
		expires := time.Now().Add(time.Duration(cacheDays) * 24 * time.Hour)
		_ = db.WithContext(ctx).Model(&certificate).Updates(map[string]any{"cached_object_key": key, "cache_expires_at": &expires}).Error
	}
	return pdf, nil
}

func certificateFallbackVariables(ctx context.Context, db *gorm.DB, certificate models.Certificate) map[string]string {
	var user models.User
	var course models.Course
	_ = db.WithContext(ctx).First(&user, certificate.UserID).Error
	_ = db.WithContext(ctx).First(&course, certificate.CourseID).Error
	studentName := strings.TrimSpace(user.Name)
	if studentName == "" {
		studentName = "Prometheus Learner"
	}
	courseName := strings.TrimSpace(course.TitleEn)
	if courseName == "" {
		courseName = strings.TrimSpace(course.TitleID)
	}
	if courseName == "" {
		courseName = "Prometheus Academy Course"
	}
	issuedAt := certificate.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now()
	}
	return map[string]string{
		"site_name":          "Prometheus Academy",
		"document_number":    CertificateDisplayCode(certificate),
		"recipient_name":     studentName,
		"recipient_email":    user.Email,
		"issued_at":          issuedAt.Format("2006-01-02"),
		"locale":             "en",
		"verification_url":   CertificateDownloadURL(certificate),
		"qr_verification":    CertificateDownloadURL(certificate),
		"certificate_number": CertificateDisplayCode(certificate),
		"certificate_uuid":   certificate.UUID,
		"student_name":       studentName,
		"course_name":        courseName,
		"course_name_en":     fallbackString(course.TitleEn, course.TitleID),
		"course_name_id":     fallbackString(course.TitleID, course.TitleEn),
		"instructor_name":    "Prometheus Academy",
		"completion_date":    issuedAt.Format("2006-01-02"),
		"signatory_name":     "Prometheus Academy",
		"signatory_title":    "Academic Team",
		"signature_image":    "",
	}
}

func CertificateDisplayCode(certificate models.Certificate) string {
	if certificate.CertificateCode != nil && strings.TrimSpace(*certificate.CertificateCode) != "" {
		return strings.ToUpper(strings.TrimSpace(*certificate.CertificateCode))
	}
	code := strings.TrimSpace(certificate.UUID)
	if len(code) == 36 {
		code = strings.ToUpper(strings.ReplaceAll(code, "-", ""))
		if len(code) > 12 {
			return code[:12]
		}
	}
	return strings.ToUpper(code)
}

func WriteCertificatePDF(ctx context.Context, db *gorm.DB, cfg config.Config, course models.Course, userID uint, publicPath string) error {
	targetPath := StorageFilePath(cfg, publicPath)
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create certificate directory: %w", err)
	}
	var user models.User
	name := "Prometheus Learner"
	if err := db.WithContext(ctx).First(&user, userID).Error; err == nil && strings.TrimSpace(user.Name) != "" {
		name = user.Name
	}
	courseTitle := course.TitleEn
	if strings.TrimSpace(courseTitle) == "" {
		courseTitle = course.TitleID
	}
	body := certificatePDFBody(name, courseTitle, time.Now())
	if err := os.WriteFile(targetPath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write certificate pdf: %w", err)
	}
	return nil
}

func LooksLikeUUID(value string) bool {
	return LooksLikeCertificateCode(value)
}

func LooksLikeCertificateCode(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) == 12 {
		for _, char := range value {
			if (char < '0' || char > '9') && (char < 'A' || char > 'Z') {
				return false
			}
		}
		return true
	}
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func newCertificateCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random certificate code bytes: %w", err)
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	code = strings.NewReplacer("I", "A", "O", "B").Replace(code)
	if len(code) > 12 {
		code = code[:12]
	}
	return code, nil
}

func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random uuid bytes: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}

func certificatePDFBody(name string, courseTitle string, issuedAt time.Time) string {
	lines := []string{
		"Prometheus Academy",
		"Certificate of Completion",
		name,
		"has successfully completed",
		courseTitle,
		"Issued " + issuedAt.Format("2 Jan 2006"),
	}
	content := "BT\n/F1 24 Tf\n72 730 Td\n"
	for index, line := range lines {
		size := 24
		if index == 1 {
			size = 30
		} else if index == 2 || index == 4 {
			size = 20
		} else if index > 1 {
			size = 14
		}
		if index > 0 {
			content += "0 -54 Td\n"
		}
		content += fmt.Sprintf("/F1 %d Tf\n(%s) Tj\n", size, certificatePDFEscape(line))
	}
	content += "ET\n"
	stream := fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 842 595] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj\n",
		stream,
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
	}
	pdf := "%PDF-1.4\n"
	offsets := []int{0}
	for _, obj := range objects {
		offsets = append(offsets, len(pdf))
		pdf += obj
	}
	xrefOffset := len(pdf)
	pdf += fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i < len(offsets); i++ {
		pdf += fmt.Sprintf("%010d 00000 n \n", offsets[i])
	}
	pdf += fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return pdf
}

func certificatePDFEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	value = strings.ReplaceAll(value, ")", `\)`)
	return value
}
