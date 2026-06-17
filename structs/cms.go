package structs

import "time"

type ContactLeadRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Subject     string `json:"subject"`
	Message     string `json:"message"`
	GDPRConsent bool   `json:"gdpr_consent"`
	Status      string `json:"status"`
}

type ContactLeadResponse struct {
	ModelResponse
	ContactLeadRequest
}

type LeadNoteRequest struct {
	LeadID    uint   `json:"lead_id"`
	LeadType  string `json:"lead_type"`
	Note      string `json:"note"`
	CreatedBy uint   `json:"created_by"`
}

type LeadNoteResponse struct {
	ModelResponse
	LeadNoteRequest
}

type NewsletterSubscriberRequest struct {
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	GDPRConsent  bool      `json:"gdpr_consent"`
	SubscribedAt time.Time `json:"subscribed_at"`
}

type NewsletterSubscriberResponse struct {
	ModelResponse
	NewsletterSubscriberRequest
}

type SettingRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SettingResponse struct {
	ModelResponse
	SettingRequest
}

type SEOMetaRequest struct {
	PageSlug      string `json:"page_slug"`
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	OGImage       string `json:"og_image"`
}

type SEOMetaResponse struct {
	ModelResponse
	SEOMetaRequest
}

type PageRequest struct {
	Slug          string `json:"slug"`
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	ImagePath     string `json:"image_path"`
	ContentEn     string `json:"content_en"`
	ContentID     string `json:"content_id"`
}

type PageResponse struct {
	ModelResponse
	PageRequest
}

type FAQRequest struct {
	QuestionEn string `json:"question_en"`
	QuestionID string `json:"question_id"`
	AnswerEn   string `json:"answer_en"`
	AnswerID   string `json:"answer_id"`
	Order      int    `json:"order"`
}

type FAQResponse struct {
	ModelResponse
	FAQRequest
}

type TestimonialRequest struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Company   string `json:"company"`
	Avatar    string `json:"avatar"`
	ContentEn string `json:"content_en"`
	ContentID string `json:"content_id"`
	Rating    int    `json:"rating"`
	IsActive  bool   `json:"is_active"`
}

type TestimonialResponse struct {
	ModelResponse
	TestimonialRequest
}

type BannerRequest struct {
	TitleEn   string `json:"title_en"`
	TitleID   string `json:"title_id"`
	ImagePath string `json:"image_path"`
	LinkURL   string `json:"link_url"`
	IsActive  bool   `json:"is_active"`
	Order     int    `json:"order"`
}

type BannerResponse struct {
	ModelResponse
	BannerRequest
}

type MediaFileRequest struct {
	FilePath   string `json:"file_path"`
	FileName   string `json:"file_name"`
	FileType   string `json:"file_type"`
	FileSize   int64  `json:"file_size"`
	UploadedBy uint   `json:"uploaded_by"`
}

type MediaFileResponse struct {
	ModelResponse
	MediaFileRequest
}

type EmailTemplateRequest struct {
	DesignID        uint   `json:"design_id"`
	Key             string `json:"key"`
	SubjectEn       string `json:"subject_en"`
	SubjectID       string `json:"subject_id"`
	PreheaderEn     string `json:"preheader_en"`
	PreheaderID     string `json:"preheader_id"`
	BodyEn          string `json:"body_en"`
	BodyID          string `json:"body_id"`
	DesignJSON      string `json:"design_json"`
	DesignJSONEn    string `json:"design_json_en"`
	DesignJSONID    string `json:"design_json_id"`
	FooterEn        string `json:"footer_en"`
	FooterID        string `json:"footer_id"`
	BackgroundColor string `json:"background_color"`
	AccentColor     string `json:"accent_color"`
}

type EmailTemplateResponse struct {
	ModelResponse
	EmailTemplateRequest
}

type EmailDesignRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	BackgroundColor string `json:"background_color"`
	ContentColor    string `json:"content_color"`
	AccentColor     string `json:"accent_color"`
	TextColor       string `json:"text_color"`
	Width           int    `json:"width"`
	BlocksJSON      string `json:"blocks_json"`
	IsDefault       bool   `json:"is_default"`
}

type MailerTestRequest struct {
	ToEmail string `json:"to_email" binding:"required,email"`
	ToName  string `json:"to_name"`
	Subject string `json:"subject" binding:"required"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

type BrevoSenderRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type MailerBroadcastRequest struct {
	Target  string `json:"target" binding:"required"`
	UserIDs []uint `json:"user_ids"`
	Subject string `json:"subject" binding:"required"`
	HTML    string `json:"html" binding:"required"`
	Text    string `json:"text"`
}

type MailerCampaignRequest struct {
	DesignID           uint   `json:"design_id"`
	TemplateKey        string `json:"template_key"`
	Name               string `json:"name"`
	Target             string `json:"target" binding:"required"`
	UserIDs            []uint `json:"user_ids"`
	Subject            string `json:"subject" binding:"required"`
	SubjectEn          string `json:"subject_en"`
	SubjectID          string `json:"subject_id"`
	HTML               string `json:"html" binding:"required"`
	HTMLEn             string `json:"html_en"`
	HTMLID             string `json:"html_id"`
	Text               string `json:"text"`
	TextEn             string `json:"text_en"`
	TextID             string `json:"text_id"`
	SenderName         string `json:"sender_name"`
	SenderEmail        string `json:"sender_email" binding:"omitempty,email"`
	RateLimitPerMinute int    `json:"rate_limit_per_minute"`
}
