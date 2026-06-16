package public

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) ListProductCategories(c *gin.Context) {
	var categories []models.ProductCategory
	if err := h.db.WithContext(c.Request.Context()).Order("name_en asc").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load product categories"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product categories loaded", Data: categories})
}

func (h *Controller) ListProducts(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	perPage := positiveInt(c.Query("per_page"), 9)
	if perPage > 24 {
		perPage = 24
	}
	query := h.db.WithContext(c.Request.Context()).Model(&models.Product{}).Where("is_published = ?", true)
	categorySlug := strings.TrimSpace(c.Query("category"))
	if categorySlug == "" || categorySlug == "all" {
		categorySlug = strings.TrimSpace(c.Query("type"))
	}
	if categorySlug != "" && categorySlug != "all" {
		var category models.ProductCategory
		if err := h.db.WithContext(c.Request.Context()).Where("slug = ?", categorySlug).First(&category).Error; err == nil {
			query = query.Where("category_id = ?", category.ID)
		} else {
			query = query.Where("type = ?", categorySlug)
		}
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where("title_en LIKE ? OR title_id LIKE ? OR description_en LIKE ? OR description_id LIKE ?", like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count products"})
		return
	}
	order := "created_at desc"
	if c.Query("sort") == "price_asc" {
		order = "price asc, created_at desc"
	} else if c.Query("sort") == "price_desc" {
		order = "price desc, created_at desc"
	}
	var products []models.Product
	if err := query.Order(order).Limit(perPage).Offset((page - 1) * perPage).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load products"})
		return
	}
	items, err := productSummaries(c.Request.Context(), h.db, products)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to format products"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Products loaded", Data: gin.H{"items": items, "page": page, "total_pages": totalPages(total, perPage), "total": total}})
}

func (h *Controller) GetProduct(c *gin.Context) {
	var product models.Product
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND is_published = ?", strings.TrimSpace(c.Param("slug")), true).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Product not found"})
		return
	}
	var reviews []models.Review
	_ = h.db.WithContext(c.Request.Context()).Where("reviewable_type = ? AND reviewable_id = ?", "product", product.ID).Order("created_at desc").Limit(10).Find(&reviews).Error
	reviewItems, err := formatReviews(c.Request.Context(), h.db, reviews)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to format reviews"})
		return
	}
	productPayload, err := productSummary(c.Request.Context(), h.db, product)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to format product"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Product loaded", Data: gin.H{"product": productPayload, "reviews": reviewItems}})
}

func (h *Controller) CreateProductPurchase(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	_ = services.CancelExpiredPendingOrders(c.Request.Context(), h.db)
	var req structs.PurchaseRequest
	_ = c.ShouldBindJSON(&req)
	var product models.Product
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND is_published = ?", strings.TrimSpace(c.Param("slug")), true).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Product not found"})
		return
	}
	amount, coupon, err := discountedAmount(c.Request.Context(), h.db, product.Price, req.CouponCode, "product")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if existingOrder, ok := services.PendingOrderForItem(c.Request.Context(), h.db, user.ID, "product", product.ID); ok && coupon == nil {
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Pending payment loaded", Data: services.OrderPaymentResponse(existingOrder, false)})
		return
	}
	orderID := fmt.Sprintf("PROM-PROD-%d-%d", user.ID, time.Now().UnixNano())
	order := models.Order{UserID: user.ID, TotalAmount: amount, Status: "pending", MidtransOrderID: orderID}
	if coupon != nil {
		order.AppliedCouponID = coupon.ID
	}
	if amount == 0 || h.cfg.MidtransServerKey == "" {
		order.Status = "success"
	}
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.OrderItem{OrderID: order.ID, ItemType: "product", ItemID: product.ID, Price: amount}).Error; err != nil {
			return err
		}
		if order.Status == "success" {
			if err := services.FulfillSuccessfulOrder(c.Request.Context(), tx, order); err != nil {
				return err
			}
			_, err := services.EnsureInvoice(c.Request.Context(), tx, h.cfg, order)
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create order"})
		return
	}
	data := services.OrderPaymentResponse(order, false)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Payment order created", Data: data})
}

func (h *Controller) GetProductReviewEligibility(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var product models.Product
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND is_published = ?", strings.TrimSpace(c.Param("slug")), true).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Product not found"})
		return
	}
	purchased := user.IsAdmin || hasSuccessfulOrderItem(c.Request.Context(), h.db, user.ID, "product", product.ID)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Review eligibility loaded", Data: gin.H{"can_review": purchased, "has_purchased": purchased}})
}

func (h *Controller) CreateProductReview(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var product models.Product
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND is_published = ?", strings.TrimSpace(c.Param("slug")), true).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Product not found"})
		return
	}
	if !user.IsAdmin && !hasSuccessfulOrderItem(c.Request.Context(), h.db, user.ID, "product", product.ID) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Purchase this product before leaving a review"})
		return
	}

	review, err := saveReview(c.Request.Context(), h.db, user.ID, product.ID, "product", c)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Review saved", Data: review})
}

func (h *Controller) ApplyCoupon(c *gin.Context) {
	var req structs.CouponApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid coupon payload"})
		return
	}
	amount, coupon, err := discountedAmount(c.Request.Context(), h.db, req.Amount, req.Code, req.Scope)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Coupon applied", Data: gin.H{"amount": amount, "discount": req.Amount - amount, "code": coupon.Code}})
}

func productSummaries(ctx context.Context, db *gorm.DB, products []models.Product) ([]gin.H, error) {
	items := make([]gin.H, 0, len(products))
	for _, product := range products {
		item, err := productSummary(ctx, db, product)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func productSummary(ctx context.Context, db *gorm.DB, product models.Product) (gin.H, error) {
	var category models.ProductCategory
	_ = db.WithContext(ctx).First(&category, product.CategoryID).Error
	var ratingRow struct {
		Average float64
		Count   int64
	}
	if err := db.WithContext(ctx).
		Model(&models.Review{}).
		Select("COALESCE(AVG(rating), 0) AS average, COUNT(*) AS count").
		Where("reviewable_type = ? AND reviewable_id = ?", "product", product.ID).
		Scan(&ratingRow).Error; err != nil {
		return nil, err
	}
	return gin.H{
		"id":                    product.ID,
		"title_en":              product.TitleEn,
		"title_id":              product.TitleID,
		"slug":                  product.Slug,
		"description_en":        product.DescriptionEn,
		"description_id":        product.DescriptionID,
		"thumbnail":             product.Thumbnail,
		"price":                 product.Price,
		"type":                  product.Type,
		"included_en":           product.IncludedEn,
		"included_id":           product.IncludedID,
		"faq_en":                product.FAQEn,
		"faq_id":                product.FAQID,
		"is_popular":            product.IsPopular,
		"is_published":          product.IsPublished,
		"category_id":           product.CategoryID,
		"category_slug":         category.Slug,
		"category_name_en":      category.NameEn,
		"category_name_id":      category.NameID,
		"requires_booking_time": category.RequiresBookingTime,
		"rating":                ratingRow.Average,
		"reviews_count":         ratingRow.Count,
	}, nil
}

func hasSuccessfulOrderItem(ctx context.Context, db *gorm.DB, userID uint, itemType string, itemID uint) bool {
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.OrderItem{}).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.user_id = ? AND orders.status = ? AND order_items.item_type = ? AND order_items.item_id = ?", userID, "success", itemType, itemID).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func discountedAmount(ctx context.Context, db *gorm.DB, amount int, code string, scope string) (int, *models.Coupon, error) {
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

func saveReview(ctx context.Context, db *gorm.DB, userID uint, reviewableID uint, reviewableType string, c *gin.Context) (models.Review, error) {
	var req structs.ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Review{}, errors.New("Invalid review payload")
	}
	comment := strings.TrimSpace(req.Comment)
	if req.Rating < 1 || req.Rating > 5 {
		return models.Review{}, errors.New("Rating must be between 1 and 5")
	}
	if comment == "" {
		return models.Review{}, errors.New("Review comment is required")
	}
	review := models.Review{
		UserID:         userID,
		ReviewableID:   reviewableID,
		ReviewableType: reviewableType,
		Rating:         req.Rating,
		Comment:        comment,
	}
	err := db.WithContext(ctx).
		Where(models.Review{UserID: userID, ReviewableID: reviewableID, ReviewableType: reviewableType}).
		Assign(review).
		FirstOrCreate(&review).Error
	return review, err
}
