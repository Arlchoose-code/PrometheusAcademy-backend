package structs

import "time"

type ProductRequest struct {
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	Slug          string `json:"slug"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	Thumbnail     string `json:"thumbnail"`
	Price         int    `json:"price"`
	Type          string `json:"type"`
	IncludedEn    string `json:"included_en"`
	IncludedID    string `json:"included_id"`
	FAQEn         string `json:"faq_en"`
	FAQID         string `json:"faq_id"`
	IsPopular     bool   `json:"is_popular"`
	IsPublished   bool   `json:"is_published"`
	CategoryID    uint   `json:"category_id"`
}

type ProductResponse struct {
	ModelResponse
	ProductRequest
}

type ProductCategoryRequest struct {
	NameEn              string `json:"name_en"`
	NameID              string `json:"name_id"`
	Slug                string `json:"slug"`
	RequiresBookingTime bool   `json:"requires_booking_time"`
}

type ProductCategoryResponse struct {
	ModelResponse
	ProductCategoryRequest
}

type ProductFileRequest struct {
	ProductID uint   `json:"product_id"`
	FilePath  string `json:"file_path"`
	FileName  string `json:"file_name"`
}

type ProductFileResponse struct {
	ModelResponse
	ProductFileRequest
}

type OrderRequest struct {
	UserID           uint       `json:"user_id"`
	TotalAmount      int        `json:"total_amount"`
	Status           string     `json:"status"`
	MidtransOrderID  string     `json:"midtrans_order_id"`
	AppliedCouponID  uint       `json:"applied_coupon_id"`
	SnapToken        string     `json:"snap_token"`
	SnapRedirectURL  string     `json:"snap_redirect_url"`
	PaymentExpiresAt *time.Time `json:"payment_expires_at"`
	PaidAt           *time.Time `json:"paid_at"`
}

type OrderResponse struct {
	ModelResponse
	OrderRequest
}

type OrderItemRequest struct {
	OrderID  uint   `json:"order_id"`
	ItemType string `json:"item_type"`
	ItemID   uint   `json:"item_id"`
	Price    int    `json:"price"`
}

type OrderItemResponse struct {
	ModelResponse
	OrderItemRequest
}

type TransactionRequest struct {
	OrderID               uint   `json:"order_id"`
	MidtransTransactionID string `json:"midtrans_transaction_id"`
	PaymentType           string `json:"payment_type"`
	Status                string `json:"status"`
	RawResponse           string `json:"raw_response"`
}

type TransactionResponse struct {
	ModelResponse
	TransactionRequest
}

type MidtransWebhookRequest struct {
	OrderID       string `json:"order_id"`
	StatusCode    string `json:"status_code"`
	GrossAmount   string `json:"gross_amount"`
	SignatureKey  string `json:"signature_key"`
	TransactionID string `json:"transaction_id"`
	PaymentType   string `json:"payment_type"`
	Status        string `json:"transaction_status"`
}

type CouponRequest struct {
	Code          string     `json:"code"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue int        `json:"discount_value"`
	MaxUses       int        `json:"max_uses"`
	UsedCount     int        `json:"used_count"`
	ExpiresAt     *time.Time `json:"expires_at"`
	AppliesTo     string     `json:"applies_to"`
}

type CouponResponse struct {
	ModelResponse
	CouponRequest
}

type PurchaseRequest struct {
	CouponCode string `json:"coupon_code"`
}

type CouponApplyRequest struct {
	Code   string `json:"code"`
	Amount int    `json:"amount"`
	Scope  string `json:"scope"`
}

type CouponUsageRequest struct {
	CouponID uint      `json:"coupon_id"`
	UserID   uint      `json:"user_id"`
	OrderID  uint      `json:"order_id"`
	UsedAt   time.Time `json:"used_at"`
}

type CouponUsageResponse struct {
	ModelResponse
	CouponUsageRequest
}

type InvoiceRequest struct {
	OrderID       uint      `json:"order_id"`
	InvoiceNumber string    `json:"invoice_number"`
	FilePath      string    `json:"file_path"`
	IssuedAt      time.Time `json:"issued_at"`
}

type InvoiceResponse struct {
	ModelResponse
	InvoiceRequest
}

type ConsultationSlotRequest struct {
	Date        time.Time `json:"date"`
	TimeStart   string    `json:"time_start"`
	TimeEnd     string    `json:"time_end"`
	IsAvailable bool      `json:"is_available"`
}

type ConsultationSlotResponse struct {
	ModelResponse
	ConsultationSlotRequest
}

type ConsultationBookingRequest struct {
	SlotID          uint   `json:"slot_id"`
	UserID          uint   `json:"user_id"`
	OrderID         uint   `json:"order_id"`
	Status          string `json:"status"`
	Notes           string `json:"notes"`
	RescheduleCount int    `json:"reschedule_count"`
}

type ConsultationBookingResponse struct {
	ModelResponse
	ConsultationBookingRequest
}

type BookConsultationSlotRequest struct {
	OrderID uint   `json:"order_id"`
	Notes   string `json:"notes"`
}

type UpdateConsultationBookingRequest struct {
	Status string `json:"status"`
	SlotID uint   `json:"slot_id"`
	Notes  string `json:"notes"`
}
