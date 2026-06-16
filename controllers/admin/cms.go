package admin

import (
	"net/http"
	"strconv"
	"strings"

	"academyprometheus/backend/models"
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
	createRow[models.Testimonial](h.db, "Testimonial created")(c)
}

func (h *Controller) UpdateTestimonial(c *gin.Context) {
	updateRow[models.Testimonial](h.db, "Testimonial saved")(c)
}

func (h *Controller) DeleteTestimonial(c *gin.Context) {
	deleteRow[models.Testimonial](h.db, "Testimonial deleted")(c)
}

func (h *Controller) UpdateTestimonialAvatar(c *gin.Context) {
	h.updateUploadField(c, h.uploadService.SaveTestimonialAvatar, &models.Testimonial{}, "avatar", "Avatar uploaded")
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
	listRows[models.MediaFile](h.db, "created_at desc", "Media files loaded")(c)
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
	deleteRow[models.MediaFile](h.db, "Media deleted")(c)
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
