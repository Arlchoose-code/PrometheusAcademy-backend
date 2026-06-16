package structs

import "time"

type CourseCategoryRequest struct {
	NameEn string `json:"name_en"`
	NameID string `json:"name_id"`
	Slug   string `json:"slug"`
}

type CourseCategoryResponse struct {
	ModelResponse
	CourseCategoryRequest
}

type CourseRequest struct {
	TitleEn          string `json:"title_en"`
	TitleID          string `json:"title_id"`
	Slug             string `json:"slug"`
	DescriptionEn    string `json:"description_en"`
	DescriptionID    string `json:"description_id"`
	Thumbnail        string `json:"thumbnail"`
	Price            int    `json:"price"`
	Status           string `json:"status"`
	IsFree           bool   `json:"is_free"`
	InstructorID     uint   `json:"instructor_id"`
	CategoryID       uint   `json:"category_id"`
	MinQuizScore     int    `json:"min_quiz_score"`
	QuizAttemptLimit int    `json:"quiz_attempt_limit"`
}

type CourseResponse struct {
	ModelResponse
	CourseRequest
}

type CourseModuleRequest struct {
	CourseID uint   `json:"course_id"`
	TitleEn  string `json:"title_en"`
	TitleID  string `json:"title_id"`
	Order    int    `json:"order"`
}

type CourseModuleResponse struct {
	ModelResponse
	CourseModuleRequest
}

type TopicRequest struct {
	ModuleID        uint   `json:"module_id"`
	TitleEn         string `json:"title_en"`
	TitleID         string `json:"title_id"`
	ContentEn       string `json:"content_en"`
	ContentID       string `json:"content_id"`
	VideoURL        string `json:"video_url"`
	DurationSeconds int    `json:"duration_seconds"`
	Order           int    `json:"order"`
}

type TopicResponse struct {
	ModelResponse
	TopicRequest
}

type TopicAttachmentRequest struct {
	TopicID  uint   `json:"topic_id"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"`
	NameEn   string `json:"name_en"`
	NameID   string `json:"name_id"`
}

type TopicAttachmentResponse struct {
	ModelResponse
	TopicAttachmentRequest
}

type TopicBlockRequest struct {
	TopicID         uint   `json:"topic_id"`
	Type            string `json:"type"`
	TitleEn         string `json:"title_en"`
	TitleID         string `json:"title_id"`
	BodyEn          string `json:"body_en"`
	BodyID          string `json:"body_id"`
	URL             string `json:"url"`
	FilePath        string `json:"file_path"`
	FileName        string `json:"file_name"`
	FileType        string `json:"file_type"`
	DurationSeconds int    `json:"duration_seconds"`
	Order           int    `json:"order"`
}

type TopicBlockResponse struct {
	ModelResponse
	TopicBlockRequest
}

type CourseEnrollmentRequest struct {
	UserID      uint       `json:"user_id"`
	CourseID    uint       `json:"course_id"`
	EnrolledAt  time.Time  `json:"enrolled_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type CourseEnrollmentResponse struct {
	ModelResponse
	CourseEnrollmentRequest
}

type TopicProgressRequest struct {
	UserID       uint       `json:"user_id"`
	TopicID      uint       `json:"topic_id"`
	CompletedAt  *time.Time `json:"completed_at"`
	VideoWatched bool       `json:"video_watched"`
}

type TopicProgressResponse struct {
	ModelResponse
	TopicProgressRequest
}

type AssignmentRequest struct {
	TopicID       uint   `json:"topic_id"`
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	DescriptionEn string `json:"description_en"`
	DescriptionID string `json:"description_id"`
}

type AssignmentResponse struct {
	ModelResponse
	AssignmentRequest
}

type AssignmentSubmissionRequest struct {
	AssignmentID uint       `json:"assignment_id"`
	UserID       uint       `json:"user_id"`
	FilePath     string     `json:"file_path"`
	Score        int        `json:"score"`
	Feedback     string     `json:"feedback"`
	SubmittedAt  *time.Time `json:"submitted_at"`
}

type AssignmentSubmissionResponse struct {
	ModelResponse
	AssignmentSubmissionRequest
}

type CertificateRequest struct {
	UserID         uint      `json:"user_id"`
	CourseID       uint      `json:"course_id"`
	UUID           string    `json:"uuid"`
	IssuedAt       time.Time `json:"issued_at"`
	CertificateURL string    `json:"certificate_url"`
}

type CertificateResponse struct {
	ModelResponse
	CertificateRequest
}

type DripScheduleRequest struct {
	CourseID           uint `json:"course_id"`
	ModuleID           uint `json:"module_id"`
	AvailableAfterDays int  `json:"available_after_days"`
}

type DripScheduleResponse struct {
	ModelResponse
	DripScheduleRequest
}

type ReviewRequest struct {
	UserID         uint   `json:"user_id"`
	ReviewableID   uint   `json:"reviewable_id"`
	ReviewableType string `json:"reviewable_type"`
	Rating         int    `json:"rating"`
	Comment        string `json:"comment"`
}

type ReviewResponse struct {
	ModelResponse
	ReviewRequest
}
