package models

import "time"

type CourseConversation struct {
	BaseModel
	CourseID      uint      `gorm:"not null;index" json:"course_id"`
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	Subject       string    `gorm:"size:191;not null" json:"subject"`
	Status        string    `gorm:"size:20;not null;default:'open';index" json:"status"`
	StudentUnread int       `gorm:"not null;default:0" json:"student_unread"`
	StaffUnread   int       `gorm:"not null;default:0" json:"staff_unread"`
	LastMessageAt time.Time `gorm:"not null;index" json:"last_message_at"`
}

type CourseMessage struct {
	BaseModel
	ConversationID uint   `gorm:"not null;index" json:"conversation_id"`
	UserID         uint   `gorm:"not null;index" json:"user_id"`
	Body           string `gorm:"type:text;not null" json:"body"`
	IsInstructor   bool   `gorm:"not null;default:false;index" json:"is_instructor"`
}
