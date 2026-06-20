package models

import "time"

type AutomationWorkflow struct {
	BaseModel
	Key          string `gorm:"size:191;not null;uniqueIndex" json:"key"`
	Name         string `gorm:"size:191;not null" json:"name"`
	Category     string `gorm:"size:40;not null;index" json:"category"`
	TriggerType  string `gorm:"size:60;not null;index" json:"trigger_type"`
	DelayMinutes int    `gorm:"not null;default:0" json:"delay_minutes"`
	TemplateKey  string `gorm:"size:191" json:"template_key"`
	SubjectEn    string `gorm:"size:191;not null" json:"subject_en"`
	SubjectID    string `gorm:"size:191;not null" json:"subject_id"`
	BodyEn       string `gorm:"type:longtext;not null" json:"body_en"`
	BodyID       string `gorm:"type:longtext;not null" json:"body_id"`
	IsEnabled    bool   `gorm:"not null;default:true;index" json:"is_enabled"`
}

type AutomationRun struct {
	BaseModel
	WorkflowID     uint       `gorm:"not null;index" json:"workflow_id"`
	UserID         uint       `gorm:"not null;default:0;index" json:"user_id"`
	Email          string     `gorm:"size:191;not null;index" json:"email"`
	EntityType     string     `gorm:"size:40;index" json:"entity_type"`
	EntityID       uint       `gorm:"not null;default:0;index" json:"entity_id"`
	IdempotencyKey string     `gorm:"size:191;not null;uniqueIndex" json:"idempotency_key"`
	Status         string     `gorm:"size:30;not null;default:'scheduled';index" json:"status"`
	ScheduledAt    time.Time  `gorm:"not null;index" json:"scheduled_at"`
	SentAt         *time.Time `json:"sent_at"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message"`
	MessageID      string     `gorm:"size:191;index" json:"message_id"`
	VariablesJSON  string     `gorm:"type:text" json:"variables_json"`
}

type EmailSuppression struct {
	BaseModel
	Email  string `gorm:"size:191;not null;uniqueIndex" json:"email"`
	Reason string `gorm:"size:40;not null;index" json:"reason"`
	Source string `gorm:"size:60" json:"source"`
}

type EmailEvent struct {
	BaseModel
	CampaignID uint      `gorm:"not null;default:0;index" json:"campaign_id"`
	RunID      uint      `gorm:"not null;default:0;index" json:"run_id"`
	Email      string    `gorm:"size:191;not null;index" json:"email"`
	MessageID  string    `gorm:"size:191;index" json:"message_id"`
	EventType  string    `gorm:"size:30;not null;index" json:"event_type"`
	Revenue    int       `gorm:"not null;default:0" json:"revenue"`
	OccurredAt time.Time `gorm:"not null;index" json:"occurred_at"`
}
