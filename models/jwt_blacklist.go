package models

import "time"

type JWTBlacklist struct {
	BaseModel
	TokenJTI  string    `gorm:"size:191;not null;uniqueIndex" json:"token_jti"`
	ExpiredAt time.Time `gorm:"not null;index" json:"expired_at"`
}
