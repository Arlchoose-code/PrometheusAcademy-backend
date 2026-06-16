package models

import "time"

type CourseCategory struct {
	BaseModel
	NameEn string `gorm:"size:191;not null" json:"name_en"`
	NameID string `gorm:"size:191;not null" json:"name_id"`
	Slug   string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
}

type Course struct {
	BaseModel
	TitleEn          string `gorm:"size:191;not null" json:"title_en"`
	TitleID          string `gorm:"size:191;not null" json:"title_id"`
	Slug             string `gorm:"size:191;not null;uniqueIndex" json:"slug"`
	DescriptionEn    string `gorm:"type:longtext" json:"description_en"`
	DescriptionID    string `gorm:"type:longtext" json:"description_id"`
	Thumbnail        string `gorm:"size:255" json:"thumbnail"`
	Price            int    `gorm:"not null;default:0" json:"price"`
	Status           string `gorm:"size:20;not null;default:'draft';index" json:"status"`
	IsFree           bool   `gorm:"not null;default:false" json:"is_free"`
	InstructorID     uint   `gorm:"index" json:"instructor_id"`
	CategoryID       uint   `gorm:"index" json:"category_id"`
	MinQuizScore     int    `gorm:"not null;default:70" json:"min_quiz_score"`
	QuizAttemptLimit int    `gorm:"not null;default:3" json:"quiz_attempt_limit"`
}

type CourseModule struct {
	BaseModel
	CourseID uint   `gorm:"not null;index" json:"course_id"`
	TitleEn  string `gorm:"size:191;not null" json:"title_en"`
	TitleID  string `gorm:"size:191;not null" json:"title_id"`
	Order    int    `gorm:"not null;default:0;index" json:"order"`
}

type Topic struct {
	BaseModel
	ModuleID        uint   `gorm:"not null;index" json:"module_id"`
	TitleEn         string `gorm:"size:191;not null" json:"title_en"`
	TitleID         string `gorm:"size:191;not null" json:"title_id"`
	ContentEn       string `gorm:"type:longtext" json:"content_en"`
	ContentID       string `gorm:"type:longtext" json:"content_id"`
	VideoURL        string `gorm:"size:500" json:"video_url"`
	DurationSeconds int    `gorm:"not null;default:0" json:"duration_seconds"`
	Order           int    `gorm:"not null;default:0;index" json:"order"`
}

type TopicAttachment struct {
	BaseModel
	TopicID  uint   `gorm:"not null;index" json:"topic_id"`
	FilePath string `gorm:"size:255;not null" json:"file_path"`
	FileType string `gorm:"size:50;not null" json:"file_type"`
	NameEn   string `gorm:"size:191;not null" json:"name_en"`
	NameID   string `gorm:"size:191;not null" json:"name_id"`
}

// TopicBlock is an ordered piece of content inside a Topic.
// A topic is built from a sequence of blocks (Coursera/Notion style):
// text, video, pdf, file, or image. This replaces the old single
// content_en/video_url/attachments model with a flexible builder.
type TopicBlock struct {
	BaseModel
	TopicID         uint   `gorm:"not null;index" json:"topic_id"`
	Type            string `gorm:"size:30;not null;default:'text'" json:"type"` // text | video | pdf | file | image
	TitleEn         string `gorm:"size:255" json:"title_en"`
	TitleID         string `gorm:"size:255" json:"title_id"`
	BodyEn          string `gorm:"type:longtext" json:"body_en"` // rich text / description / caption
	BodyID          string `gorm:"type:longtext" json:"body_id"`
	URL             string `gorm:"size:500" json:"url"`       // youtube / external link
	FilePath        string `gorm:"size:500" json:"file_path"` // uploaded pdf/file/image path
	FileName        string `gorm:"size:255" json:"file_name"`
	FileType        string `gorm:"size:100" json:"file_type"`
	DurationSeconds int    `gorm:"not null;default:0" json:"duration_seconds"`
	Order           int    `gorm:"not null;default:0;index" json:"order"`
}

type CourseEnrollment struct {
	BaseModel
	UserID      uint       `gorm:"not null;index;uniqueIndex:idx_user_course" json:"user_id"`
	CourseID    uint       `gorm:"not null;index;uniqueIndex:idx_user_course" json:"course_id"`
	EnrolledAt  time.Time  `gorm:"not null" json:"enrolled_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type TopicProgress struct {
	BaseModel
	UserID       uint       `gorm:"not null;index;uniqueIndex:idx_user_topic" json:"user_id"`
	TopicID      uint       `gorm:"not null;index;uniqueIndex:idx_user_topic" json:"topic_id"`
	CompletedAt  *time.Time `json:"completed_at"`
	VideoWatched bool       `gorm:"not null;default:false" json:"video_watched"`
}

type Assignment struct {
	BaseModel
	TopicID       uint   `gorm:"not null;index" json:"topic_id"`
	TitleEn       string `gorm:"size:191;not null" json:"title_en"`
	TitleID       string `gorm:"size:191;not null" json:"title_id"`
	DescriptionEn string `gorm:"type:longtext" json:"description_en"`
	DescriptionID string `gorm:"type:longtext" json:"description_id"`
}

type AssignmentSubmission struct {
	BaseModel
	AssignmentID uint       `gorm:"not null;index" json:"assignment_id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	FilePath     string     `gorm:"size:255" json:"file_path"`
	Score        int        `gorm:"not null;default:0" json:"score"`
	Feedback     string     `gorm:"type:text" json:"feedback"`
	SubmittedAt  *time.Time `json:"submitted_at"`
}

type Certificate struct {
	BaseModel
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	CourseID       uint      `gorm:"not null;index" json:"course_id"`
	UUID           string    `gorm:"size:36;uniqueIndex" json:"uuid"`
	IssuedAt       time.Time `gorm:"not null" json:"issued_at"`
	CertificateURL string    `gorm:"size:255;not null" json:"certificate_url"`
}

type DripSchedule struct {
	BaseModel
	CourseID           uint `gorm:"not null;index" json:"course_id"`
	ModuleID           uint `gorm:"not null;index" json:"module_id"`
	AvailableAfterDays int  `gorm:"not null;default:0" json:"available_after_days"`
}

type Review struct {
	BaseModel
	UserID         uint   `gorm:"not null;index" json:"user_id"`
	ReviewableID   uint   `gorm:"not null;index:idx_reviewable" json:"reviewable_id"`
	ReviewableType string `gorm:"size:20;not null;index:idx_reviewable" json:"reviewable_type"`
	Rating         int    `gorm:"not null" json:"rating"`
	Comment        string `gorm:"type:text" json:"comment"`
}
