package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) ListProductCategories(c *gin.Context) {
	listRows[models.ProductCategory](h.db, "name_en asc", "Product categories loaded")(c)
}

func (h *Controller) CreateProductCategory(c *gin.Context) {
	var category models.ProductCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid category payload"})
		return
	}
	if strings.TrimSpace(category.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "product_categories", category.NameEn, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		category.Slug = slug
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create category"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product category created", Data: category})
}

func (h *Controller) UpdateProductCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid category id"})
		return
	}
	var category models.ProductCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid category payload"})
		return
	}
	if strings.TrimSpace(category.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "product_categories", category.NameEn, uint(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		category.Slug = slug
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.ProductCategory{}).Where("id = ?", uint(id)).Select("name_en", "name_id", "slug", "requires_booking_time").Updates(category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save category"})
		return
	}
	category.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product category saved", Data: category})
}

func (h *Controller) DeleteProductCategory(c *gin.Context) {
	deleteRow[models.ProductCategory](h.db, "Product category deleted")(c)
}

func (h *Controller) ListProducts(c *gin.Context) {
	var products []models.Product
	query := h.db.WithContext(c.Request.Context()).Model(&models.Product{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where("title_en LIKE ? OR title_id LIKE ? OR slug LIKE ?", like, like, like)
	}
	if productType := strings.TrimSpace(c.Query("type")); productType != "" && productType != "all" {
		query = query.Where("type = ?", productType)
	}
	if status := strings.TrimSpace(c.Query("status")); status == "published" {
		query = query.Where("is_published = ?", true)
	} else if status == "draft" {
		query = query.Where("is_published = ?", false)
	}
	if err := query.Order("created_at desc").Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load products"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Products loaded", Data: products})
}

func (h *Controller) CreateProduct(c *gin.Context) {
	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid product payload"})
		return
	}
	if strings.TrimSpace(product.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "products", product.TitleEn, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		product.Slug = slug
	}
	if err := h.applyProductCategoryType(c.Request.Context(), &product); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create product"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product created", Data: product})
}

func (h *Controller) UpdateProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid product id"})
		return
	}
	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid product payload"})
		return
	}
	if strings.TrimSpace(product.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "products", product.TitleEn, uint(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		product.Slug = slug
	}
	if err := h.applyProductCategoryType(c.Request.Context(), &product); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Product{}).Where("id = ?", uint(id)).Select(
		"title_en", "title_id", "slug", "description_en", "description_id", "thumbnail", "price", "type", "category_id", "included_en", "included_id", "faq_en", "faq_id", "is_popular", "is_published",
	).Updates(product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save product"})
		return
	}
	product.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product saved", Data: product})
}

func (h *Controller) applyProductCategoryType(ctx context.Context, product *models.Product) error {
	if product.CategoryID == 0 {
		return fmt.Errorf("Product category is required")
	}
	var category models.ProductCategory
	if err := h.db.WithContext(ctx).First(&category, product.CategoryID).Error; err != nil {
		return fmt.Errorf("Product category is invalid")
	}
	product.Type = category.Slug
	return nil
}

func (h *Controller) DeleteProduct(c *gin.Context) {
	deleteRow[models.Product](h.db, "Product deleted")(c)
}

func (h *Controller) UpdateProductThumbnail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	file, fileErr := c.FormFile("file")
	if err != nil || id == 0 || fileErr != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Thumbnail file is required"})
		return
	}
	path, err := h.uploadService.SaveProductThumbnail(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Product{}).Where("id = ?", uint(id)).Update("thumbnail", path).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save thumbnail"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Thumbnail uploaded", Data: gin.H{"thumbnail": path}})
}

func (h *Controller) ListProductFiles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid product id"})
		return
	}
	var files []models.ProductFile
	if err := h.db.WithContext(c.Request.Context()).Where("product_id = ?", uint(id)).Order("id asc").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load product files"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product files loaded", Data: files})
}

func (h *Controller) CreateProductFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	file, fileErr := c.FormFile("file")
	if err != nil || id == 0 || fileErr != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Product file is required"})
		return
	}
	path, name, err := h.uploadService.SaveProductFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	row := models.ProductFile{ProductID: uint(id), FilePath: path, FileName: name}
	if err := h.db.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save product file"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product file uploaded", Data: row})
}

func (h *Controller) DeleteProductFile(c *gin.Context) {
	deleteRow[models.ProductFile](h.db, "Product file deleted")(c)
}

func (h *Controller) ListTransactions(c *gin.Context) {
	_ = services.CancelExpiredPendingOrders(c.Request.Context(), h.db)
	type row struct {
		ID              uint      `json:"id"`
		OrderID         uint      `json:"order_id"`
		UserName        string    `json:"user_name"`
		UserEmail       string    `json:"user_email"`
		Item            string    `json:"item"`
		ItemType        string    `json:"item_type"`
		Amount          int       `json:"amount"`
		Status          string    `json:"status"`
		MidtransOrderID string    `json:"midtrans_order_id"`
		CreatedAt       time.Time `json:"created_at"`
	}
	var rows []row
	if err := h.db.WithContext(c.Request.Context()).Raw(`
			SELECT o.id, o.id AS order_id, u.name AS user_name, u.email AS user_email,
				COALESCE(c.title_en, p.title_en, o.midtrans_order_id) AS item,
				oi.item_type, o.total_amount AS amount, o.status, o.midtrans_order_id, o.created_at
			FROM orders o
			JOIN users u ON u.id = o.user_id
			LEFT JOIN order_items oi ON oi.order_id = o.id
			LEFT JOIN courses c ON oi.item_type = 'course' AND c.id = oi.item_id
			LEFT JOIN products p ON oi.item_type = 'product' AND p.id = oi.item_id
			ORDER BY o.created_at DESC
			LIMIT 100
		`).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load transactions"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Transactions loaded", Data: rows})
}

func (h *Controller) ListCoupons(c *gin.Context) {
	_ = services.ReconcileCouponUsageCounts(c.Request.Context(), h.db)
	var rows []models.Coupon
	if err := h.db.WithContext(c.Request.Context()).Order("created_at desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load coupons"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Coupons loaded", Data: rows})
}

func (h *Controller) CreateCoupon(c *gin.Context) {
	createRow[models.Coupon](h.db, "Coupon created")(c)
}

func (h *Controller) UpdateCoupon(c *gin.Context) {
	updateRow[models.Coupon](h.db, "Coupon saved")(c)
}

func (h *Controller) DeleteCoupon(c *gin.Context) {
	deleteRow[models.Coupon](h.db, "Coupon deleted")(c)
}
