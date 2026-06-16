package models

import "time"

type TalentJob struct {
	BaseModel
	TitleEn       string `gorm:"size:191;not null" json:"title_en"`
	TitleID       string `gorm:"size:191;not null" json:"title_id"`
	Slug          string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
	DescriptionEn string `gorm:"type:longtext" json:"description_en"`
	DescriptionID string `gorm:"type:longtext" json:"description_id"`
	OpenPositions int    `gorm:"not null;default:1" json:"open_positions"`
	Status        string `gorm:"size:20;not null;default:'open';index" json:"status"`
}

type TalentJobApplication struct {
	BaseModel
	JobID     uint      `gorm:"not null;index" json:"job_id"`
	Name      string    `gorm:"size:191;not null" json:"name"`
	Email     string    `gorm:"size:191;not null;index" json:"email"`
	CVPath    string    `gorm:"size:255;not null" json:"cv_path"`
	Status    string    `gorm:"size:30;not null;default:'new';index" json:"status"`
	AppliedAt time.Time `gorm:"not null" json:"applied_at"`
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
}

type TalentPlusApplication struct {
	BaseModel
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
	Name          string `gorm:"size:191;not null" json:"name"`
	Country       string `gorm:"size:100;not null" json:"country"`
	Logo          string `gorm:"size:255" json:"logo"`
	Website       string `gorm:"size:255" json:"website"`
	DescriptionEn string `gorm:"type:text" json:"description_en"`
	DescriptionID string `gorm:"type:text" json:"description_id"`
	IsActive      bool   `gorm:"not null;default:true;index" json:"is_active"`
}
