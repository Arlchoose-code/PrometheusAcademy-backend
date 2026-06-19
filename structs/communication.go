package structs

import "time"

type CreateConversationRequest struct {
	CourseID uint   `json:"course_id" binding:"required"`
	Subject  string `json:"subject" binding:"required,max=191"`
	Message  string `json:"message" binding:"required,max=5000"`
}

type CreateMessageRequest struct {
	Message string `json:"message" binding:"required,max=5000"`
}

type UpdateConversationRequest struct {
	Status string `json:"status" binding:"required,oneof=open resolved closed"`
}

type ConversationResponse struct {
	ID                 uint      `json:"id"`
	CourseID           uint      `json:"course_id"`
	CourseTitleEn      string    `json:"course_title_en"`
	CourseTitleID      string    `json:"course_title_id"`
	StudentID          uint      `json:"student_id"`
	StudentName        string    `json:"student_name"`
	Subject            string    `json:"subject"`
	Status             string    `json:"status"`
	StudentUnread      int       `json:"student_unread"`
	StaffUnread        int       `json:"staff_unread"`
	LastMessageAt      time.Time `json:"last_message_at"`
	CurrentUserIsStaff bool      `json:"current_user_is_staff"`
}

type ConversationMessageResponse struct {
	ID           uint      `json:"id"`
	UserID       uint      `json:"user_id"`
	UserName     string    `json:"user_name"`
	Body         string    `json:"body"`
	IsInstructor bool      `json:"is_instructor"`
	CreatedAt    time.Time `json:"created_at"`
}

type ConversationCourseResponse struct {
	ID      uint   `json:"id"`
	TitleEn string `json:"title_en"`
	TitleID string `json:"title_id"`
}

type CommunicationHubResponse struct {
	Conversations []ConversationResponse       `json:"conversations"`
	Courses       []ConversationCourseResponse `json:"courses"`
	UnreadCount   int64                        `json:"unread_count"`
}
