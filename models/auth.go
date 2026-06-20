package models

import "time"

type User struct {
	BaseModel
	Name                string     `gorm:"size:191;not null" json:"name"`
	Email               string     `gorm:"size:191;not null;uniqueIndex" json:"email"`
	Password            string     `gorm:"size:255;not null" json:"-"`
	Avatar              string     `gorm:"size:255" json:"avatar"`
	Phone               string     `gorm:"size:50" json:"phone"`
	IsStudent           bool       `gorm:"not null;default:true" json:"is_student"`
	IsAdmin             bool       `gorm:"not null;default:false;index" json:"is_admin"`
	IsInstructor        bool       `gorm:"not null;default:false;index" json:"is_instructor"`
	InstructorGrantedAt *time.Time `json:"instructor_granted_at"`
	InstructorGrantedBy *uint      `gorm:"index" json:"instructor_granted_by"`
	Language            string     `gorm:"size:5;not null;default:'en'" json:"language"`
	TokenVersion        int        `gorm:"not null;default:0" json:"-"`
	EmailVerifiedAt     *time.Time `json:"email_verified_at"`
	LastLoginOTPAt      *time.Time `json:"last_login_otp_at"`
}

type AuthEmailOTP struct {
	BaseModel
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Email     string     `gorm:"size:191;not null;index" json:"email"`
	Purpose   string     `gorm:"size:30;not null;index" json:"purpose"`
	CodeHash  string     `gorm:"size:191;not null;index" json:"-"`
	Attempts  int        `gorm:"not null;default:0" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
}

type PasswordResetToken struct {
	BaseModel
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:191;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
}

type UserProfile struct {
	BaseModel
	UserID       uint   `gorm:"not null;uniqueIndex" json:"user_id"`
	BioEn        string `gorm:"type:text" json:"bio_en"`
	BioID        string `gorm:"type:text" json:"bio_id"`
	LinkedinURL  string `gorm:"size:255" json:"linkedin_url"`
	PortfolioURL string `gorm:"size:255" json:"portfolio_url"`
	Skills       string `gorm:"type:text" json:"skills"`
}
