package services

import (
	"context"
	"fmt"

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
	ID            uint   `json:"id"`
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	Slug          string `json:"slug"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	Thumbnail     string `json:"thumbnail"`
	Price         int    `json:"price"`
	IsFree        bool   `json:"is_free"`
}

type HomepageProduct struct {
	ID            uint   `json:"id"`
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	Slug          string `json:"slug"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	Thumbnail     string `json:"thumbnail"`
	Price         int    `json:"price"`
	Type          string `json:"type"`
}

type HomepageTestimonial struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Company   string `json:"company"`
	Avatar    string `json:"avatar"`
	ContentEn string `json:"content_en"`
	ContentID string `json:"content_id"`
	Rating    int    `json:"rating"`
}

type HomepagePartner struct {
	Name          string `json:"name"`
	Country       string `json:"country"`
	Logo          string `json:"logo"`
	Website       string `json:"website"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
}

type HomepageBanner struct {
	TitleEn   string `json:"title_en"`
	TitleID   string `json:"title_id"`
	ImagePath string `json:"image_path"`
	LinkURL   string `json:"link_url"`
	Order     int    `json:"order"`
}

type HomepageData struct {
	Stats        HomepageStats         `json:"stats"`
	Courses      []HomepageCourse      `json:"courses"`
	Products     []HomepageProduct     `json:"products"`
	Testimonials []HomepageTestimonial `json:"testimonials"`
	Partners     []HomepagePartner     `json:"partners"`
	Banners      []HomepageBanner      `json:"banners"`
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
	if err := s.loadCourses(ctx, &data); err != nil {
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
		data.Courses = append(data.Courses, HomepageCourse{
			ID:            course.ID,
			TitleEn:       course.TitleEn,
			TitleID:       course.TitleID,
			Slug:          course.Slug,
			DescriptionEn: course.DescriptionEn,
			DescriptionID: course.DescriptionID,
			Thumbnail:     course.Thumbnail,
			Price:         course.Price,
			IsFree:        course.IsFree,
		})
	}
	return nil
}

func (s *homepageService) loadProducts(ctx context.Context, data *HomepageData) error {
	var products []models.Product
	if err := s.db.WithContext(ctx).Where("is_published = ?", true).Order("created_at desc").Limit(3).Find(&products).Error; err != nil {
		return fmt.Errorf("homepage products: %w", err)
	}
	for _, product := range products {
		data.Products = append(data.Products, HomepageProduct{
			ID:            product.ID,
			TitleEn:       product.TitleEn,
			TitleID:       product.TitleID,
			Slug:          product.Slug,
			DescriptionEn: product.DescriptionEn,
			DescriptionID: product.DescriptionID,
			Thumbnail:     product.Thumbnail,
			Price:         product.Price,
			Type:          product.Type,
		})
	}
	return nil
}

func (s *homepageService) loadTestimonials(ctx context.Context, data *HomepageData) error {
	var testimonials []models.Testimonial
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("created_at desc").Limit(3).Find(&testimonials).Error; err != nil {
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
		})
	}
	return nil
}

func (s *homepageService) loadPartners(ctx context.Context, data *HomepageData) error {
	var partners []models.Partner
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("created_at desc").Limit(6).Find(&partners).Error; err != nil {
		return fmt.Errorf("homepage partners: %w", err)
	}
	for _, partner := range partners {
		data.Partners = append(data.Partners, HomepagePartner{
			Name:          partner.Name,
			Country:       partner.Country,
			Logo:          partner.Logo,
			Website:       partner.Website,
			DescriptionEn: partner.DescriptionEn,
			DescriptionID: partner.DescriptionID,
		})
	}
	return nil
}
