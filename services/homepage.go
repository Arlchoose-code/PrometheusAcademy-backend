package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

type HomepageStats struct {
	Students string `json:"students"`
	Courses  string `json:"courses"`
	Regions  string `json:"regions"`
	Support  string `json:"support"`
}

type HomepageCourse struct {
	ID             uint   `json:"id"`
	TitleEn        string `json:"title_en"`
	TitleID        string `json:"title_id"`
	Slug           string `json:"slug"`
	DescriptionEn  string `json:"description_en"`
	DescriptionID  string `json:"description_id"`
	Thumbnail      string `json:"thumbnail"`
	Price          int    `json:"price"`
	IsFree         bool   `json:"is_free"`
	JoinedCount    int64  `json:"joined_count"`
	CompletedCount int64  `json:"completed_count"`
}

type HomepageProduct struct {
	ID             uint   `json:"id"`
	TitleEn        string `json:"title_en"`
	TitleID        string `json:"title_id"`
	Slug           string `json:"slug"`
	DescriptionEn  string `json:"description_en"`
	DescriptionID  string `json:"description_id"`
	Thumbnail      string `json:"thumbnail"`
	Price          int    `json:"price"`
	Type           string `json:"type"`
	CategorySlug   string `json:"category_slug"`
	CategoryNameEn string `json:"category_name_en"`
	CategoryNameID string `json:"category_name_id"`
	JoinedCount    int64  `json:"joined_count"`
	CompletedCount int64  `json:"completed_count"`
}

type HomepageKnowledgeCategory struct {
	Slug   string `json:"slug"`
	NameEn string `json:"name_en"`
	NameID string `json:"name_id"`
}

type HomepageTestimonial struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Company   string `json:"company"`
	Avatar    string `json:"avatar"`
	ContentEn string `json:"content_en"`
	ContentID string `json:"content_id"`
	Rating    int    `json:"rating"`
	SourceURL string `json:"source_url"`
}

type HomepagePartner struct {
	Name          string `json:"name"`
	Country       string `json:"country"`
	Logo          string `json:"logo"`
	Website       string `json:"website"`
	PartnerType   string `json:"partner_type"`
	ContactInfo   string `json:"contact_info"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	Status        string `json:"status"`
	Notes         string `json:"notes"`
}

type HomepageBanner struct {
	TitleEn   string `json:"title_en"`
	TitleID   string `json:"title_id"`
	ImagePath string `json:"image_path"`
	LinkURL   string `json:"link_url"`
	Order     int    `json:"order"`
}

type HomepageEnrollmentUrgency struct {
	Enabled   bool `json:"enabled"`
	Limit     int  `json:"limit"`
	Enrolled  int  `json:"enrolled"`
	Remaining int  `json:"remaining"`
}

type HomepageData struct {
	Stats               HomepageStats               `json:"stats"`
	Courses             []HomepageCourse            `json:"courses"`
	Products            []HomepageProduct           `json:"products"`
	KnowledgeCategories []HomepageKnowledgeCategory `json:"knowledge_categories"`
	Testimonials        []HomepageTestimonial       `json:"testimonials"`
	Partners            []HomepagePartner           `json:"partners"`
	Banners             []HomepageBanner            `json:"banners"`
	EnrollmentUrgency   HomepageEnrollmentUrgency   `json:"enrollment_urgency"`
}

type HomepageService interface {
	GetHomepage(ctx context.Context) (HomepageData, error)
}

type homepageService struct {
	db *gorm.DB
}

func NewHomepageService(db *gorm.DB) HomepageService {
	return &homepageService{db: db}
}

func (s *homepageService) GetHomepage(ctx context.Context) (HomepageData, error) {
	data := HomepageData{
		Stats: HomepageStats{
			Students: "10K+",
			Courses:  "4",
			Regions:  "2",
			Support:  "7/24",
		},
	}
	if s.db == nil {
		return data, nil
	}

	if err := s.applyStats(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadEnrollmentUrgency(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadCourses(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadKnowledgeCategories(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadProducts(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadTestimonials(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadPartners(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadBanners(ctx, &data); err != nil {
		return data, err
	}

	return data, nil
}

func (s *homepageService) loadEnrollmentUrgency(ctx context.Context, data *HomepageData) error {
	var rows []models.Setting
	if err := s.db.WithContext(ctx).Where("`key` IN ?", []string{"monthly_enrollment_limit", "monthly_enrollment_banner_enabled"}).Find(&rows).Error; err != nil {
		return fmt.Errorf("homepage enrollment settings: %w", err)
	}
	limit := 100
	enabled := true
	for _, row := range rows {
		switch row.Key {
		case "monthly_enrollment_limit":
			if value, err := strconv.Atoi(row.Value); err == nil && value >= 0 {
				limit = value
			}
		case "monthly_enrollment_banner_enabled":
			enabled = row.Value == "true"
		}
	}
	start := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	var enrolled int64
	if err := s.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("enrolled_at >= ?", start).Distinct("user_id").Count(&enrolled).Error; err != nil {
		return fmt.Errorf("homepage monthly enrollments: %w", err)
	}
	remaining := limit - int(enrolled)
	if remaining < 0 {
		remaining = 0
	}
	data.EnrollmentUrgency = HomepageEnrollmentUrgency{Enabled: enabled, Limit: limit, Enrolled: int(enrolled), Remaining: remaining}
	return nil
}

func (s *homepageService) loadKnowledgeCategories(ctx context.Context, data *HomepageData) error {
	var categories []models.ProductCategory
	if err := s.db.WithContext(ctx).
		Where("show_in_knowledge_base = ? AND requires_booking_time = ?", true, false).
		Order("id asc").
		Find(&categories).Error; err != nil {
		return fmt.Errorf("homepage knowledge categories: %w", err)
	}
	for _, category := range categories {
		data.KnowledgeCategories = append(data.KnowledgeCategories, HomepageKnowledgeCategory{
			Slug:   category.Slug,
			NameEn: category.NameEn,
			NameID: category.NameID,
		})
	}
	return nil
}

func (s *homepageService) loadBanners(ctx context.Context, data *HomepageData) error {
	var banners []models.Banner
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("`order` asc, id asc").Limit(3).Find(&banners).Error; err != nil {
		return fmt.Errorf("homepage banners: %w", err)
	}
	for _, banner := range banners {
		data.Banners = append(data.Banners, HomepageBanner{
			TitleEn:   banner.TitleEn,
			TitleID:   banner.TitleID,
			ImagePath: banner.ImagePath,
			LinkURL:   banner.LinkURL,
			Order:     banner.Order,
		})
	}
	return nil
}

func (s *homepageService) applyStats(ctx context.Context, data *HomepageData) error {
	var rows []models.Setting
	keys := []string{"home_students_stat", "home_courses_stat", "home_regions_stat", "home_support_stat"}
	if err := s.db.WithContext(ctx).Where("`key` IN ?", keys).Find(&rows).Error; err != nil {
		return fmt.Errorf("homepage stats: %w", err)
	}

	for _, row := range rows {
		if row.Value == "" {
			continue
		}
		switch row.Key {
		case "home_students_stat":
			data.Stats.Students = row.Value
		case "home_courses_stat":
			data.Stats.Courses = row.Value
		case "home_regions_stat":
			data.Stats.Regions = row.Value
		case "home_support_stat":
			data.Stats.Support = row.Value
		}
	}

	return nil
}

func (s *homepageService) loadCourses(ctx context.Context, data *HomepageData) error {
	var courses []models.Course
	if err := s.db.WithContext(ctx).Where("status = ?", "open").Order("created_at desc").Limit(3).Find(&courses).Error; err != nil {
		return fmt.Errorf("homepage courses: %w", err)
	}
	for _, course := range courses {
		var joined int64
		var completed int64
		if err := s.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("course_id = ?", course.ID).Count(&joined).Error; err != nil {
			return fmt.Errorf("homepage course joined count: %w", err)
		}
		if err := s.db.WithContext(ctx).Model(&models.CourseEnrollment{}).Where("course_id = ? AND completed_at IS NOT NULL", course.ID).Count(&completed).Error; err != nil {
			return fmt.Errorf("homepage course completed count: %w", err)
		}
		data.Courses = append(data.Courses, HomepageCourse{
			ID:             course.ID,
			TitleEn:        course.TitleEn,
			TitleID:        course.TitleID,
			Slug:           course.Slug,
			DescriptionEn:  course.DescriptionEn,
			DescriptionID:  course.DescriptionID,
			Thumbnail:      course.Thumbnail,
			Price:          course.Price,
			IsFree:         course.IsFree,
			JoinedCount:    joined,
			CompletedCount: completed,
		})
	}
	return nil
}

func (s *homepageService) loadProducts(ctx context.Context, data *HomepageData) error {
	if len(data.KnowledgeCategories) == 0 {
		return nil
	}
	var products []models.Product
	query := s.db.WithContext(ctx).Model(&models.Product{}).Where("is_published = ?", true)
	slugs := make([]string, 0, len(data.KnowledgeCategories))
	for _, category := range data.KnowledgeCategories {
		slugs = append(slugs, category.Slug)
	}
	query = query.Where("category_id IN (?)", s.db.Model(&models.ProductCategory{}).Select("id").Where("slug IN ?", slugs))
	if err := query.Order("created_at desc").Limit(6).Find(&products).Error; err != nil {
		return fmt.Errorf("homepage products: %w", err)
	}
	categoryIDs := make([]uint, 0, len(products))
	for _, product := range products {
		if product.CategoryID != 0 {
			categoryIDs = append(categoryIDs, product.CategoryID)
		}
	}
	categories := map[uint]models.ProductCategory{}
	if len(categoryIDs) > 0 {
		var rows []models.ProductCategory
		if err := s.db.WithContext(ctx).Where("id IN ?", categoryIDs).Find(&rows).Error; err != nil {
			return fmt.Errorf("homepage product categories: %w", err)
		}
		for _, row := range rows {
			categories[row.ID] = row
		}
	}
	for _, product := range products {
		category := categories[product.CategoryID]
		var purchased int64
		if err := s.db.WithContext(ctx).
			Table("order_items").
			Joins("JOIN orders ON orders.id = order_items.order_id").
			Where("order_items.item_type = ? AND order_items.item_id = ? AND orders.status = ?", "product", product.ID, "success").
			Count(&purchased).Error; err != nil {
			return fmt.Errorf("homepage product purchased count: %w", err)
		}
		data.Products = append(data.Products, HomepageProduct{
			ID:             product.ID,
			TitleEn:        product.TitleEn,
			TitleID:        product.TitleID,
			Slug:           product.Slug,
			DescriptionEn:  product.DescriptionEn,
			DescriptionID:  product.DescriptionID,
			Thumbnail:      product.Thumbnail,
			Price:          product.Price,
			Type:           product.Type,
			CategorySlug:   category.Slug,
			CategoryNameEn: category.NameEn,
			CategoryNameID: category.NameID,
			JoinedCount:    purchased,
			CompletedCount: 0,
		})
	}
	return nil
}

func (s *homepageService) loadTestimonials(ctx context.Context, data *HomepageData) error {
	var testimonials []models.Testimonial
	if err := s.db.WithContext(ctx).Where("is_active = ? AND review_status = ? AND display_context IN ?", true, "approved", []string{"general", "all"}).Order("created_at desc").Limit(3).Find(&testimonials).Error; err != nil {
		return fmt.Errorf("homepage testimonials: %w", err)
	}
	for _, testimonial := range testimonials {
		data.Testimonials = append(data.Testimonials, HomepageTestimonial{
			Name:      testimonial.Name,
			Role:      testimonial.Role,
			Company:   testimonial.Company,
			Avatar:    testimonial.Avatar,
			ContentEn: testimonial.ContentEn,
			ContentID: testimonial.ContentID,
			Rating:    testimonial.Rating,
			SourceURL: testimonial.SourceURL,
		})
	}
	return nil
}

func (s *homepageService) loadPartners(ctx context.Context, data *HomepageData) error {
	var partners []models.Partner
	if err := s.db.WithContext(ctx).Where("is_active = ? AND status = ?", true, "active").Order("partner_type desc, created_at desc").Find(&partners).Error; err != nil {
		return fmt.Errorf("homepage partners: %w", err)
	}
	for _, partner := range partners {
		data.Partners = append(data.Partners, HomepagePartner{
			Name:          partner.Name,
			Country:       partner.Country,
			Logo:          partner.Logo,
			Website:       partner.Website,
			PartnerType:   partner.PartnerType,
			ContactInfo:   partner.ContactInfo,
			DescriptionEn: partner.DescriptionEn,
			DescriptionID: partner.DescriptionID,
			Status:        partner.Status,
			Notes:         partner.Notes,
		})
	}
	return nil
}
