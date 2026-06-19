package models

import "time"

type XPLedger struct {
	BaseModel
	UserID        uint   `gorm:"not null;index;uniqueIndex:idx_xp_unique" json:"user_id"`
	EventType     string `gorm:"size:60;not null;index;uniqueIndex:idx_xp_unique" json:"event_type"`
	ReferenceType string `gorm:"size:60;not null;default:'';index;uniqueIndex:idx_xp_unique" json:"reference_type"`
	ReferenceID   uint   `gorm:"not null;default:0;index;uniqueIndex:idx_xp_unique" json:"reference_id"`
	Points        int    `gorm:"not null" json:"points"`
	DescriptionEn string `gorm:"size:255" json:"description_en"`
	DescriptionID string `gorm:"size:255" json:"description_id"`
}

type Achievement struct {
	BaseModel
	Code          string `gorm:"size:80;not null;uniqueIndex" json:"code"`
	NameEn        string `gorm:"size:120;not null" json:"name_en"`
	NameID        string `gorm:"size:120;not null" json:"name_id"`
	DescriptionEn string `gorm:"size:255" json:"description_en"`
	DescriptionID string `gorm:"size:255" json:"description_id"`
	Icon          string `gorm:"size:40" json:"icon"`
	RequiredXP    int    `gorm:"not null;default:0" json:"required_xp"`
	EventType     string `gorm:"size:60;index" json:"event_type"`
	Threshold     int    `gorm:"not null;default:0" json:"threshold"`
	IsActive      bool   `gorm:"not null;default:true;index" json:"is_active"`
}

type UserAchievement struct {
	BaseModel
	UserID        uint        `gorm:"not null;index;uniqueIndex:idx_user_achievement" json:"user_id"`
	AchievementID uint        `gorm:"not null;index;uniqueIndex:idx_user_achievement" json:"achievement_id"`
	AwardedAt     time.Time   `gorm:"not null" json:"awarded_at"`
	Achievement   Achievement `json:"achievement,omitempty"`
}
