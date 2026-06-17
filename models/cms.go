package models

import "time"

type ContactLead struct {
	BaseModel
	Name        string `gorm:"size:191;not null" json:"name"`
	Email       string `gorm:"size:191;not null;index" json:"email"`
	Subject     string `gorm:"size:191;not null" json:"subject"`
	Message     string `gorm:"type:text;not null" json:"message"`
	GDPRConsent bool   `gorm:"not null;default:false" json:"gdpr_consent"`
	Status      string `gorm:"size:30;not null;default:'new';index" json:"status"`
}

type LeadNote struct {
	BaseModel
	LeadID    uint   `gorm:"not null;index:idx_lead" json:"lead_id"`
	LeadType  string `gorm:"size:30;not null;index:idx_lead" json:"lead_type"`
	Note      string `gorm:"type:text;not null" json:"note"`
	CreatedBy uint   `gorm:"not null;index" json:"created_by"`
}

type NewsletterSubscriber struct {
	BaseModel
	FullName     string    `gorm:"size:191;not null" json:"full_name"`
	Email        string    `gorm:"size:191;not null;uniqueIndex" json:"email"`
	GDPRConsent  bool      `gorm:"not null;default:false" json:"gdpr_consent"`
	SubscribedAt time.Time `gorm:"not null" json:"subscribed_at"`
}

type Setting struct {
	BaseModel
	Key   string `gorm:"size:191;not null;uniqueIndex" json:"key"`
	Value string `gorm:"type:longtext" json:"value"`
}

type SEOMeta struct {
	BaseModel
	PageSlug      string `gorm:"size:191;not null;uniqueIndex" json:"page_slug"`
	TitleEn       string `gorm:"size:191;not null" json:"title_en"`
	TitleID       string `gorm:"size:191;not null" json:"title_id"`
	DescriptionEn string `gorm:"type:text" json:"description_en"`
	DescriptionID string `gorm:"type:text" json:"description_id"`
	OGImage       string `gorm:"size:255" json:"og_image"`
}

type Page struct {
	BaseModel
	Slug          string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
	TitleEn       string `gorm:"size:191;not null" json:"title_en"`
	TitleID       string `gorm:"size:191;not null" json:"title_id"`
	DescriptionEn string `gorm:"type:text" json:"description_en"`
	DescriptionID string `gorm:"type:text" json:"description_id"`
	ImagePath     string `gorm:"size:255" json:"image_path"`
	ContentEn     string `gorm:"type:longtext" json:"content_en"`
	ContentID     string `gorm:"type:longtext" json:"content_id"`
}

type FAQ struct {
	BaseModel
	QuestionEn string `gorm:"type:text;not null" json:"question_en"`
	QuestionID string `gorm:"type:text;not null" json:"question_id"`
	AnswerEn   string `gorm:"type:longtext;not null" json:"answer_en"`
	AnswerID   string `gorm:"type:longtext;not null" json:"answer_id"`
	Order      int    `gorm:"not null;default:0;index" json:"order"`
}

type Testimonial struct {
	BaseModel
	Name      string `gorm:"size:191;not null" json:"name"`
	Role      string `gorm:"size:191" json:"role"`
	Company   string `gorm:"size:191" json:"company"`
	Avatar    string `gorm:"size:255" json:"avatar"`
	ContentEn string `gorm:"type:text;not null" json:"content_en"`
	ContentID string `gorm:"type:text;not null" json:"content_id"`
	Rating    int    `gorm:"not null;default:5" json:"rating"`
	IsActive  bool   `gorm:"not null;default:true;index" json:"is_active"`
}

type Banner struct {
	BaseModel
	TitleEn   string `gorm:"size:191;not null" json:"title_en"`
	TitleID   string `gorm:"size:191;not null" json:"title_id"`
	ImagePath string `gorm:"size:255;not null" json:"image_path"`
	LinkURL   string `gorm:"size:255" json:"link_url"`
	IsActive  bool   `gorm:"not null;default:true;index" json:"is_active"`
	Order     int    `gorm:"not null;default:0;index" json:"order"`
}

type MediaFile struct {
	BaseModel
	FilePath   string `gorm:"size:255;not null" json:"file_path"`
	FileName   string `gorm:"size:191;not null" json:"file_name"`
	FileType   string `gorm:"size:80;not null" json:"file_type"`
	FileSize   int64  `gorm:"not null;default:0" json:"file_size"`
	UploadedBy uint   `gorm:"not null;index" json:"uploaded_by"`
}

type EmailTemplate struct {
	BaseModel
	DesignID        uint   `gorm:"not null;default:0;index" json:"design_id"`
	Key             string `gorm:"size:191;not null;uniqueIndex" json:"key"`
	SubjectEn       string `gorm:"size:191;not null" json:"subject_en"`
	SubjectID       string `gorm:"size:191;not null" json:"subject_id"`
	PreheaderEn     string `gorm:"size:255" json:"preheader_en"`
	PreheaderID     string `gorm:"size:255" json:"preheader_id"`
	BodyEn          string `gorm:"type:longtext;not null" json:"body_en"`
	BodyID          string `gorm:"type:longtext;not null" json:"body_id"`
	DesignJSON      string `gorm:"type:longtext" json:"design_json"`
	DesignJSONEn    string `gorm:"type:longtext" json:"design_json_en"`
	DesignJSONID    string `gorm:"type:longtext" json:"design_json_id"`
	FooterEn        string `gorm:"type:longtext" json:"footer_en"`
	FooterID        string `gorm:"type:longtext" json:"footer_id"`
	SenderName      string `gorm:"size:191" json:"sender_name"`
	SenderEmail     string `gorm:"size:191" json:"sender_email"`
	BackgroundColor string `gorm:"size:30;not null;default:'#F8F9FA'" json:"background_color"`
	AccentColor     string `gorm:"size:30;not null;default:'#C9A84C'" json:"accent_color"`
}

type EmailDesign struct {
	BaseModel
	Name            string `gorm:"size:191;not null" json:"name"`
	Description     string `gorm:"type:text" json:"description"`
	BackgroundColor string `gorm:"size:30;not null;default:'#F8F9FA'" json:"background_color"`
	ContentColor    string `gorm:"size:30;not null;default:'#FFFFFF'" json:"content_color"`
	AccentColor     string `gorm:"size:30;not null;default:'#C9A84C'" json:"accent_color"`
	TextColor       string `gorm:"size:30;not null;default:'#212529'" json:"text_color"`
	Width           int    `gorm:"not null;default:620" json:"width"`
	BlocksJSON      string `gorm:"type:longtext" json:"blocks_json"`
	IsDefault       bool   `gorm:"not null;default:false;index" json:"is_default"`
}

type EmailCampaign struct {
	BaseModel
	DesignID           uint       `gorm:"not null;default:0;index" json:"design_id"`
	TemplateKey        string     `gorm:"size:191;index" json:"template_key"`
	Name               string     `gorm:"size:191" json:"name"`
	Subject            string     `gorm:"size:191;not null" json:"subject"`
	SubjectEn          string     `gorm:"size:191" json:"subject_en"`
	SubjectID          string     `gorm:"size:191" json:"subject_id"`
	HTML               string     `gorm:"type:longtext;not null" json:"html"`
	HTMLEn             string     `gorm:"type:longtext" json:"html_en"`
	HTMLID             string     `gorm:"type:longtext" json:"html_id"`
	Text               string     `gorm:"type:longtext" json:"text"`
	TextEn             string     `gorm:"type:longtext" json:"text_en"`
	TextID             string     `gorm:"type:longtext" json:"text_id"`
	Target             string     `gorm:"size:30;not null;index" json:"target"`
	UserIDsJSON        string     `gorm:"type:longtext" json:"user_ids_json"`
	SenderName         string     `gorm:"size:191" json:"sender_name"`
	SenderEmail        string     `gorm:"size:191" json:"sender_email"`
	RateLimitPerMinute int        `gorm:"not null;default:30" json:"rate_limit_per_minute"`
	Status             string     `gorm:"size:30;not null;default:'draft';index" json:"status"`
	RecipientCount     int        `gorm:"not null;default:0" json:"recipient_count"`
	SentCount          int        `gorm:"not null;default:0" json:"sent_count"`
	FailedCount        int        `gorm:"not null;default:0" json:"failed_count"`
	FailedJSON         string     `gorm:"type:longtext" json:"failed_json"`
	CreatedBy          uint       `gorm:"not null;default:0;index" json:"created_by"`
	QueuedAt           *time.Time `json:"queued_at"`
	StartedAt          *time.Time `json:"started_at"`
	FinishedAt         *time.Time `json:"finished_at"`
}
