package admin

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) ListPages(c *gin.Context) {
	var rows []models.Page
	if err := h.db.WithContext(c.Request.Context()).Order("slug asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load pages"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Pages loaded", Data: rows})
}

func (h *Controller) UpdatePage(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	var req models.Page
	if err := c.ShouldBindJSON(&req); err != nil || slug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid page payload"})
		return
	}
	page := models.Page{
		Slug:          slug,
		TitleEn:       req.TitleEn,
		TitleID:       req.TitleID,
		DescriptionEn: req.DescriptionEn,
		DescriptionID: req.DescriptionID,
		ImagePath:     req.ImagePath,
		ContentEn:     req.ContentEn,
		ContentID:     req.ContentID,
	}
	if err := h.db.WithContext(c.Request.Context()).Where(models.Page{Slug: slug}).Assign(page).FirstOrCreate(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save page"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Page saved", Data: page})
}

func (h *Controller) UpdatePageImage(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	file, err := c.FormFile("file")
	if err != nil || slug == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Page image file is required"})
		return
	}
	path, err := h.uploadService.SavePageImage(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Page{}).Where("slug = ?", slug).Update("image_path", path).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update page image"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Page image uploaded", Data: gin.H{"image_path": path}})
}

func (h *Controller) ReorderFAQs(c *gin.Context) {
	var req []struct {
		ID    uint `json:"id"`
		Order int  `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid FAQ order payload"})
		return
	}
	for _, item := range req {
		if item.ID == 0 {
			continue
		}
		if err := h.db.WithContext(c.Request.Context()).Model(&models.FAQ{}).Where("id = ?", item.ID).Update("order", item.Order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reorder FAQs"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "FAQs reordered"})
}

func (h *Controller) ListFAQs(c *gin.Context) {
	listRows[models.FAQ](h.db, "`order` asc, id asc", "FAQs loaded")(c)
}

func (h *Controller) CreateFAQ(c *gin.Context) {
	createRow[models.FAQ](h.db, "FAQ created")(c)
}

func (h *Controller) UpdateFAQ(c *gin.Context) {
	updateRow[models.FAQ](h.db, "FAQ saved")(c)
}

func (h *Controller) DeleteFAQ(c *gin.Context) {
	deleteRow[models.FAQ](h.db, "FAQ deleted")(c)
}

func (h *Controller) ListTestimonials(c *gin.Context) {
	listRows[models.Testimonial](h.db, "created_at desc", "Testimonials loaded")(c)
}

func (h *Controller) CreateTestimonial(c *gin.Context) {
	var testimonial models.Testimonial
	if err := c.ShouldBindJSON(&testimonial); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid testimonial payload"})
		return
	}
	normalizeTestimonialReview(&testimonial)
	if err := h.db.WithContext(c.Request.Context()).Create(&testimonial).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create testimonial"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Testimonial created", Data: testimonial})
}

func (h *Controller) UpdateTestimonial(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
		return
	}
	var testimonial models.Testimonial
	if err := c.ShouldBindJSON(&testimonial); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid testimonial payload"})
		return
	}
	normalizeTestimonialReview(&testimonial)
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Testimonial{}).Where("id = ?", uint(id)).Select("*").Omit("id", "created_at").Updates(testimonial).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save testimonial"})
		return
	}
	testimonial.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Testimonial saved", Data: testimonial})
}

func (h *Controller) DeleteTestimonial(c *gin.Context) {
	deleteRow[models.Testimonial](h.db, "Testimonial deleted")(c)
}

func (h *Controller) SyncGoogleTestimonials(c *gin.Context) {
	apiKey, placeID := h.googleReviewSettings(c.Request.Context())
	if apiKey == "" || placeID == "" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Google Reviews API key and Place ID are required in admin settings"})
		return
	}
	endpoint := "https://maps.googleapis.com/maps/api/place/details/json"
	values := url.Values{}
	values.Set("place_id", placeID)
	values.Set("fields", "name,rating,reviews,url")
	values.Set("key", apiKey)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create Google request"})
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: "Failed to reach Google Places API"})
		return
	}
	defer resp.Body.Close()
	var payload googlePlaceDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: "Invalid Google Places response"})
		return
	}
	if payload.Status != "OK" {
		message := payload.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("Google Places returned %s", payload.Status)
		}
		c.JSON(http.StatusBadGateway, structs.Response{Success: false, Message: message})
		return
	}
	imported := 0
	for _, review := range payload.Result.Reviews {
		text := strings.TrimSpace(review.Text)
		if text == "" {
			continue
		}
		name := strings.TrimSpace(review.AuthorName)
		if name == "" {
			name = "Google reviewer"
		}
		externalID := googleReviewExternalID(placeID, review)
		row := models.Testimonial{
			Name:           name,
			Role:           "Google Review",
			Company:        payload.Result.Name,
			Avatar:         review.ProfilePhotoURL,
			ContentEn:      text,
			ContentID:      text,
			Rating:         review.Rating,
			ReviewSource:   "google",
			DisplayContext: "talent_bridge",
			ReviewStatus:   "approved",
			ExternalID:     externalID,
			SourceURL:      review.AuthorURL,
			IsActive:       true,
		}
		if row.Rating < 1 {
			row.Rating = 5
		}
		if err := h.db.WithContext(c.Request.Context()).Where(models.Testimonial{ReviewSource: "google", ExternalID: externalID}).Assign(row).FirstOrCreate(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save Google review"})
			return
		}
		imported++
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Google reviews synced", Data: gin.H{"imported": imported}})
}

func (h *Controller) UpdateTestimonialAvatar(c *gin.Context) {
	h.updateUploadField(c, h.uploadService.SaveTestimonialAvatar, &models.Testimonial{}, "avatar", "Avatar uploaded")
}

type googlePlaceDetailsResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Result       struct {
		Name    string              `json:"name"`
		URL     string              `json:"url"`
		Reviews []googlePlaceReview `json:"reviews"`
	} `json:"result"`
}

type googlePlaceReview struct {
	AuthorName      string `json:"author_name"`
	AuthorURL       string `json:"author_url"`
	ProfilePhotoURL string `json:"profile_photo_url"`
	Rating          int    `json:"rating"`
	Text            string `json:"text"`
	Time            int64  `json:"time"`
}

func (h *Controller) googleReviewSettings(ctx context.Context) (string, string) {
	values := map[string]string{}
	var rows []models.Setting
	_ = h.db.WithContext(ctx).Where("`key` IN ?", []string{"google_reviews_api_key", "google_reviews_place_id"}).Find(&rows).Error
	for _, row := range rows {
		values[row.Key] = strings.TrimSpace(row.Value)
	}
	apiKey := values["google_reviews_api_key"]
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_REVIEWS_API_KEY"))
	}
	placeID := values["google_reviews_place_id"]
	if placeID == "" {
		placeID = strings.TrimSpace(os.Getenv("GOOGLE_REVIEWS_PLACE_ID"))
	}
	return apiKey, placeID
}

func googleReviewExternalID(placeID string, review googlePlaceReview) string {
	hash := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%d|%s", placeID, review.AuthorName, review.Time, review.Text)))
	return hex.EncodeToString(hash[:])
}

func normalizeTestimonialReview(testimonial *models.Testimonial) {
	testimonial.ReviewSource = strings.TrimSpace(testimonial.ReviewSource)
	if testimonial.ReviewSource == "" {
		testimonial.ReviewSource = "student"
	}
	testimonial.DisplayContext = strings.TrimSpace(testimonial.DisplayContext)
	if testimonial.DisplayContext == "" {
		testimonial.DisplayContext = "general"
	}
	testimonial.ReviewStatus = strings.TrimSpace(testimonial.ReviewStatus)
	if testimonial.ReviewStatus == "" {
		testimonial.ReviewStatus = "approved"
	}
	if testimonial.Rating < 1 {
		testimonial.Rating = 1
	}
	if testimonial.Rating > 5 {
		testimonial.Rating = 5
	}
	testimonial.IsActive = testimonial.ReviewStatus == "approved" && testimonial.IsActive
	if testimonial.ReviewStatus == "approved" && !testimonial.IsActive {
		testimonial.IsActive = true
	}
	if testimonial.ReviewStatus != "approved" {
		testimonial.IsActive = false
	}
}

func (h *Controller) ListBanners(c *gin.Context) {
	listRows[models.Banner](h.db, "`order` asc, id asc", "Banners loaded")(c)
}

func (h *Controller) CreateBanner(c *gin.Context) {
	createRow[models.Banner](h.db, "Banner created")(c)
}

func (h *Controller) UpdateBanner(c *gin.Context) {
	updateRow[models.Banner](h.db, "Banner saved")(c)
}

func (h *Controller) DeleteBanner(c *gin.Context) {
	deleteRow[models.Banner](h.db, "Banner deleted")(c)
}

func (h *Controller) UpdateBannerImage(c *gin.Context) {
	h.updateUploadField(c, h.uploadService.SaveBannerImage, &models.Banner{}, "image_path", "Banner image uploaded")
}

func (h *Controller) ListMedia(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var rows []models.MediaFile
	query := h.db.WithContext(c.Request.Context()).Model(&models.MediaFile{}).Order("media_files.created_at desc")
	if user.IsAdmin {
		query = query.Joins("LEFT JOIN users ON users.id = media_files.uploaded_by").Where("media_files.uploaded_by = 0 OR users.is_admin = ?", true)
	} else {
		query = query.Where("uploaded_by = ?", user.ID)
	}
	if err := query.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load media"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Media files loaded", Data: rows})
}

func (h *Controller) CreateMedia(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Media file is required"})
		return
	}
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
		return
	}
	media, err := h.uploadService.SaveMediaFile(c.Request.Context(), userID.(uint), file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Media uploaded", Data: media})
}

func (h *Controller) DeleteMedia(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid media ID"})
		return
	}
	user := c.MustGet("user").(models.User)
	var media models.MediaFile
	query := h.db.WithContext(c.Request.Context()).Model(&models.MediaFile{}).Where("media_files.id = ?", id)
	if user.IsAdmin {
		query = query.Joins("LEFT JOIN users ON users.id = media_files.uploaded_by").Where("media_files.uploaded_by = 0 OR users.is_admin = ?", true)
	} else {
		query = query.Where("uploaded_by = ?", user.ID)
	}
	if err := query.First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Media not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load media"})
		return
	}
	if err := services.DeleteStoredPublicPath(c.Request.Context(), h.db, h.cfg, media.FilePath); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete stored media: " + err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&media).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete media"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Media deleted"})
}

func listRows[T any](db *gorm.DB, order string, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rows []T
		if err := db.WithContext(c.Request.Context()).Order(order).Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load data"})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message, Data: rows})
	}
}

func createRow[T any](db *gorm.DB, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var row T
		if err := c.ShouldBindJSON(&row); err != nil {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid payload"})
			return
		}
		if err := db.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create data"})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message, Data: row})
	}
}

func updateRow[T any](db *gorm.DB, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
			return
		}
		var row T
		if err := c.ShouldBindJSON(&row); err != nil {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid payload"})
			return
		}
		if err := db.WithContext(c.Request.Context()).Model(&row).Where("id = ?", uint(id)).Select("*").Omit("id", "created_at").Updates(row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save data"})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message, Data: row})
	}
}

func reorderRows[T any](db *gorm.DB, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req []struct {
			ID    uint `json:"id"`
			Order int  `json:"order"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid reorder payload"})
			return
		}
		for _, item := range req {
			if item.ID == 0 {
				continue
			}
			var row T
			if err := db.WithContext(c.Request.Context()).Model(&row).Where("id = ?", item.ID).Update("order", item.Order).Error; err != nil {
				c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reorder"})
				return
			}
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message})
	}
}

func deleteRow[T any](db *gorm.DB, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
			return
		}
		var row T
		if err := db.WithContext(c.Request.Context()).Unscoped().Delete(&row, uint(id)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete data"})
			return
		}
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: message})
	}
}
