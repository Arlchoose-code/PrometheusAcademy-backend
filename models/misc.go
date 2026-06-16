package models

import "time"

type Notification struct {
	BaseModel
	UserID    uint   `gorm:"not null;index" json:"user_id"`
	TitleEn   string `gorm:"size:191;not null" json:"title_en"`
	TitleID   string `gorm:"size:191;not null" json:"title_id"`
	MessageEn string `gorm:"type:text;not null" json:"message_en"`
	MessageID string `gorm:"type:text;not null" json:"message_id"`
	IsRead    bool   `gorm:"not null;default:false;index" json:"is_read"`
	Type      string `gorm:"size:50;not null;index" json:"type"`
	Link      string `gorm:"size:255" json:"link"`
}

type Event struct {
	BaseModel
	TitleEn       string    `gorm:"size:191;not null" json:"title_en"`
	TitleID       string    `gorm:"size:191;not null" json:"title_id"`
	DescriptionEn string    `gorm:"type:text" json:"description_en"`
	DescriptionID string    `gorm:"type:text" json:"description_id"`
	StartDate     time.Time `gorm:"not null;index" json:"start_date"`
	EndDate       time.Time `gorm:"not null;index" json:"end_date"`
	IsActive      bool      `gorm:"not null;default:true;index" json:"is_active"`
}
