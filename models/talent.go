package models

import "time"

type TalentJob struct {
	BaseModel
	HiringInquiryID uint   `gorm:"index" json:"hiring_inquiry_id"`
	TitleEn         string `gorm:"size:191;not null" json:"title_en"`
	TitleID         string `gorm:"size:191;not null" json:"title_id"`
	Slug            string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
	DescriptionEn   string `gorm:"type:longtext" json:"description_en"`
	DescriptionID   string `gorm:"type:longtext" json:"description_id"`
	OpenPositions   int    `gorm:"not null;default:1" json:"open_positions"`
	Status          string `gorm:"size:20;not null;default:'open';index" json:"status"`
}

type TalentJobApplication struct {
	BaseModel
	UserID    uint      `gorm:"index" json:"user_id"`
	JobID     uint      `gorm:"not null;index" json:"job_id"`
	Name      string    `gorm:"size:191;not null" json:"name"`
	Email     string    `gorm:"size:191;not null;index" json:"email"`
	CVPath    string    `gorm:"size:255;not null" json:"cv_path"`
	Status    string    `gorm:"size:30;not null;default:'new';index" json:"status"`
	AppliedAt time.Time `gorm:"not null" json:"applied_at"`
}

type TalentTrustPhoto struct {
	BaseModel
	TitleEn   string `gorm:"size:191;not null" json:"title_en"`
	TitleID   string `gorm:"size:191;not null" json:"title_id"`
	Category  string `gorm:"size:60;not null;default:'general';index" json:"category"`
	ImagePath string `gorm:"size:255;not null" json:"image_path"`
	Order     int    `gorm:"not null;default:0;index" json:"order"`
	IsActive  bool   `gorm:"not null;default:true;index" json:"is_active"`
}

type HiringInquiry struct {
	BaseModel
	FirstName   string `gorm:"size:100;not null" json:"first_name"`
	LastName    string `gorm:"size:100;not null" json:"last_name"`
	WorkEmail   string `gorm:"size:191;not null;index" json:"work_email"`
	CompanyName string `gorm:"size:191;not null" json:"company_name"`
	CompanySize string `gorm:"size:100" json:"company_size"`
	RolesNeeded string `gorm:"type:text;not null" json:"roles_needed"`
	Headcount   int    `gorm:"not null;default:1" json:"headcount"`
	Challenge   string `gorm:"type:text" json:"challenge"`
	GDPRConsent bool   `gorm:"not null;default:false" json:"gdpr_consent"`
	Status      string `gorm:"size:30;not null;default:'new';index" json:"status"`
	Language    string `gorm:"-" json:"language,omitempty"`
}

type TalentPlusApplication struct {
	BaseModel
	UserID            uint   `gorm:"index" json:"user_id"`
	FirstName         string `gorm:"size:100;not null" json:"first_name"`
	LastName          string `gorm:"size:100;not null" json:"last_name"`
	Email             string `gorm:"size:191;not null;index" json:"email"`
	Phone             string `gorm:"size:50;not null" json:"phone"`
	Country           string `gorm:"size:100;not null" json:"country"`
	CurrentStatus     string `gorm:"size:191;not null" json:"current_status"`
	JobField          string `gorm:"size:191;not null" json:"job_field"`
	ProgrammeInterest string `gorm:"size:191;not null" json:"programme_interest"`
	TargetCountries   string `gorm:"type:text;not null" json:"target_countries"`
	CareerGoals       string `gorm:"type:text" json:"career_goals"`
	GDPRConsent       bool   `gorm:"not null;default:false" json:"gdpr_consent"`
	Status            string `gorm:"size:30;not null;default:'new';index" json:"status"`
	Language          string `gorm:"-" json:"language,omitempty"`
}

type TalentReviewInvitation struct {
	BaseModel
	ApplicationType string     `gorm:"size:30;not null;uniqueIndex:idx_talent_review_application;index" json:"application_type"`
	ApplicationID   uint       `gorm:"not null;uniqueIndex:idx_talent_review_application;index" json:"application_id"`
	Name            string     `gorm:"size:191;not null" json:"name"`
	Email           string     `gorm:"size:191;not null;index" json:"email"`
	TokenHash       string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt       time.Time  `gorm:"not null;index" json:"expires_at"`
	SentAt          *time.Time `gorm:"index" json:"sent_at"`
	UsedAt          *time.Time `gorm:"index" json:"used_at"`
	TestimonialID   uint       `gorm:"index" json:"testimonial_id"`
}

type PartnerApplication struct {
	BaseModel
	UniversityName   string `gorm:"size:191;not null" json:"university_name"`
	Country          string `gorm:"size:100;not null" json:"country"`
	ContactPerson    string `gorm:"size:191;not null" json:"contact_person"`
	RolePosition     string `gorm:"size:191;not null" json:"role_position"`
	Email            string `gorm:"size:191;not null;index" json:"email"`
	Phone            string `gorm:"size:50" json:"phone"`
	CurrentQSRanking string `gorm:"size:100" json:"current_qs_ranking"`
	PartnershipGoals string `gorm:"type:text;not null" json:"partnership_goals"`
	Status           string `gorm:"size:30;not null;default:'new';index" json:"status"`
}

type Partner struct {
	BaseModel
	PartnerType   string `gorm:"size:30;not null;default:'university';index" json:"partner_type"`
	Name          string `gorm:"size:191;not null" json:"name"`
	Country       string `gorm:"size:100;not null" json:"country"`
	Logo          string `gorm:"size:255" json:"logo"`
	Website       string `gorm:"size:255" json:"website"`
	ContactInfo   string `gorm:"type:text" json:"contact_info"`
	DescriptionEn string `gorm:"type:text" json:"description_en"`
	DescriptionID string `gorm:"type:text" json:"description_id"`
	Status        string `gorm:"size:30;not null;default:'active';index" json:"status"`
	Notes         string `gorm:"type:text" json:"notes"`
	IsActive      bool   `gorm:"not null;default:true;index" json:"is_active"`
}
