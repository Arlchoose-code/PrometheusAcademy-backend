package structs

import "time"

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=191"`
	Email    string `json:"email" binding:"required,email,max=191"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Language string `json:"language" binding:"omitempty,oneof=en id"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=191"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type VerifyAuthOTPRequest struct {
	Email   string `json:"email" binding:"required,email,max=191"`
	Purpose string `json:"purpose" binding:"required,oneof=register login"`
	Code    string `json:"code" binding:"required,len=6"`
}

type ResendAuthOTPRequest struct {
	Email   string `json:"email" binding:"required,email,max=191"`
	Purpose string `json:"purpose" binding:"required,oneof=register login"`
}

type AuthChallengeResponse struct {
	RequiresOTP bool   `json:"requires_otp"`
	Purpose     string `json:"purpose"`
	Email       string `json:"email"`
	Message     string `json:"message"`
}

type UpdateProfileRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=191"`
	Phone        string `json:"phone" binding:"omitempty,max=50"`
	Language     string `json:"language" binding:"omitempty,oneof=en id"`
	BioEn        string `json:"bio_en" binding:"omitempty,max=5000"`
	BioID        string `json:"bio_id" binding:"omitempty,max=5000"`
	LinkedinURL  string `json:"linkedin_url" binding:"omitempty,max=255"`
	PortfolioURL string `json:"portfolio_url" binding:"omitempty,max=255"`
	Skills       string `json:"skills" binding:"omitempty,max=2000"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type RequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email,max=191"`
}

type ConfirmPasswordResetRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type UserRoleRequest struct {
	Role    string `json:"role"`
	Enabled *bool  `json:"enabled"`
}

type UserRequest struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	Avatar       string `json:"avatar"`
	Phone        string `json:"phone"`
	IsStudent    bool   `json:"is_student"`
	IsAdmin      bool   `json:"is_admin"`
	IsInstructor bool   `json:"is_instructor"`
	Language     string `json:"language"`
}

type UserResponse struct {
	ID                  uint                `json:"id"`
	Name                string              `json:"name"`
	Email               string              `json:"email"`
	Avatar              string              `json:"avatar"`
	Phone               string              `json:"phone"`
	IsStudent           bool                `json:"is_student"`
	IsAdmin             bool                `json:"is_admin"`
	IsInstructor        bool                `json:"is_instructor"`
	InstructorGrantedAt *time.Time          `json:"instructor_granted_at,omitempty"`
	InstructorGrantedBy *uint               `json:"instructor_granted_by,omitempty"`
	Language            string              `json:"language"`
	EmailVerifiedAt     *time.Time          `json:"email_verified_at,omitempty"`
	Profile             UserProfileResponse `json:"profile"`
}

type UserProfileRequest struct {
	UserID       uint   `json:"user_id"`
	BioEn        string `json:"bio_en"`
	BioID        string `json:"bio_id"`
	LinkedinURL  string `json:"linkedin_url"`
	PortfolioURL string `json:"portfolio_url"`
	Skills       string `json:"skills"`
}

type UserProfileResponse struct {
	ID           uint   `json:"id,omitempty"`
	UserID       uint   `json:"user_id,omitempty"`
	BioEn        string `json:"bio_en"`
	BioID        string `json:"bio_id"`
	LinkedinURL  string `json:"linkedin_url"`
	PortfolioURL string `json:"portfolio_url"`
	Skills       string `json:"skills"`
}
