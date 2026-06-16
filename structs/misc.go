package structs

import "time"

type NotificationRequest struct {
	UserID    uint   `json:"user_id"`
	TitleEn   string `json:"title_en"`
	TitleID   string `json:"title_id"`
	MessageEn string `json:"message_en"`
	MessageID string `json:"message_id"`
	IsRead    bool   `json:"is_read"`
	Type      string `json:"type"`
	Link      string `json:"link"`
}

type NotificationResponse struct {
	ModelResponse
	NotificationRequest
}

type EventRequest struct {
	TitleEn       string    `json:"title_en"`
	TitleID       string    `json:"title_id"`
	DescriptionEn string    `json:"description_en"`
	DescriptionID string    `json:"description_id"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	IsActive      bool      `json:"is_active"`
}

type EventResponse struct {
	ModelResponse
	EventRequest
}
