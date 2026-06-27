package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

const DocumentRendererVersion = "internal-pdf-v1"

var variablePattern = regexp.MustCompile(`\{\{?\s*([a-zA-Z0-9_]+)\s*\}?\}`)

func EnsureDefaultDocumentTemplate(ctx context.Context, db *gorm.DB, docType string, userID uint) (models.DocumentTemplate, models.DocumentTemplateVersion, error) {
	var tmpl models.DocumentTemplate
	if err := db.WithContext(ctx).Where("document_type = ? AND is_default = ?", docType, true).First(&tmpl).Error; err == nil {
		var version models.DocumentTemplateVersion
		if err := db.WithContext(ctx).Where("template_id = ?", tmpl.ID).Order("version desc").First(&version).Error; err != nil {
			return tmpl, version, err
		}
		if strings.TrimSpace(version.DesignJSONEn) == "" || strings.TrimSpace(version.DesignJSONEn) == "[]" || !strings.Contains(version.HTMLEn, "<html") || defaultDocumentTemplateNeedsUpgrade(docType, version) {
			upgraded, err := createDefaultDocumentTemplateVersion(ctx, db, tmpl, docType, version.Version+1, userID)
			if err != nil {
				return tmpl, upgraded, err
			}
			return tmpl, upgraded, nil
		}
		return tmpl, version, nil
	}
	orientation := "portrait"
	if docType == "certificate" {
		orientation = "landscape"
	}
	tmpl = models.DocumentTemplate{Name: defaultDocumentTemplateName(docType), DocumentType: docType, PaperSize: "A4", Orientation: orientation, IsDefault: true, Status: "published", CreatedBy: userID}
	if err := db.WithContext(ctx).Create(&tmpl).Error; err != nil {
		return tmpl, models.DocumentTemplateVersion{}, err
	}
	version, err := createDefaultDocumentTemplateVersion(ctx, db, tmpl, docType, 1, userID)
	return tmpl, version, err
}

func defaultDocumentTemplateNeedsUpgrade(docType string, version models.DocumentTemplateVersion) bool {
	if docType == "certificate" {
		return strings.Contains(version.HTMLEn, "{certificate_uuid}") || strings.Contains(version.HTMLID, "{certificate_uuid}") || strings.Contains(version.HTMLEn, "border-top:1px solid #C9A84C") || !strings.Contains(version.HTMLEn, "width:297mm")
	}
	return !strings.Contains(version.HTMLEn, ".invoice:before")
}

func createDefaultDocumentTemplateVersion(ctx context.Context, db *gorm.DB, tmpl models.DocumentTemplate, docType string, versionNumber int, userID uint) (models.DocumentTemplateVersion, error) {
	htmlEN := defaultDocumentHTML(docType, "en")
	htmlID := defaultDocumentHTML(docType, "id")
	vars := allowedVariablesJSON(docType)
	designEN := documentUnlayerDesignJSON(htmlEN, docType)
	designID := documentUnlayerDesignJSON(htmlID, docType)
	checksum := DocumentTemplateChecksum(htmlEN, htmlID, "", designEN, designID)
	now := time.Now()
	version := models.DocumentTemplateVersion{TemplateID: tmpl.ID, Version: versionNumber, DesignJSONEn: designEN, DesignJSONID: designID, HTMLEn: htmlEN, HTMLID: htmlID, CSS: "", VariablesJSON: vars, ChecksumSHA256: checksum, RendererVersion: DocumentRendererVersion, PublishedAt: &now, CreatedBy: userID}
	if err := db.WithContext(ctx).Create(&version).Error; err != nil {
		return version, err
	}
	_ = db.WithContext(ctx).Model(&tmpl).Updates(map[string]any{"name": defaultDocumentTemplateName(docType), "status": "published"}).Error
	return version, nil
}

func SelectDocumentTemplateVersion(ctx context.Context, db *gorm.DB, docType string, userID uint) (models.DocumentTemplate, models.DocumentTemplateVersion, error) {
	settingKey := "document_template_" + docType
	var setting models.Setting
	if err := db.WithContext(ctx).Where("`key` = ?", settingKey).First(&setting).Error; err == nil && strings.TrimSpace(setting.Value) != "" {
		var tmpl models.DocumentTemplate
		if err := db.WithContext(ctx).Where("id = ? AND document_type = ?", strings.TrimSpace(setting.Value), docType).First(&tmpl).Error; err == nil {
			var version models.DocumentTemplateVersion
			if err := db.WithContext(ctx).Where("template_id = ?", tmpl.ID).Order("version desc").First(&version).Error; err == nil {
				return tmpl, version, nil
			}
		}
	}
	return EnsureDefaultDocumentTemplate(ctx, db, docType, userID)
}

func defaultDocumentTemplateName(docType string) string {
	if docType == "certificate" {
		return "Prometheus Certificate - Elegant Gold"
	}
	return "Prometheus Invoice - Premium Clean"
}

func defaultDocumentHTML(docType string, locale string) string {
	if docType == "certificate" {
		if locale == "id" {
			return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    @page { size: A4 landscape; margin: 0; }
    * { box-sizing:border-box; }
    html, body { width:297mm; height:210mm; margin:0; overflow:hidden; font-family: Arial, sans-serif; color:#0D1B2E; background:#FFFDF8; }
    .certificate { width:297mm; height:210mm; border: 3mm solid #C9A84C; background: linear-gradient(135deg,#FFFFFF 0%,#FFF8E6 100%); padding: 16mm 22mm; position: relative; overflow:hidden; display:flex; align-items:center; justify-content:center; }
    .certificate:before { content:""; position:absolute; inset:7mm; border:0.4mm solid rgba(201,168,76,.45); }
    .watermark { position:absolute; right:-40px; bottom:-40px; font-size:130px; font-weight:800; color:rgba(201,168,76,.10); letter-spacing:-8px; }
    .eyebrow { letter-spacing:4px; color:#C9A84C; font-weight:800; text-transform:uppercase; font-size:13px; }
    h1 { margin:24px 0 8px; font-size:46px; line-height:1.05; color:#0D1B2E; }
    .lead { margin:0; color:#6C757D; font-size:16px; }
    .student { margin:34px 0 12px; font-size:42px; font-weight:800; color:#1A3256; }
    .course { margin:12px auto 24px; max-width:760px; font-size:24px; font-weight:700; color:#0D1B2E; }
    .meta { display:flex; justify-content:space-between; gap:14mm; margin-top:12mm; font-size:13px; color:#343A40; }
    .line { border-bottom:1px solid #C9A84C; padding-bottom:3mm; min-width:58mm; }
    .verify { margin-top:18px; font-size:11px; color:#6C757D; }
  </style>
</head>
<body>
  <main class="certificate">
    <div class="watermark">P</div>
    <div style="position:relative; text-align:center;">
      <div class="eyebrow">{site_name}</div>
      <h1>Sertifikat Penyelesaian</h1>
      <p class="lead">Sertifikat ini diberikan kepada</p>
      <div class="student">{student_name}</div>
      <p class="lead">karena telah menyelesaikan</p>
      <div class="course">{course_name}</div>
      <div class="meta">
        <div class="line"><strong>Diterbitkan</strong><br>{issued_at}</div>
        <div class="line"><strong>Instruktur</strong><br>{instructor_name}</div>
        <div class="line"><strong>{signatory_name}</strong><br>{signatory_title}</div>
      </div>
      <div class="verify">ID Sertifikat: {certificate_number} - Verifikasi: {verification_url}</div>
    </div>
  </main>
</body>
</html>`
		}
		return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    @page { size: A4 landscape; margin: 0; }
    * { box-sizing:border-box; }
    html, body { width:297mm; height:210mm; margin:0; overflow:hidden; font-family: Arial, sans-serif; color:#0D1B2E; background:#FFFDF8; }
    .certificate { width:297mm; height:210mm; border: 3mm solid #C9A84C; background: linear-gradient(135deg,#FFFFFF 0%,#FFF8E6 100%); padding: 16mm 22mm; position: relative; overflow:hidden; display:flex; align-items:center; justify-content:center; }
    .certificate:before { content:""; position:absolute; inset:7mm; border:0.4mm solid rgba(201,168,76,.45); }
    .watermark { position:absolute; right:-40px; bottom:-40px; font-size:130px; font-weight:800; color:rgba(201,168,76,.10); letter-spacing:-8px; }
    .eyebrow { letter-spacing:4px; color:#C9A84C; font-weight:800; text-transform:uppercase; font-size:13px; }
    h1 { margin:24px 0 8px; font-size:46px; line-height:1.05; color:#0D1B2E; }
    .lead { margin:0; color:#6C757D; font-size:16px; }
    .student { margin:34px 0 12px; font-size:42px; font-weight:800; color:#1A3256; }
    .course { margin:12px auto 24px; max-width:760px; font-size:24px; font-weight:700; color:#0D1B2E; }
    .meta { display:flex; justify-content:space-between; gap:14mm; margin-top:12mm; font-size:13px; color:#343A40; }
    .line { border-bottom:1px solid #C9A84C; padding-bottom:3mm; min-width:58mm; }
    .verify { margin-top:18px; font-size:11px; color:#6C757D; }
  </style>
</head>
<body>
  <main class="certificate">
    <div class="watermark">P</div>
    <div style="position:relative; text-align:center;">
      <div class="eyebrow">{site_name}</div>
      <h1>Certificate of Completion</h1>
      <p class="lead">This certificate is proudly presented to</p>
      <div class="student">{student_name}</div>
      <p class="lead">for successfully completing</p>
      <div class="course">{course_name}</div>
      <div class="meta">
        <div class="line"><strong>Issued</strong><br>{issued_at}</div>
        <div class="line"><strong>Instructor</strong><br>{instructor_name}</div>
        <div class="line"><strong>{signatory_name}</strong><br>{signatory_title}</div>
      </div>
      <div class="verify">Certificate ID: {certificate_number} - Verify: {verification_url}</div>
    </div>
  </main>
</body>
</html>`
	}
	if locale == "id" {
		return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    @page { size: A4 portrait; margin: 0; }
    * { box-sizing:border-box; }
    html, body { width:210mm; height:297mm; margin:0; overflow:hidden; font-family: Arial, sans-serif; color:#212529; background:#FFFFFF; }
    .invoice { width:210mm; height:297mm; background:#FFFFFF; overflow:hidden; display:flex; flex-direction:column; position:relative; }
    .invoice:before { content:""; position:absolute; inset:0 auto 0 0; width:4mm; background:#C9A84C; z-index:3; }
    .header { min-height:72mm; background:linear-gradient(135deg,#0D1B2E 0%,#1A3256 100%); color:#FFFFFF; padding:18mm 17mm 14mm 21mm; display:flex; justify-content:space-between; gap:18mm; align-items:flex-start; }
    .brand { color:#E8C96D; font-size:12px; letter-spacing:3px; text-transform:uppercase; font-weight:800; }
    h1 { margin:12mm 0 0; font-size:38px; line-height:1.05; letter-spacing:-1px; }
    .pill { display:inline-block; border:1px solid rgba(255,255,255,.18); border-radius:999px; padding:8px 12px; color:#E8C96D; font-size:12px; }
    .body { padding:14mm 17mm 10mm 21mm; flex:1; }
    .grid { display:grid; grid-template-columns: 1.25fr .75fr; gap:7mm; margin-bottom:11mm; }
    .label { color:#6C757D; font-size:11px; letter-spacing:1.6px; text-transform:uppercase; font-weight:700; margin-bottom:8px; }
    .box { border:1px solid #E9ECEF; border-radius:4mm; padding:6mm; background:#F8F9FA; min-height:29mm; }
    .items { border:1px solid #E9ECEF; border-left:1.5mm solid #C9A84C; border-radius:3mm; padding:7mm; margin:5mm 0 9mm; white-space:pre-line; font-size:15px; line-height:1.7; }
    .totals { margin-left:auto; width:82mm; border:1px solid #E9ECEF; border-radius:4mm; overflow:hidden; }
    .row { display:flex; justify-content:space-between; padding:12px 16px; border-bottom:1px solid #E9ECEF; }
    .row.total { background:#FEF3D0; color:#0D1B2E; font-size:18px; font-weight:800; border-bottom:0; }
    .footer { margin-top:auto; padding:7mm 17mm 7mm 21mm; color:#6C757D; font-size:11px; border-top:1px solid #E9ECEF; display:flex; justify-content:space-between; }
  </style>
</head>
<body>
  <main class="invoice">
    <section class="header">
      <div>
        <div class="brand">{site_name}</div>
        <h1>Invoice {invoice_number}</h1>
      </div>
      <div style="text-align:right;">
        <span class="pill">Lunas - {paid_at}</span>
        <p style="margin:16px 0 0; color:rgba(255,255,255,.72);">Referensi<br><strong style="color:#fff;">{payment_reference}</strong></p>
      </div>
    </section>
    <section class="body">
      <div class="grid">
        <div class="box">
          <div class="label">Ditagihkan kepada</div>
          <strong>{customer_name}</strong><br>
          <span>{customer_email}</span>
        </div>
        <div class="box">
          <div class="label">Pembayaran</div>
          <strong>{payment_method}</strong><br>
          <span>{currency}</span>
        </div>
      </div>
      <div class="label">Item</div>
      <div class="items">{items_table}</div>
      <div class="totals">
        <div class="row"><span>Subtotal</span><strong>{subtotal}</strong></div>
        <div class="row"><span>Diskon</span><strong>{discount}</strong></div>
        <div class="row total"><span>Total</span><span>{total}</span></div>
      </div>
    </section>
    <section class="footer">Dibuat oleh {site_name}. Simpan invoice ini untuk arsip kamu.</section>
  </main>
</body>
</html>`
	}
	return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    @page { size: A4 portrait; margin: 0; }
    * { box-sizing:border-box; }
    html, body { width:210mm; height:297mm; margin:0; overflow:hidden; font-family: Arial, sans-serif; color:#212529; background:#FFFFFF; }
    .invoice { width:210mm; height:297mm; background:#FFFFFF; overflow:hidden; display:flex; flex-direction:column; position:relative; }
    .invoice:before { content:""; position:absolute; inset:0 auto 0 0; width:4mm; background:#C9A84C; z-index:3; }
    .header { min-height:72mm; background:linear-gradient(135deg,#0D1B2E 0%,#1A3256 100%); color:#FFFFFF; padding:18mm 17mm 14mm 21mm; display:flex; justify-content:space-between; gap:18mm; align-items:flex-start; }
    .brand { color:#E8C96D; font-size:12px; letter-spacing:3px; text-transform:uppercase; font-weight:800; }
    h1 { margin:12mm 0 0; font-size:38px; line-height:1.05; letter-spacing:-1px; }
    .pill { display:inline-block; border:1px solid rgba(255,255,255,.18); border-radius:999px; padding:8px 12px; color:#E8C96D; font-size:12px; }
    .body { padding:14mm 17mm 10mm 21mm; flex:1; }
    .grid { display:grid; grid-template-columns: 1.25fr .75fr; gap:7mm; margin-bottom:11mm; }
    .label { color:#6C757D; font-size:11px; letter-spacing:1.6px; text-transform:uppercase; font-weight:700; margin-bottom:8px; }
    .box { border:1px solid #E9ECEF; border-radius:4mm; padding:6mm; background:#F8F9FA; min-height:29mm; }
    .items { border:1px solid #E9ECEF; border-left:1.5mm solid #C9A84C; border-radius:3mm; padding:7mm; margin:5mm 0 9mm; white-space:pre-line; font-size:15px; line-height:1.7; }
    .totals { margin-left:auto; width:82mm; border:1px solid #E9ECEF; border-radius:4mm; overflow:hidden; }
    .row { display:flex; justify-content:space-between; padding:12px 16px; border-bottom:1px solid #E9ECEF; }
    .row.total { background:#FEF3D0; color:#0D1B2E; font-size:18px; font-weight:800; border-bottom:0; }
    .footer { margin-top:auto; padding:7mm 17mm 7mm 21mm; color:#6C757D; font-size:11px; border-top:1px solid #E9ECEF; display:flex; justify-content:space-between; }
  </style>
</head>
<body>
  <main class="invoice">
    <section class="header">
      <div>
        <div class="brand">{site_name}</div>
        <h1>Invoice {invoice_number}</h1>
      </div>
      <div style="text-align:right;">
        <span class="pill">Paid - {paid_at}</span>
        <p style="margin:16px 0 0; color:rgba(255,255,255,.72);">Reference<br><strong style="color:#fff;">{payment_reference}</strong></p>
      </div>
    </section>
    <section class="body">
      <div class="grid">
        <div class="box">
          <div class="label">Billed to</div>
          <strong>{customer_name}</strong><br>
          <span>{customer_email}</span>
        </div>
        <div class="box">
          <div class="label">Payment</div>
          <strong>{payment_method}</strong><br>
          <span>{currency}</span>
        </div>
      </div>
      <div class="label">Items</div>
      <div class="items">{items_table}</div>
      <div class="totals">
        <div class="row"><span>Subtotal</span><strong>{subtotal}</strong></div>
        <div class="row"><span>Discount</span><strong>{discount}</strong></div>
        <div class="row total"><span>Total</span><span>{total}</span></div>
      </div>
    </section>
    <section class="footer">Generated by {site_name}. Keep this invoice for your records.</section>
  </main>
</body>
</html>`
}

func documentUnlayerDesignJSON(html string, docType string) string {
	width := "720px"
	if docType == "certificate" {
		width = "1000px"
	}
	raw, _ := json.Marshal(map[string]any{
		"schemaVersion": 21,
		"counters":      map[string]int{"u_row": 1, "u_column": 1, "u_content_html": 1},
		"body": map[string]any{
			"rows": []any{map[string]any{
				"cells": []int{1},
				"columns": []any{map[string]any{
					"contents": []any{map[string]any{
						"type":   "html",
						"values": map[string]any{"html": html, "hideDesktop": false},
					}},
					"values": map[string]any{"backgroundColor": "#FFFFFF", "padding": "0px"},
				}},
				"values": map[string]any{"backgroundColor": "#FFFFFF", "padding": "0px", "columns": false},
			}},
			"values": map[string]any{
				"backgroundColor": "#F8F9FA",
				"contentWidth":    width,
				"fontFamily":      map[string]string{"label": "Arial", "value": "arial,helvetica,sans-serif"},
			},
		},
	})
	return string(raw)
}

func allowedVariablesJSON(docType string) string {
	vars := []string{"site_name", "logo_url", "document_number", "recipient_name", "recipient_email", "issued_at", "locale", "verification_url", "qr_verification"}
	if docType == "certificate" {
		vars = append(vars, "certificate_number", "certificate_uuid", "student_name", "course_name", "instructor_name", "completion_date", "signatory_name", "signatory_title", "signature_image")
	} else {
		vars = append(vars, "invoice_number", "customer_name", "customer_email", "items_table", "subtotal", "discount", "total", "currency", "paid_at", "payment_reference", "payment_method")
	}
	raw, _ := json.Marshal(vars)
	return string(raw)
}

func AllowedDocumentVariables(docType string) string { return allowedVariablesJSON(docType) }

func SampleDocumentVariables(docType string) map[string]string {
	var keys []string
	_ = json.Unmarshal([]byte(allowedVariablesJSON(docType)), &keys)
	out := map[string]string{}
	for _, key := range keys {
		out[key] = "Sample"
	}
	out["site_name"] = "Prometheus Academy"
	out["invoice_number"] = "INV-000001"
	out["customer_name"] = "Jane Doe"
	out["customer_email"] = "jane@example.com"
	out["items_table"] = "Course #1 - Rp 750000"
	out["subtotal"] = "Rp 750000"
	out["discount"] = "Rp 0"
	out["total"] = "Rp 750000"
	out["currency"] = "IDR"
	out["paid_at"] = "2026-06-20"
	out["payment_reference"] = "ORDER-1"
	out["payment_method"] = "Midtrans"
	out["student_name"] = "Jane Doe"
	out["course_name"] = "UI/UX Design Masterclass"
	out["certificate_uuid"] = "DKBYLL0WTO2F"
	out["certificate_number"] = "DKBYLL0WTO2F"
	out["completion_date"] = "2026-06-20"
	out["instructor_name"] = "Prometheus Academy"
	out["verification_url"] = "https://example.com/certificates/sample"
	out["issued_at"] = "2026-06-20"
	out["locale"] = "en"
	out["document_number"] = "DOC-1"
	out["recipient_name"] = "Jane Doe"
	out["recipient_email"] = "jane@example.com"
	out["signatory_name"] = "Prometheus Academy"
	out["signatory_title"] = "Academic Team"
	return out
}

func DocumentTemplateChecksum(parts ...string) string {
	return checksumString(strings.Join(parts, "|") + "|" + DocumentRendererVersion)
}

func ValidateDocumentVariables(template string, variables map[string]string) error {
	unknown := make([]string, 0)
	seen := map[string]bool{}
	for _, match := range variablePattern.FindAllStringSubmatch(template, -1) {
		key := match[1]
		if _, ok := variables[key]; !ok && !seen[key] {
			unknown = append(unknown, key)
			seen[key] = true
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unknown document variables: %s", strings.Join(unknown, ", "))
	}
	return nil
}

func RenderDocumentPDF(ctx context.Context, cfg config.Config, template string, variables map[string]string, orientation string) ([]byte, error) {
	if err := ValidateDocumentVariables(template, variables); err != nil {
		return nil, err
	}
	rendered := variablePattern.ReplaceAllStringFunc(template, func(token string) string {
		m := variablePattern.FindStringSubmatch(token)
		if len(m) == 2 {
			return variables[m[1]]
		}
		return ""
	})
	if variablePattern.MatchString(rendered) {
		return nil, fmt.Errorf("unresolved document variable")
	}
	if looksLikeHTML(rendered) {
		if pdf, err := renderHTMLPDFWithChromium(ctx, cfg, rendered, orientation); err == nil {
			return pdf, nil
		}
	}
	text := rendered
	text = htmlToDocumentText(text)
	lines := strings.Split(text, "\n")
	media := "[0 0 612 792]"
	if orientation == "landscape" {
		media = "[0 0 842 595]"
	}
	content := "BT\n/F1 18 Tf\n72 730 Td\n"
	if orientation == "landscape" {
		content = "BT\n/F1 20 Tf\n72 520 Td\n"
	}
	for i, line := range lines {
		if i > 0 {
			content += "0 -30 Td\n"
		}
		size := 12
		if i < 2 {
			size = 22
		}
		content += fmt.Sprintf("/F1 %d Tf\n(%s) Tj\n", size, pdfTextEscape(line))
	}
	content += "ET\n"
	return []byte(rawPDF(media, content)), nil
}

func looksLikeHTML(value string) bool {
	return regexp.MustCompile(`(?is)<html|<!doctype|<body|<main|<section|<div`).MatchString(value)
}

func renderHTMLPDFWithChromium(ctx context.Context, cfg config.Config, html string, orientation string) ([]byte, error) {
	binary := findChromiumBinary(cfg)
	if binary == "" {
		return nil, fmt.Errorf("chromium executable not configured")
	}
	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	htmlFile, err := os.CreateTemp("", "prometheus-document-*.html")
	if err != nil {
		return nil, err
	}
	htmlPath := htmlFile.Name()
	defer os.Remove(htmlPath)
	if _, err := htmlFile.WriteString(html); err != nil {
		_ = htmlFile.Close()
		return nil, err
	}
	if err := htmlFile.Close(); err != nil {
		return nil, err
	}
	pdfFile, err := os.CreateTemp("", "prometheus-document-*.pdf")
	if err != nil {
		return nil, err
	}
	pdfPath := pdfFile.Name()
	if err := pdfFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(pdfPath)
	absHTML, _ := filepath.Abs(htmlPath)
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absHTML)}).String()
	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--run-all-compositor-stages-before-draw",
		"--print-to-pdf=" + pdfPath,
		"--print-to-pdf-no-header",
		"--no-pdf-header-footer",
		fileURL,
	}
	if orientation == "landscape" {
		args = append([]string{"--landscape"}, args...)
	}
	// #nosec G204 - binary comes from findChromiumBinary candidates/stat/LookPath and args use server-created temp files.
	output, err := exec.CommandContext(renderCtx, binary, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("chromium pdf render failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	// #nosec G304 - pdfPath is created by os.CreateTemp in this function.
	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		return nil, fmt.Errorf("chromium did not produce a valid PDF")
	}
	return pdf, nil
}

func findChromiumBinary(cfg config.Config) string {
	candidates := []string{strings.TrimSpace(cfg.ChromiumPath)}
	candidates = append(candidates,
		"chrome",
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"msedge",
		"brave",
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
	)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.ContainsAny(candidate, `\/`) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func htmlToDocumentText(value string) string {
	value = regexp.MustCompile(`(?is)<style.*?</style>`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`(?is)<script.*?</script>`).ReplaceAllString(value, "")
	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
		"</section>", "\n",
		"</h1>", "\n",
		"</h2>", "\n",
		"</h3>", "\n",
		"</li>", "\n",
		"</tr>", "\n",
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"·", "-",
	)
	value = replacer.Replace(value)
	value = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(value, "\n")
	value = regexp.MustCompile(`\n{3,}`).ReplaceAllString(value, "\n\n")
	return strings.TrimSpace(value)
}

func rawPDF(mediaBox, content string) string {
	stream := fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox %s /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj\n", mediaBox),
		stream,
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
	}
	pdf := "%PDF-1.4\n"
	offsets := []int{0}
	for _, obj := range objects {
		offsets = append(offsets, len(pdf))
		pdf += obj
	}
	xref := len(pdf)
	pdf += fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i < len(offsets); i++ {
		pdf += fmt.Sprintf("%010d 00000 n \n", offsets[i])
	}
	return pdf + fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
}

func pdfTextEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	value = strings.ReplaceAll(value, ")", `\)`)
	return value
}

func StoreGeneratedPDF(ctx context.Context, db *gorm.DB, cfg config.Config, key string, pdf []byte, publicPath string, cacheDays int) error {
	if cacheDays <= 0 {
		return nil
	}
	storage, effectiveCfg, err := NewConfiguredObjectStorage(ctx, db, cfg)
	if err != nil {
		return err
	}
	expires := time.Now().Add(time.Duration(cacheDays) * 24 * time.Hour)
	stored, err := storage.Put(ctx, PutObjectInput{Key: key, Body: bytes.NewReader(pdf), ContentType: "application/pdf", CacheControl: "private, max-age=0"})
	if err != nil {
		return err
	}
	RegisterStoredObject(ctx, db, effectiveCfg, stored, publicPath, filepathBase(key), "application/pdf", "protected", "admin", "generated", 0, &expires)
	return nil
}

func DocumentPDFCacheDays(ctx context.Context, db *gorm.DB) int {
	if db == nil {
		return 0
	}
	var setting models.Setting
	if err := db.WithContext(ctx).Where("`key` = ?", "document_pdf_cache_days").First(&setting).Error; err != nil {
		return 0
	}
	days, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil || days <= 0 {
		return 0
	}
	if days < 3 {
		return 3
	}
	if days > 7 {
		return 7
	}
	return days
}

func checksumString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func checksumJSON(value any) (string, string) {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return string(raw), hex.EncodeToString(sum[:])
}
func filepathBase(key string) string { parts := strings.Split(key, "/"); return parts[len(parts)-1] }
