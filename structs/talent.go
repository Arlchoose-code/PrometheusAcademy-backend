package structs

import "time"

type TalentJobRequest struct {
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	Slug          string `json:"slug"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	OpenPositions int    `json:"open_positions"`
	Status        string `json:"status"`
}

type TalentJobResponse struct {
	ModelResponse
	TalentJobRequest
}

type TalentJobApplicationRequest struct {
	JobID     uint      `json:"job_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CVPath    string    `json:"cv_path"`
	Status    string    `json:"status"`
	AppliedAt time.Time `json:"applied_at"`
}

type TalentJobApplicationResponse struct {
	ModelResponse
	TalentJobApplicationRequest
}

type HiringInquiryRequest struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	WorkEmail   string `json:"work_email"`
	CompanyName string `json:"company_name"`
	CompanySize string `json:"company_size"`
	RolesNeeded string `json:"roles_needed"`
	Headcount   int    `json:"headcount"`
	Challenge   string `json:"challenge"`
	GDPRConsent bool   `json:"gdpr_consent"`
	Status      string `json:"status"`
}

type HiringInquiryResponse struct {
	ModelResponse
	HiringInquiryRequest
}

type TalentPlusApplicationRequest struct {
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	Country           string `json:"country"`
	CurrentStatus     string `json:"current_status"`
	JobField          string `json:"job_field"`
	ProgrammeInterest string `json:"programme_interest"`
	TargetCountries   string `json:"target_countries"`
	CareerGoals       string `json:"career_goals"`
	GDPRConsent       bool   `json:"gdpr_consent"`
	Status            string `json:"status"`
}

type TalentPlusApplicationResponse struct {
	ModelResponse
	TalentPlusApplicationRequest
}

type PartnerApplicationRequest struct {
	UniversityName   string `json:"university_name"`
	Country          string `json:"country"`
	ContactPerson    string `json:"contact_person"`
	RolePosition     string `json:"role_position"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	CurrentQSRanking string `json:"current_qs_ranking"`
	PartnershipGoals string `json:"partnership_goals"`
	Status           string `json:"status"`
}

type PartnerApplicationResponse struct {
	ModelResponse
	PartnerApplicationRequest
}

type PartnerRequest struct {
	Name          string `json:"name"`
	Country       string `json:"country"`
	Logo          string `json:"logo"`
	Website       string `json:"website"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
	IsActive      bool   `json:"is_active"`
}

type PartnerResponse struct {
	ModelResponse
	PartnerRequest
}
