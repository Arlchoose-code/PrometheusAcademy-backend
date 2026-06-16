package structs

import "time"

type QuizRequest struct {
	ModuleID     uint   `json:"module_id"`
	TitleEn      string `json:"title_en"`
	TitleID      string `json:"title_id"`
	PassingScore int    `json:"passing_score"`
	AttemptLimit int    `json:"attempt_limit"`
	Order        int    `json:"order"`
}

type QuizResponse struct {
	ModelResponse
	QuizRequest
}

type QuizQuestionRequest struct {
	QuizID     uint   `json:"quiz_id"`
	Type       string `json:"type"`
	QuestionEn string `json:"question_en"`
	QuestionID string `json:"question_id"`
	Order      int    `json:"order"`
}

type QuizQuestionResponse struct {
	ModelResponse
	QuizQuestionRequest
}

type QuizAnswerRequest struct {
	QuestionID uint   `json:"question_id"`
	AnswerEn   string `json:"answer_en"`
	AnswerID   string `json:"answer_id"`
	IsCorrect  bool   `json:"is_correct"`
	Order      int    `json:"order"`
}

type QuizAnswerResponse struct {
	ModelResponse
	QuizAnswerRequest
}

type QuizSubmissionRequest struct {
	QuizID        uint       `json:"quiz_id"`
	UserID        uint       `json:"user_id"`
	Score         int        `json:"score"`
	Passed        bool       `json:"passed"`
	AttemptNumber int        `json:"attempt_number"`
	SubmittedAt   time.Time  `json:"submitted_at"`
	ManualReview  bool       `json:"manual_review"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	ReviewedBy    uint       `json:"reviewed_by"`
	Feedback      string     `json:"feedback"`
}

type QuizSubmissionResponse struct {
	ModelResponse
	QuizSubmissionRequest
}

type QuizSubmissionAnswerRequest struct {
	SubmissionID uint   `json:"submission_id"`
	QuestionID   uint   `json:"question_id"`
	AnswerID     uint   `json:"answer_id"`
	TextAnswer   string `json:"text_answer"`
}

type QuizSubmissionAnswerResponse struct {
	ModelResponse
	QuizSubmissionAnswerRequest
}
