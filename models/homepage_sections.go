package models

import "time"

type HowItWorksStep struct {
	BaseModel
	TitleEn       string `gorm:"size:191;not null" json:"title_en"`
	TitleID       string `gorm:"size:191;not null" json:"title_id"`
	DescriptionEn string `gorm:"type:text" json:"description_en"`
	DescriptionID string `gorm:"type:text" json:"description_id"`
	Icon          string `gorm:"size:40" json:"icon"`
	StepNumber    int    `gorm:"not null;default:0" json:"step_number"`
	IsActive      bool   `gorm:"not null;default:true;index" json:"is_active"`
}

type MomentGallery struct {
	BaseModel
	ImagePath  string     `gorm:"size:255;not null" json:"image_path"`
	CaptionEn  string     `gorm:"size:255" json:"caption_en"`
	CaptionID  string     `gorm:"size:255" json:"caption_id"`
	EventDate  *time.Time `json:"event_date"`
	Tag        string     `gorm:"size:80" json:"tag"`
	Order      int        `gorm:"not null;default:0" json:"order"`
	IsActive   bool       `gorm:"not null;default:true;index" json:"is_active"`
}
