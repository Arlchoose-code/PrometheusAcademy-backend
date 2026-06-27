package services

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

func StorageFilePath(cfg config.Config, publicPath string) string {
	storageRoot := cfg.StoragePath
	if storageRoot == "" {
		storageRoot = "storage"
	}
	clean := strings.TrimPrefix(filepath.Clean(filepath.FromSlash(publicPath)), string(filepath.Separator))
	return filepath.Join(storageRoot, clean)
}

func MidtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	hash := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(hash[:])
}

func PaymentExpiresAt(cfg config.Config) time.Time {
	minutes := cfg.PaymentExpiresMinutes
	if minutes <= 0 {
		minutes = 30
	}
	return time.Now().Add(time.Duration(minutes) * time.Minute)
}

func OrderPaymentResponse(order models.Order, enrolled bool) map[string]any {
	return map[string]any{
		"id":                 order.ID,
		"order_id":           order.MidtransOrderID,
		"status":             order.Status,
		"amount":             order.TotalAmount,
		"snap_token":         order.SnapToken,
		"redirect_url":       order.SnapRedirectURL,
		"payment_expires_at": order.PaymentExpiresAt,
		"enrolled":           enrolled,
	}
}

func OrderPaymentItem(ctx context.Context, db *gorm.DB, orderID uint) (uint, string, error) {
	var item models.OrderItem
	if err := db.WithContext(ctx).Where("order_id = ?", orderID).Order("id asc").First(&item).Error; err != nil {
		return 0, "", err
	}
	if item.ItemType == "course" {
		var course models.Course
		if err := db.WithContext(ctx).First(&course, item.ItemID).Error; err != nil {
			return 0, "", err
		}
		return course.ID, course.TitleEn, nil
	}
	var product models.Product
	if err := db.WithContext(ctx).First(&product, item.ItemID).Error; err != nil {
		return 0, "", err
	}
	return product.ID, product.TitleEn, nil
}

func EnsureOrderPaymentToken(ctx context.Context, db *gorm.DB, cfg config.Config, order *models.Order, itemID uint, itemName string, user models.User) (string, string, error) {
	if order.Status != "pending" {
		return order.SnapToken, order.SnapRedirectURL, nil
	}
	if order.SnapToken != "" && order.PaymentExpiresAt != nil && order.PaymentExpiresAt.After(time.Now()) {
		return order.SnapToken, order.SnapRedirectURL, nil
	}
	token, redirectURL, err := createMidtransSnapToken(ctx, cfg, *order, itemID, itemName, user)
	if err != nil {
		return "", "", err
	}
	expiresAt := PaymentExpiresAt(cfg)
	updates := map[string]any{
		"snap_token":         token,
		"snap_redirect_url":  redirectURL,
		"payment_expires_at": &expiresAt,
	}
	if err := db.WithContext(ctx).Model(order).Updates(updates).Error; err != nil {
		return "", "", err
	}
	order.SnapToken = token
	order.SnapRedirectURL = redirectURL
	order.PaymentExpiresAt = &expiresAt
	return token, redirectURL, nil
}

func createMidtransSnapToken(ctx context.Context, cfg config.Config, order models.Order, itemID uint, itemName string, user models.User) (string, string, error) {
	if cfg.MidtransServerKey == "" {
		return "dev-snap-token-" + order.MidtransOrderID, "", nil
	}
	endpoint := "https://app.sandbox.midtrans.com/snap/v1/transactions"
	if cfg.MidtransEnv == "production" {
		endpoint = "https://app.midtrans.com/snap/v1/transactions"
	}
	payload := map[string]any{
		"transaction_details": map[string]any{"order_id": order.MidtransOrderID, "gross_amount": order.TotalAmount},
		"item_details":        []map[string]any{{"id": itemID, "price": order.TotalAmount, "quantity": 1, "name": itemName}},
		"customer_details":    map[string]any{"first_name": user.Name, "email": user.Email, "phone": user.Phone},
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	request.SetBasicAuth(cfg.MidtransServerKey, "")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		return "", "", fmt.Errorf("Midtrans request failed")
	}
	var result struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", err
	}
	return result.Token, result.RedirectURL, nil
}

func SyncOrderPaymentStatus(ctx context.Context, db *gorm.DB, cfg config.Config, order *models.Order) error {
	if cfg.MidtransServerKey == "" {
		return fmt.Errorf("Midtrans server key is not configured")
	}
	endpoint := "https://api.sandbox.midtrans.com/v2/" + order.MidtransOrderID + "/status"
	if cfg.MidtransEnv == "production" {
		endpoint = "https://api.midtrans.com/v2/" + order.MidtransOrderID + "/status"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.SetBasicAuth(cfg.MidtransServerKey, "")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("Payment transaction not found")
	}
	if response.StatusCode >= 300 {
		return fmt.Errorf("Failed to sync payment status")
	}
	var result struct {
		TransactionID     string `json:"transaction_id"`
		PaymentType       string `json:"payment_type"`
		TransactionStatus string `json:"transaction_status"`
		FraudStatus       string `json:"fraud_status"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	status := MapMidtransStatus(result.TransactionStatus)
	if result.TransactionStatus == "capture" && result.FraudStatus == "challenge" {
		status = "pending"
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": status}
		if status == "success" {
			now := time.Now()
			updates["paid_at"] = &now
		}
		if err := tx.Model(order).Updates(updates).Error; err != nil {
			return err
		}
		order.Status = status
		trxID := strings.TrimSpace(result.TransactionID)
		if trxID == "" {
			trxID = "TRX-" + order.MidtransOrderID
		}
		trx := models.Transaction{OrderID: order.ID, MidtransTransactionID: trxID, PaymentType: result.PaymentType, Status: status, RawResponse: string(raw)}
		if err := tx.Where(models.Transaction{MidtransTransactionID: trx.MidtransTransactionID}).Assign(trx).FirstOrCreate(&trx).Error; err != nil {
			return err
		}
		if status == "success" {
			if err := FulfillSuccessfulOrder(ctx, tx, *order); err != nil {
				return err
			}
			invoice, err := EnsureInvoice(ctx, tx, cfg, *order)
			if err == nil {
				_ = SendOrderPaymentEmails(ctx, tx, cfg, *order, invoice)
			}
			return err
		}
		return nil
	})
}

func SendOrderPaymentEmails(ctx context.Context, db *gorm.DB, cfg config.Config, order models.Order, invoice models.Invoice) error {
	var user models.User
	if err := db.WithContext(ctx).First(&user, order.UserID).Error; err != nil {
		return err
	}
	_, itemName, _ := OrderPaymentItem(ctx, db, order.ID)
	amount := fmt.Sprintf("Rp %d", order.TotalAmount)
	variables := map[string]string{
		"amount":         amount,
		"product":        itemName,
		"transaction_id": order.MidtransOrderID,
		"invoice_number": invoice.InvoiceNumber,
		"invoice_url":    localizedFrontendURL(cfg, user.Language, fmt.Sprintf("/downloads/invoices/%d", order.ID)),
		"dashboard_url":  localizedFrontendURL(cfg, user.Language, "/dashboard"),
	}
	_ = SendTransactionalTemplateEmail(ctx, db, EmailTemplatePaymentSuccess, "payment_success", user, variables)
	_ = SendTransactionalTemplateEmail(ctx, db, EmailTemplateInvoice, "invoice", user, variables)
	return nil
}

func MapMidtransStatus(status string) string {
	switch status {
	case "capture", "settlement":
		return "success"
	case "cancel", "expire":
		return "cancelled"
	case "deny", "failure":
		return "failed"
	default:
		return "pending"
	}
}

func CancelExpiredPendingOrders(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	return db.WithContext(ctx).
		Model(&models.Order{}).
		Where("status = ? AND payment_expires_at IS NOT NULL AND payment_expires_at < ?", "pending", time.Now()).
		Update("status", "cancelled").
		Error
}

func ReconcileCouponUsageCounts(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if err := db.WithContext(ctx).Exec(`
		DELETE cu FROM coupon_usages cu
		JOIN orders o ON o.id = cu.order_id
		WHERE o.status <> ?
	`, "success").Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Model(&models.Coupon{}).Where("1 = 1").Update("used_count", 0).Error; err != nil {
		return err
	}
	type row struct {
		CouponID uint
		Count    int
	}
	var rows []row
	if err := db.WithContext(ctx).Raw(`
		SELECT cu.coupon_id AS coupon_id, COUNT(*) AS count
		FROM coupon_usages cu
		JOIN orders o ON o.id = cu.order_id
		WHERE o.status = ?
		GROUP BY cu.coupon_id
	`, "success").Scan(&rows).Error; err != nil {
		return err
	}
	for _, item := range rows {
		if err := db.WithContext(ctx).Model(&models.Coupon{}).Where("id = ?", item.CouponID).Update("used_count", item.Count).Error; err != nil {
			return err
		}
	}
	return nil
}

func DiscountedAmount(ctx context.Context, db *gorm.DB, amount int, code string, scope string) (int, *models.Coupon, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return amount, nil, nil
	}
	var coupon models.Coupon
	if err := db.WithContext(ctx).Where("code = ?", code).First(&coupon).Error; err != nil {
		return amount, nil, fmt.Errorf("Coupon is invalid")
	}
	if coupon.ExpiresAt != nil && coupon.ExpiresAt.Before(time.Now()) {
		return amount, nil, fmt.Errorf("Coupon has expired")
	}
	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return amount, nil, fmt.Errorf("Coupon usage limit reached")
	}
	if coupon.AppliesTo != "all" && coupon.AppliesTo != scope {
		return amount, nil, fmt.Errorf("Coupon cannot be used here")
	}
	next := amount
	if coupon.DiscountType == "percent" {
		next = amount - (amount * coupon.DiscountValue / 100)
	} else {
		next = amount - coupon.DiscountValue
	}
	if next < 0 {
		next = 0
	}
	return next, &coupon, nil
}

func PendingOrderForItem(ctx context.Context, db *gorm.DB, userID uint, itemType string, itemID uint) (models.Order, bool) {
	var order models.Order
	err := db.WithContext(ctx).
		Joins("JOIN order_items oi ON oi.order_id = orders.id").
		Where("orders.user_id = ? AND orders.status = ? AND oi.item_type = ? AND oi.item_id = ?", userID, "pending", itemType, itemID).
		Order("orders.created_at desc").
		First(&order).Error
	return order, err == nil
}

func FulfillSuccessfulOrder(ctx context.Context, db *gorm.DB, order models.Order) error {
	if order.AppliedCouponID != 0 {
		usage := models.CouponUsage{CouponID: order.AppliedCouponID, UserID: order.UserID, OrderID: order.ID, UsedAt: time.Now()}
		result := db.WithContext(ctx).Where(models.CouponUsage{CouponID: order.AppliedCouponID, OrderID: order.ID}).Attrs(usage).FirstOrCreate(&usage)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			if err := db.WithContext(ctx).Model(&models.Coupon{}).Where("id = ?", order.AppliedCouponID).UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error; err != nil {
				return err
			}
		}
	}
	var items []models.OrderItem
	if err := db.WithContext(ctx).Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if item.ItemType != "course" {
			continue
		}
		enrollment := models.CourseEnrollment{UserID: order.UserID, CourseID: item.ItemID, EnrolledAt: time.Now()}
		result := db.WithContext(ctx).Where(models.CourseEnrollment{UserID: order.UserID, CourseID: item.ItemID}).Attrs(enrollment).FirstOrCreate(&enrollment)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			if err := AwardXP(ctx, db, order.UserID, XPEventCourseEnrolled, "course", item.ItemID, XPCourseEnrolled, "Enrolled in a course", "Terdaftar di course"); err != nil {
				return err
			}
		}
	}
	return nil
}

func EnsureInvoice(ctx context.Context, db *gorm.DB, cfg config.Config, order models.Order) (models.Invoice, error) {
	var invoice models.Invoice
	if err := db.WithContext(ctx).Where(models.Invoice{OrderID: order.ID}).First(&invoice).Error; err == nil {
		return invoice, nil
	}
	var user models.User
	_ = db.WithContext(ctx).First(&user, order.UserID).Error
	_, version, _ := SelectDocumentTemplateVersion(ctx, db, "invoice", 0)
	snapshot := map[string]any{
		"invoice_number":    fmt.Sprintf("INV-%06d", order.ID),
		"customer_name":     fallbackString(user.Name, "Customer"),
		"customer_email":    user.Email,
		"subtotal":          fmt.Sprintf("Rp %d", order.TotalAmount),
		"discount":          "Rp 0",
		"total":             fmt.Sprintf("Rp %d", order.TotalAmount),
		"currency":          "IDR",
		"paid_at":           time.Now().Format(time.RFC3339),
		"payment_reference": order.MidtransOrderID,
		"payment_method":    "Midtrans",
		"items_table":       invoiceItemsTextLocale(ctx, db, order.ID, fallbackString(user.Language, "en")),
		"items_table_en":    invoiceItemsTextLocale(ctx, db, order.ID, "en"),
		"items_table_id":    invoiceItemsTextLocale(ctx, db, order.ID, "id"),
		"site_name":         "Prometheus Academy",
		"document_number":   fmt.Sprintf("INV-%06d", order.ID),
		"recipient_name":    fallbackString(user.Name, "Customer"),
		"recipient_email":   user.Email,
		"issued_at":         time.Now().Format("2006-01-02"),
		"locale":            fallbackString(user.Language, "en"),
	}
	rawSnapshot, snapshotChecksum := checksumJSON(snapshot)

	invoice = models.Invoice{
		OrderID:           order.ID,
		InvoiceNumber:     fmt.Sprintf("INV-%06d", order.ID),
		FilePath:          fmt.Sprintf("/uploads/invoices/invoice-%d.pdf", order.ID),
		IssuedAt:          time.Now(),
		TemplateID:        version.TemplateID,
		TemplateVersionID: version.ID,
		Locale:            fallbackString(user.Language, "en"),
		SnapshotJSON:      rawSnapshot,
		SnapshotChecksum:  snapshotChecksum,
	}
	if err := db.WithContext(ctx).Create(&invoice).Error; err != nil {
		return models.Invoice{}, err
	}
	return invoice, nil
}

func EnsureInvoicePDF(ctx context.Context, db *gorm.DB, cfg config.Config, invoice models.Invoice) ([]byte, error) {
	var version models.DocumentTemplateVersion
	if invoice.TemplateVersionID == 0 {
		_, fallbackVersion, err := SelectDocumentTemplateVersion(ctx, db, "invoice", 0)
		if err != nil {
			return nil, err
		}
		version = fallbackVersion
	} else if err := db.WithContext(ctx).First(&version, invoice.TemplateVersionID).Error; err != nil {
		return nil, err
	}
	var variables map[string]string
	if err := json.Unmarshal([]byte(invoice.SnapshotJSON), &variables); err != nil {
		var generic map[string]any
		_ = json.Unmarshal([]byte(invoice.SnapshotJSON), &generic)
		variables = mapStringAny(generic)
	}
	if len(variables) == 0 {
		variables = invoiceFallbackVariables(ctx, db, invoice)
	}
	for key, value := range invoiceFallbackVariables(ctx, db, invoice) {
		if strings.TrimSpace(variables[key]) == "" {
			variables[key] = value
		}
	}
	if regexp.MustCompile(`(?i)\b(course|product|course_addon|addon)\s+#\d+`).MatchString(variables["items_table"]) {
		variables["items_table"] = invoiceItemsTextLocale(ctx, db, invoice.OrderID, invoice.Locale)
	}
	if invoice.Locale == "id" {
		variables["items_table"] = fallbackString(variables["items_table_id"], invoiceItemsTextLocale(ctx, db, invoice.OrderID, "id"))
	} else {
		variables["items_table"] = fallbackString(variables["items_table_en"], invoiceItemsTextLocale(ctx, db, invoice.OrderID, "en"))
	}
	template := version.HTMLEn
	if invoice.Locale == "id" && strings.TrimSpace(version.HTMLID) != "" {
		template = version.HTMLID
	}
	pdf, err := RenderDocumentPDF(ctx, cfg, template, variables, "portrait")
	if err != nil {
		return nil, err
	}
	if cacheDays := DocumentPDFCacheDays(ctx, db); cacheDays > 0 {
		key := fmt.Sprintf("generated/invoices/%d/%d-%s.pdf", invoice.ID, version.ID, invoice.SnapshotChecksum)
		_ = StoreGeneratedPDF(ctx, db, cfg, key, pdf, invoice.FilePath, cacheDays)
		expires := time.Now().Add(time.Duration(cacheDays) * 24 * time.Hour)
		_ = db.WithContext(ctx).Model(&invoice).Updates(map[string]any{"cached_object_key": key, "cache_expires_at": &expires}).Error
	}
	return pdf, nil
}

func invoiceFallbackVariables(ctx context.Context, db *gorm.DB, invoice models.Invoice) map[string]string {
	var order models.Order
	var user models.User
	_ = db.WithContext(ctx).First(&order, invoice.OrderID).Error
	_ = db.WithContext(ctx).First(&user, order.UserID).Error
	paidAt := invoice.IssuedAt
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	invoiceNumber := strings.TrimSpace(invoice.InvoiceNumber)
	if invoiceNumber == "" {
		invoiceNumber = fmt.Sprintf("INV-%06d", invoice.OrderID)
	}
	return map[string]string{
		"site_name":         "Prometheus Academy",
		"document_number":   invoiceNumber,
		"recipient_name":    fallbackString(user.Name, "Customer"),
		"recipient_email":   user.Email,
		"issued_at":         paidAt.Format("2006-01-02"),
		"locale":            fallbackString(user.Language, "en"),
		"verification_url":  "",
		"qr_verification":   "",
		"invoice_number":    invoiceNumber,
		"customer_name":     fallbackString(user.Name, "Customer"),
		"customer_email":    user.Email,
		"items_table":       invoiceItemsTextLocale(ctx, db, invoice.OrderID, fallbackString(user.Language, "en")),
		"items_table_en":    invoiceItemsTextLocale(ctx, db, invoice.OrderID, "en"),
		"items_table_id":    invoiceItemsTextLocale(ctx, db, invoice.OrderID, "id"),
		"subtotal":          formatIDR(order.TotalAmount),
		"discount":          "Rp 0",
		"total":             formatIDR(order.TotalAmount),
		"currency":          "IDR",
		"paid_at":           paidAt.Format("2006-01-02"),
		"payment_reference": order.MidtransOrderID,
		"payment_method":    "Midtrans",
	}
}

func invoiceItemsText(ctx context.Context, db *gorm.DB, orderID uint) string {
	return invoiceItemsTextLocale(ctx, db, orderID, "en")
}

func invoiceItemsTextLocale(ctx context.Context, db *gorm.DB, orderID uint, locale string) string {
	var items []models.OrderItem
	if err := db.WithContext(ctx).Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return "-"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s - %s", invoiceItemTitle(ctx, db, item, locale), formatIDR(item.Price)))
	}
	if len(lines) == 0 {
		return "-"
	}
	return strings.Join(lines, "\n")
}

func invoiceItemTitle(ctx context.Context, db *gorm.DB, item models.OrderItem, locale string) string {
	switch strings.ToLower(strings.TrimSpace(item.ItemType)) {
	case "course":
		var course models.Course
		if err := db.WithContext(ctx).Select("title_en", "title_id").First(&course, item.ItemID).Error; err == nil {
			if locale == "id" {
				return fallbackString(course.TitleID, course.TitleEn)
			}
			return fallbackString(course.TitleEn, course.TitleID)
		}
	case "product":
		var product models.Product
		if err := db.WithContext(ctx).Select("title_en", "title_id").First(&product, item.ItemID).Error; err == nil {
			if locale == "id" {
				return fallbackString(product.TitleID, product.TitleEn)
			}
			return fallbackString(product.TitleEn, product.TitleID)
		}
	case "course_addon", "addon":
		var addon models.CourseAddon
		if err := db.WithContext(ctx).First(&addon, item.ItemID).Error; err == nil {
			if locale == "id" {
				return fallbackString(addon.TitleID, addon.TitleEn)
			}
			return fallbackString(addon.TitleEn, addon.TitleID)
		}
	}
	return "Prometheus Academy item"
}

func formatIDR(amount int) string {
	raw := strconv.Itoa(amount)
	if raw == "" {
		return "Rp 0"
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return "Rp " + strings.Join(parts, ".")
}

func mapStringAny(input map[string]any) map[string]string {
	out := map[string]string{}
	for key, value := range input {
		out[key] = fmt.Sprint(value)
	}
	return out
}

func writeSimpleInvoicePDF(path string, invoice models.Invoice, order models.Order) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	body := fmt.Sprintf("Prometheus Academy Invoice\n\nInvoice: %s\nOrder: %s\nAmount: Rp %d\nStatus: %s\nIssued: %s\n",
		invoice.InvoiceNumber,
		order.MidtransOrderID,
		order.TotalAmount,
		order.Status,
		invoice.IssuedAt.Format("2006-01-02 15:04"),
	)
	pdf := "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj\n5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\n"
	stream := "BT /F1 12 Tf 72 720 Td " + pdfEscape(body) + " Tj ET"
	pdf += fmt.Sprintf("4 0 obj<</Length %d>>stream\n%s\nendstream endobj\nxref\n0 6\n0000000000 65535 f \ntrailer<</Root 1 0 R/Size 6>>\nstartxref\n0\n%%%%EOF", len(stream), stream)
	return os.WriteFile(path, []byte(pdf), 0o600)
}

func pdfEscape(text string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\n", "\\n")
	return "(" + replacer.Replace(text) + ")"
}
