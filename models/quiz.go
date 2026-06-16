package models

import "time"

type Quiz struct {
	BaseModel
	ModuleID     uint   `gorm:"not null;index" json:"module_id"`
	TitleEn      string `gorm:"size:191;not null" json:"title_en"`
	TitleID      string `gorm:"size:191;not null" json:"title_id"`
	PassingScore int    `gorm:"not null;default:70" json:"passing_score"`
	AttemptLimit int    `gorm:"not null;default:3" json:"attempt_limit"`
	Order        int    `gorm:"not null;default:0;index" json:"order"`
}

type QuizQuestion struct {
	BaseModel
	QuizID     uint   `gorm:"not null;index" json:"quiz_id"`
	Type       string `gorm:"size:40;not null" json:"type"`
	QuestionEn string `gorm:"type:longtext;not null" json:"question_en"`
	QuestionID string `gorm:"type:longtext;not null" json:"question_id"`
	Order      int    `gorm:"not null;default:0;index" json:"order"`
}

type QuizAnswer struct {
	BaseModel
	QuestionID uint   `gorm:"not null;index" json:"question_id"`
	AnswerEn   string `gorm:"type:text;not null" json:"answer_en"`
	AnswerID   string `gorm:"type:text;not null" json:"answer_id"`
	IsCorrect  bool   `gorm:"not null;default:false" json:"is_correct"`
	Order      int    `gorm:"not null;default:0;index" json:"order"`
}

type QuizSubmission struct {
	BaseModel
	QuizID        uint       `gorm:"not null;index" json:"quiz_id"`
	UserID        uint       `gorm:"not null;index" json:"user_id"`
	Score         int        `gorm:"not null;default:0" json:"score"`
	Passed        bool       `gorm:"not null;default:false" json:"passed"`
	AttemptNumber int        `gorm:"not null;default:1" json:"attempt_number"`
	SubmittedAt   time.Time  `gorm:"not null" json:"submitted_at"`
	ManualReview  bool       `gorm:"not null;default:false;index" json:"manual_review"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	ReviewedBy    uint       `gorm:"index" json:"reviewed_by"`
	Feedback      string     `gorm:"type:text" json:"feedback"`
}

type QuizSubmissionAnswer struct {
	BaseModel
	SubmissionID uint   `gorm:"not null;index" json:"submission_id"`
	QuestionID   uint   `gorm:"not null;index" json:"question_id"`
	AnswerID     uint   `gorm:"index" json:"answer_id"`
	TextAnswer   string `gorm:"type:text" json:"text_answer"`
}
