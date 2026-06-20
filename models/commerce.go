package models

import "time"

type Product struct {
	BaseModel
	TitleEn       string `gorm:"size:191;not null" json:"title_en"`
	TitleID       string `gorm:"size:191;not null" json:"title_id"`
	Slug          string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
	DescriptionEn string `gorm:"type:longtext" json:"description_en"`
	DescriptionID string `gorm:"type:longtext" json:"description_id"`
	Thumbnail     string `gorm:"size:255" json:"thumbnail"`
	Price         int    `gorm:"not null;default:0" json:"price"`
	Type          string `gorm:"size:30;not null;index" json:"type"`
	IncludedEn    string `gorm:"type:text" json:"included_en"`
	IncludedID    string `gorm:"type:text" json:"included_id"`
	FAQEn         string `gorm:"type:longtext" json:"faq_en"`
	FAQID         string `gorm:"type:longtext" json:"faq_id"`
	IsPopular     bool   `gorm:"not null;default:false;index" json:"is_popular"`
	IsPublished   bool   `gorm:"not null;default:false;index" json:"is_published"`
	CategoryID    uint   `gorm:"index" json:"category_id"`
}

type ProductCategory struct {
	BaseModel
	NameEn              string `gorm:"size:191;not null" json:"name_en"`
	NameID              string `gorm:"size:191;not null" json:"name_id"`
	Slug                string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
	RequiresBookingTime bool   `gorm:"not null;default:false;index" json:"requires_booking_time"`
	ShowInKnowledgeBase bool   `gorm:"not null;default:false;index" json:"show_in_knowledge_base"`
}

type ProductFile struct {
	BaseModel
	ProductID uint   `gorm:"not null;index" json:"product_id"`
	FilePath  string `gorm:"size:255;not null" json:"file_path"`
	FileName  string `gorm:"size:191;not null" json:"file_name"`
}

type Order struct {
	BaseModel
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	TotalAmount      int        `gorm:"not null;default:0" json:"total_amount"`
	Status           string     `gorm:"size:30;not null;default:'pending';index" json:"status"`
	MidtransOrderID  string     `gorm:"size:191;uniqueIndex" json:"midtrans_order_id"`
	AppliedCouponID  uint       `gorm:"index" json:"applied_coupon_id"`
	SnapToken        string     `gorm:"size:255" json:"snap_token"`
	SnapRedirectURL  string     `gorm:"size:500" json:"snap_redirect_url"`
	PaymentExpiresAt *time.Time `gorm:"index" json:"payment_expires_at"`
	PaidAt           *time.Time `json:"paid_at"`
}

type OrderItem struct {
	BaseModel
	OrderID  uint   `gorm:"not null;index" json:"order_id"`
	ItemType string `gorm:"size:20;not null;index:idx_order_item" json:"item_type"`
	ItemID   uint   `gorm:"not null;index:idx_order_item" json:"item_id"`
	Price    int    `gorm:"not null;default:0" json:"price"`
}

type Transaction struct {
	BaseModel
	OrderID               uint   `gorm:"not null;index" json:"order_id"`
	MidtransTransactionID string `gorm:"size:191;uniqueIndex" json:"midtrans_transaction_id"`
	PaymentType           string `gorm:"size:50" json:"payment_type"`
	Status                string `gorm:"size:30;not null;index" json:"status"`
	RawResponse           string `gorm:"type:longtext" json:"raw_response"`
}

type Coupon struct {
	BaseModel
	Code          string     `gorm:"size:50;not null;uniqueIndex" json:"code"`
	DiscountType  string     `gorm:"size:20;not null" json:"discount_type"`
	DiscountValue int        `gorm:"not null;default:0" json:"discount_value"`
	MaxUses       int        `gorm:"not null;default:0" json:"max_uses"`
	UsedCount     int        `gorm:"not null;default:0" json:"used_count"`
	ExpiresAt     *time.Time `json:"expires_at"`
	AppliesTo     string     `gorm:"size:20;not null;default:'all'" json:"applies_to"`
}

type CouponUsage struct {
	BaseModel
	CouponID uint      `gorm:"not null;index" json:"coupon_id"`
	UserID   uint      `gorm:"not null;index" json:"user_id"`
	OrderID  uint      `gorm:"not null;index" json:"order_id"`
	UsedAt   time.Time `gorm:"not null" json:"used_at"`
}

type Invoice struct {
	BaseModel
	OrderID           uint       `gorm:"not null;uniqueIndex" json:"order_id"`
	InvoiceNumber     string     `gorm:"size:191;not null;uniqueIndex" json:"invoice_number"`
	FilePath          string     `gorm:"size:700" json:"file_path"`
	IssuedAt          time.Time  `gorm:"not null" json:"issued_at"`
	TemplateID        uint       `gorm:"index" json:"template_id"`
	TemplateVersionID uint       `gorm:"index" json:"template_version_id"`
	Locale            string     `gorm:"size:5;not null;default:'en'" json:"locale"`
	SnapshotJSON      string     `gorm:"type:longtext" json:"-"`
	SnapshotChecksum  string     `gorm:"size:64" json:"snapshot_checksum"`
	CachedObjectKey   string     `gorm:"size:700" json:"cached_object_key"`
	CacheExpiresAt    *time.Time `gorm:"index" json:"cache_expires_at"`
}

type ConsultationSlot struct {
	BaseModel
	OwnerID     uint      `gorm:"not null;default:0;index" json:"owner_id"`
	Date        time.Time `gorm:"type:date;not null;index" json:"date"`
	TimeStart   string    `gorm:"size:10;not null" json:"time_start"`
	TimeEnd     string    `gorm:"size:10;not null" json:"time_end"`
	Capacity    int       `gorm:"not null;default:1" json:"capacity"`
	IsAvailable bool      `gorm:"not null;default:true;index" json:"is_available"`
}

type ConsultationBooking struct {
	BaseModel
	SlotID          uint   `gorm:"not null;index" json:"slot_id"`
	UserID          uint   `gorm:"not null;index" json:"user_id"`
	OrderID         uint   `gorm:"not null;index" json:"order_id"`
	Status          string `gorm:"size:30;not null;default:'pending';index" json:"status"`
	Notes           string `gorm:"type:text" json:"notes"`
	RescheduleCount int    `gorm:"not null;default:0" json:"reschedule_count"`
}
