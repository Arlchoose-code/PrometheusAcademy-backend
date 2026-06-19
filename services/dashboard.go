package services

import (
	"context"
	"fmt"
	"time"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

type DashboardService struct {
	db *gorm.DB
}

type DashboardData struct {
	Stats        DashboardStats         `json:"stats"`
	Courses      []DashboardCourse      `json:"courses"`
	Certificates []DashboardCertificate `json:"certificates"`
	Transactions []DashboardTransaction `json:"transactions"`
	Gamification GamificationSummary    `json:"gamification"`
}

type DashboardStats struct {
	CoursesEnrolled int `json:"courses_enrolled"`
	Completed       int `json:"completed"`
	Certificates    int `json:"certificates"`
	TotalSpent      int `json:"total_spent"`
}

type DashboardCourse struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Thumbnail   string `json:"thumbnail"`
	Progress    int    `json:"progress"`
	EnrolledAt  string `json:"enrolled_at"`
	ContinueURL string `json:"continue_url"`
}

type DashboardCertificate struct {
	CourseName     string `json:"course_name"`
	IssuedAt       string `json:"issued_at"`
	CertificateURL string `json:"certificate_url"`
}

type DashboardTransaction struct {
	OrderID             uint   `json:"order_id"`
	ItemID              uint   `json:"item_id"`
	ItemType            string `json:"item_type"`
	ProductType         string `json:"product_type"`
	RequiresBookingTime bool   `json:"requires_booking_time"`
	Item                string `json:"item"`
	Amount              int    `json:"amount"`
	Status              string `json:"status"`
	Date                string `json:"date"`
	InvoiceURL          string `json:"invoice_url"`
	DownloadURL         string `json:"download_url"`
}

func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

func (s *DashboardService) GetStudentDashboard(ctx context.Context, userID uint) (DashboardData, error) {
	if s.db == nil {
		return DashboardData{}, fmt.Errorf("database is not configured")
	}
	if err := CancelExpiredPendingOrders(ctx, s.db); err != nil {
		return DashboardData{}, fmt.Errorf("dashboard payment expiry: %w", err)
	}

	var stats DashboardStats
	var coursesEnrolled, completed, certificateCount int64
	if err := s.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("user_id = ?", userID).Count(&coursesEnrolled).Error; err != nil {
		return DashboardData{}, fmt.Errorf("dashboard enrolled count: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("user_id = ? AND completed_at IS NOT NULL", userID).Count(&completed).Error; err != nil {
		return DashboardData{}, fmt.Errorf("dashboard completed count: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&models.Certificate{}).Where("user_id = ?", userID).Count(&certificateCount).Error; err != nil {
		return DashboardData{}, fmt.Errorf("dashboard certificate count: %w", err)
	}
	stats.CoursesEnrolled = int(coursesEnrolled)
	stats.Completed = int(completed)
	stats.Certificates = int(certificateCount)
	if err := s.db.WithContext(ctx).Model(&models.Order{}).Select("COALESCE(SUM(total_amount), 0)").Where("user_id = ? AND status = ?", userID, "success").Scan(&stats.TotalSpent).Error; err != nil {
		return DashboardData{}, fmt.Errorf("dashboard total spent: %w", err)
	}

	courses, err := s.dashboardCourses(ctx, userID)
	if err != nil {
		return DashboardData{}, err
	}
	certificates, err := s.dashboardCertificates(ctx, userID)
	if err != nil {
		return DashboardData{}, err
	}
	transactions, err := s.dashboardTransactions(ctx, userID)
	if err != nil {
		return DashboardData{}, err
	}
	gamification, err := GamificationForUser(ctx, s.db, userID)
	if err != nil {
		return DashboardData{}, err
	}

	return DashboardData{Stats: stats, Courses: courses, Certificates: certificates, Transactions: transactions, Gamification: gamification}, nil
}

func (s *DashboardService) dashboardCourses(ctx context.Context, userID uint) ([]DashboardCourse, error) {
	type row struct {
		Title       string
		Slug        string
		Thumbnail   string
		EnrolledAt  time.Time
		TotalTopics int
		DoneTopics  int
	}

	var rows []row
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			c.title_en AS title,
			c.slug,
			c.thumbnail,
			e.enrolled_at,
			COUNT(DISTINCT t.id) AS total_topics,
			COUNT(DISTINCT CASE WHEN tp.completed_at IS NOT NULL THEN tp.id END) AS done_topics
		FROM course_enrollments e
		JOIN courses c ON c.id = e.course_id
		LEFT JOIN course_modules m ON m.course_id = c.id
		LEFT JOIN topics t ON t.module_id = m.id
		LEFT JOIN topic_progresses tp ON tp.topic_id = t.id AND tp.user_id = e.user_id
		WHERE e.user_id = ?
		GROUP BY c.id, c.title_en, c.slug, c.thumbnail, e.enrolled_at
		ORDER BY e.enrolled_at DESC
		LIMIT 6
	`, userID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("dashboard courses: %w", err)
	}

	courses := make([]DashboardCourse, 0, len(rows))
	for _, item := range rows {
		progress := 0
		if item.TotalTopics > 0 {
			progress = (item.DoneTopics * 100) / item.TotalTopics
		}
		courses = append(courses, DashboardCourse{
			Title:       item.Title,
			Slug:        item.Slug,
			Thumbnail:   item.Thumbnail,
			Progress:    progress,
			EnrolledAt:  item.EnrolledAt.Format(time.RFC3339),
			ContinueURL: "/dashboard/courses/" + item.Slug + "/learn",
		})
	}

	return courses, nil
}

func (s *DashboardService) dashboardCertificates(ctx context.Context, userID uint) ([]DashboardCertificate, error) {
	type row struct {
		ID             uint
		UUID           string
		CourseName     string
		IssuedAt       time.Time
		CertificateURL string
	}

	var rows []row
	if err := s.db.WithContext(ctx).Raw(`
		SELECT cert.id, cert.uuid, c.title_en AS course_name, cert.issued_at, cert.certificate_url
		FROM certificates cert
		JOIN courses c ON c.id = cert.course_id
		WHERE cert.user_id = ?
		ORDER BY cert.issued_at DESC
		LIMIT 4
	`, userID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("dashboard certificates: %w", err)
	}

	items := make([]DashboardCertificate, 0, len(rows))
	for _, row := range rows {
		certificate := models.Certificate{}
		certificate.ID = row.ID
		certificate.UUID = row.UUID
		if err := EnsureCertificateUUID(ctx, s.db, &certificate); err != nil {
			return nil, fmt.Errorf("dashboard certificate uuid: %w", err)
		}
		items = append(items, DashboardCertificate{
			CourseName:     row.CourseName,
			IssuedAt:       row.IssuedAt.Format(time.RFC3339),
			CertificateURL: CertificateDownloadURL(certificate),
		})
	}
	return items, nil
}

func (s *DashboardService) dashboardTransactions(ctx context.Context, userID uint) ([]DashboardTransaction, error) {
	type row struct {
		OrderID             uint
		ItemID              uint
		ItemType            string
		ProductType         string
		RequiresBookingTime bool
		Item                string
		Amount              int
		Status              string
		CreatedAt           time.Time
	}

	var rows []row
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			o.id AS order_id,
			oi.item_id,
			oi.item_type,
			COALESCE(p.type, '') AS product_type,
			(COALESCE(pc.requires_booking_time, false) OR COALESCE(course_booking.requires_booking_time, false)) AS requires_booking_time,
			COALESCE(c.title_en, p.title_en, o.midtrans_order_id) AS item,
			oi.price AS amount,
			o.status,
			o.created_at
		FROM orders o
		LEFT JOIN order_items oi ON oi.order_id = o.id
		LEFT JOIN courses c ON oi.item_type = 'course' AND c.id = oi.item_id
		LEFT JOIN products p ON oi.item_type = 'product' AND p.id = oi.item_id
		LEFT JOIN product_categories pc ON pc.id = p.category_id
		LEFT JOIN (
			SELECT ca.course_id, MAX(CASE WHEN addon_categories.requires_booking_time THEN 1 ELSE 0 END) AS requires_booking_time
			FROM course_addons ca
			JOIN product_categories addon_categories ON addon_categories.id = ca.product_category_id
			WHERE ca.is_active = true
			GROUP BY ca.course_id
		) course_booking ON course_booking.course_id = c.id
		WHERE o.user_id = ?
		ORDER BY o.created_at DESC
		LIMIT 6
	`, userID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("dashboard transactions: %w", err)
	}

	items := make([]DashboardTransaction, 0, len(rows))
	for _, row := range rows {
		invoiceURL := ""
		downloadURL := ""
		if row.Status == "success" {
			invoiceURL = fmt.Sprintf("/downloads/invoices/%d", row.OrderID)
			if row.ItemType == "product" && row.ItemID != 0 && !row.RequiresBookingTime {
				downloadURL = fmt.Sprintf("/downloads/products/%d", row.ItemID)
			}
		}
		items = append(items, DashboardTransaction{
			OrderID:             row.OrderID,
			ItemID:              row.ItemID,
			ItemType:            row.ItemType,
			ProductType:         row.ProductType,
			RequiresBookingTime: row.RequiresBookingTime,
			Item:                row.Item,
			Amount:              row.Amount,
			Status:              row.Status,
			Date:                row.CreatedAt.Format(time.RFC3339),
			InvoiceURL:          invoiceURL,
			DownloadURL:         downloadURL,
		})
	}
	return items, nil
}
