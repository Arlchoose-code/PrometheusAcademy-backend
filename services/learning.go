package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

var ErrQuizAttemptLimitReached = errors.New("quiz attempt limit reached")

type LearningState struct {
	Enrolled        bool
	CourseCompleted bool
	ModuleUnlocked  map[uint]bool
	ModuleUnlockAt  map[uint]*time.Time
	TopicUnlocked   map[uint]bool
	TopicUnlockAt   map[uint]*time.Time
	QuizUnlocked    map[uint]bool
	QuizUnlockAt    map[uint]*time.Time
	TotalItems      int
	CompletedItems  int
}

type orderedLearningItem struct {
	Kind     string
	ID       uint
	ModuleID uint
	Order    int
}

type QuizAnswerInput struct {
	QuestionID uint   `json:"question_id"`
	AnswerID   uint   `json:"answer_id"`
	AnswerIDs  []uint `json:"answer_ids"`
	TextAnswer string `json:"text_answer"`
}

func CourseForModuleItem(ctx context.Context, db *gorm.DB, moduleID uint) (models.Course, error) {
	var module models.CourseModule
	if err := db.WithContext(ctx).First(&module, moduleID).Error; err != nil {
		return models.Course{}, err
	}
	var course models.Course
	if err := db.WithContext(ctx).First(&course, module.CourseID).Error; err != nil {
		return models.Course{}, err
	}
	return course, nil
}

func LearningStateForCourse(ctx context.Context, db *gorm.DB, course models.Course, userID uint) (LearningState, error) {
	state := LearningState{
		ModuleUnlocked: map[uint]bool{},
		ModuleUnlockAt: map[uint]*time.Time{},
		TopicUnlocked:  map[uint]bool{},
		TopicUnlockAt:  map[uint]*time.Time{},
		QuizUnlocked:   map[uint]bool{},
		QuizUnlockAt:   map[uint]*time.Time{},
	}
	if userID == 0 {
		return state, nil
	}
	var enrollment models.CourseEnrollment
	if err := db.WithContext(ctx).Where(models.CourseEnrollment{UserID: userID, CourseID: course.ID}).First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return state, nil
		}
		return state, err
	}
	state.Enrolled = true
	state.CourseCompleted = enrollment.CompletedAt != nil

	var drips []models.DripSchedule
	if err := db.WithContext(ctx).Where("course_id = ?", course.ID).Find(&drips).Error; err != nil {
		return state, err
	}
	dripByModule := map[uint]int{}
	for _, drip := range drips {
		dripByModule[drip.ModuleID] = drip.AvailableAfterDays
	}

	var modules []models.CourseModule
	if err := db.WithContext(ctx).Where("course_id = ?", course.ID).Order("`order` asc, id asc").Find(&modules).Error; err != nil {
		return state, err
	}
	now := time.Now()
	sequentialUnlocked := true
	for _, module := range modules {
		unlockAt := enrollment.EnrolledAt.AddDate(0, 0, dripByModule[module.ID])
		state.ModuleUnlockAt[module.ID] = &unlockAt
		moduleUnlocked := !now.Before(unlockAt)
		state.ModuleUnlocked[module.ID] = moduleUnlocked

		var topics []models.Topic
		if err := db.WithContext(ctx).Where("module_id = ?", module.ID).Order("`order` asc, id asc").Find(&topics).Error; err != nil {
			return state, err
		}
		var quizzes []models.Quiz
		if err := db.WithContext(ctx).Where("module_id = ?", module.ID).Order("`order` asc, id asc").Find(&quizzes).Error; err != nil {
			return state, err
		}
		items := make([]orderedLearningItem, 0, len(topics)+len(quizzes))
		for _, topic := range topics {
			items = append(items, orderedLearningItem{Kind: "topic", ID: topic.ID, ModuleID: module.ID, Order: topic.Order})
		}
		for _, quiz := range quizzes {
			items = append(items, orderedLearningItem{Kind: "quiz", ID: quiz.ID, ModuleID: module.ID, Order: quiz.Order})
		}
		sortLearningItems(items)

		for _, item := range items {
			state.TotalItems++
			unlocked := moduleUnlocked && sequentialUnlocked
			switch item.Kind {
			case "topic":
				state.TopicUnlocked[item.ID] = unlocked
				state.TopicUnlockAt[item.ID] = &unlockAt
				done, err := topicCompleted(ctx, db, item.ID, userID)
				if err != nil {
					return state, err
				}
				if done {
					state.CompletedItems++
				} else {
					sequentialUnlocked = false
				}
			case "quiz":
				state.QuizUnlocked[item.ID] = unlocked
				state.QuizUnlockAt[item.ID] = &unlockAt
				passed, err := quizPassed(ctx, db, item.ID, userID)
				if err != nil {
					return state, err
				}
				if passed {
					state.CompletedItems++
				} else {
					sequentialUnlocked = false
				}
			}
		}
	}
	return state, nil
}

func SyncCourseCompletion(ctx context.Context, db *gorm.DB, cfg config.Config, course models.Course, userID uint) (*models.Certificate, bool, error) {
	state, err := LearningStateForCourse(ctx, db, course, userID)
	if err != nil {
		return nil, false, err
	}
	if !state.Enrolled || state.TotalItems == 0 || state.CompletedItems < state.TotalItems {
		return nil, false, nil
	}
	now := time.Now()
	if err := db.WithContext(ctx).Model(&models.CourseEnrollment{}).
		Where(models.CourseEnrollment{UserID: userID, CourseID: course.ID}).
		Where("completed_at IS NULL").
		Update("completed_at", now).Error; err != nil {
		return nil, false, err
	}
	certificate := models.Certificate{
		UserID:         userID,
		CourseID:       course.ID,
		IssuedAt:       now,
		CertificateURL: "",
	}
	if err := db.WithContext(ctx).Where(models.Certificate{UserID: userID, CourseID: course.ID}).Attrs(certificate).FirstOrCreate(&certificate).Error; err != nil {
		return nil, false, err
	}
	if err := EnsureCertificateUUID(ctx, db, &certificate); err != nil {
		return nil, false, err
	}
	var user models.User
	_ = db.WithContext(ctx).First(&user, userID).Error
	_, version, _ := SelectDocumentTemplateVersion(ctx, db, "certificate", 0)
	courseTitle := course.TitleEn
	if strings.TrimSpace(courseTitle) == "" {
		courseTitle = course.TitleID
	}
	snapshot := map[string]any{
		"site_name":          "Prometheus Academy",
		"document_number":    CertificateDisplayCode(certificate),
		"recipient_name":     fallbackString(user.Name, "Prometheus Learner"),
		"recipient_email":    user.Email,
		"issued_at":          certificate.IssuedAt.Format("2006-01-02"),
		"locale":             fallbackString(user.Language, "en"),
		"verification_url":   localizedFrontendURL(cfg, fallbackString(user.Language, "en"), "/certificates/"+certificate.UUID),
		"certificate_number": CertificateDisplayCode(certificate),
		"certificate_uuid":   certificate.UUID,
		"student_name":       fallbackString(user.Name, "Prometheus Learner"),
		"course_name":        courseTitle,
		"course_name_en":     fallbackString(course.TitleEn, course.TitleID),
		"course_name_id":     fallbackString(course.TitleID, course.TitleEn),
		"instructor_name":    "Prometheus Academy",
		"completion_date":    certificate.IssuedAt.Format("2006-01-02"),
		"signatory_name":     "Prometheus Academy",
		"signatory_title":    "Academic Team",
	}
	rawSnapshot, snapshotChecksum := checksumJSON(snapshot)
	updates := map[string]any{"template_id": version.TemplateID, "template_version_id": version.ID, "locale": fallbackString(user.Language, "en"), "snapshot_json": rawSnapshot, "snapshot_checksum": snapshotChecksum}
	protectedURL := CertificateDownloadURL(certificate)
	if certificate.CertificateURL != protectedURL {
		updates["certificate_url"] = protectedURL
		certificate.CertificateURL = protectedURL
	}
	if err := db.WithContext(ctx).Model(&certificate).Updates(updates).Error; err != nil {
		return nil, false, err
	}
	certificate.TemplateID = version.TemplateID
	certificate.TemplateVersionID = version.ID
	certificate.Locale = fallbackString(user.Language, "en")
	certificate.SnapshotJSON = rawSnapshot
	certificate.SnapshotChecksum = snapshotChecksum
	_ = SendCertificateReadyEmail(ctx, db, cfg, course, certificate)
	return &certificate, true, nil
}

func SendCertificateReadyEmail(ctx context.Context, db *gorm.DB, cfg config.Config, course models.Course, certificate models.Certificate) error {
	var user models.User
	if err := db.WithContext(ctx).First(&user, certificate.UserID).Error; err != nil {
		return err
	}
	courseTitle := course.TitleEn
	if normalizeMailerLocale(user.Language) == "id" && strings.TrimSpace(course.TitleID) != "" {
		courseTitle = course.TitleID
	}
	return SendTransactionalTemplateEmail(ctx, db, EmailTemplateCertificate, "certificate", user, map[string]string{
		"course":          courseTitle,
		"certificate_url": localizedFrontendURL(cfg, user.Language, certificate.CertificateURL),
		"dashboard_url":   localizedFrontendURL(cfg, user.Language, "/dashboard"),
	})
}

func SubmitQuiz(ctx context.Context, db *gorm.DB, quizID, userID uint, answers []QuizAnswerInput) (map[string]any, error) {
	var quiz models.Quiz
	if err := db.WithContext(ctx).First(&quiz, quizID).Error; err != nil {
		return nil, err
	}
	var questions []models.QuizQuestion
	if err := db.WithContext(ctx).Where("quiz_id = ?", quizID).Find(&questions).Error; err != nil {
		return nil, err
	}
	var attempts int64
	if err := db.WithContext(ctx).Model(&models.QuizSubmission{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).Count(&attempts).Error; err != nil {
		return nil, err
	}
	if quiz.AttemptLimit > 0 && attempts >= int64(quiz.AttemptLimit) {
		return nil, ErrQuizAttemptLimitReached
	}
	correctQuestions := 0
	gradableQuestions := 0
	manualReview := false
	answerByQuestion := map[uint]QuizAnswerInput{}
	for _, answer := range answers {
		answer.TextAnswer = strings.TrimSpace(answer.TextAnswer)
		answerByQuestion[answer.QuestionID] = answer
	}
	for _, question := range questions {
		ok, gradable, manual, err := gradeQuestion(ctx, db, question, answerByQuestion[question.ID])
		if err != nil {
			return nil, err
		}
		if manual {
			manualReview = true
		}
		if !gradable {
			continue
		}
		gradableQuestions++
		if ok {
			correctQuestions++
		}
	}
	score := 0
	if gradableQuestions > 0 {
		score = (correctQuestions * 100) / gradableQuestions
	}
	submission := models.QuizSubmission{QuizID: quizID, UserID: userID, Score: score, Passed: !manualReview && score >= quiz.PassingScore, AttemptNumber: int(attempts) + 1, SubmittedAt: time.Now(), ManualReview: manualReview}
	if err := db.WithContext(ctx).Create(&submission).Error; err != nil {
		return nil, err
	}
	for _, answer := range answers {
		answerID := answer.AnswerID
		if answerID == 0 && len(answer.AnswerIDs) > 0 {
			answerID = answer.AnswerIDs[0]
		}
		textAnswer := strings.TrimSpace(answer.TextAnswer)
		if textAnswer == "" && len(answer.AnswerIDs) > 0 {
			ids := make([]string, 0, len(answer.AnswerIDs))
			for _, id := range answer.AnswerIDs {
				ids = append(ids, strconv.FormatUint(uint64(id), 10))
			}
			textAnswer = strings.Join(ids, ",")
		}
		row := models.QuizSubmissionAnswer{SubmissionID: submission.ID, QuestionID: answer.QuestionID, AnswerID: answerID, TextAnswer: textAnswer}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, err
		}
	}
	return map[string]any{"score": score, "passed": submission.Passed, "attempt_number": submission.AttemptNumber, "attempts": attempts + 1, "passing_score": quiz.PassingScore, "manual_review": manualReview, "review": quizAnswerReview(ctx, db, questions, answerByQuestion)}, nil
}

func sortLearningItems(items []orderedLearningItem) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Order < items[i].Order || (items[j].Order == items[i].Order && items[j].ID < items[i].ID) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func topicCompleted(ctx context.Context, db *gorm.DB, topicID, userID uint) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&models.TopicProgress{}).Where("topic_id = ? AND user_id = ? AND completed_at IS NOT NULL", topicID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func quizPassed(ctx context.Context, db *gorm.DB, quizID, userID uint) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&models.QuizSubmission{}).Where("quiz_id = ? AND user_id = ? AND passed = ?", quizID, userID, true).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func gradeQuestion(ctx context.Context, db *gorm.DB, question models.QuizQuestion, answer QuizAnswerInput) (bool, bool, bool, error) {
	questionType := strings.TrimSpace(question.Type)
	var options []models.QuizAnswer
	if err := db.WithContext(ctx).Where("question_id = ?", question.ID).Order("`order` asc, id asc").Find(&options).Error; err != nil {
		return false, false, false, err
	}
	correctOptions := make([]models.QuizAnswer, 0, len(options))
	for _, option := range options {
		if option.IsCorrect {
			correctOptions = append(correctOptions, option)
		}
	}
	switch questionType {
	case "assessment_survey", "survey":
		return true, false, false, nil
	case "essay":
		return false, false, true, nil
	case "free_choice", "short_answer", "fill_blank", "fill_in_blank":
		if len(correctOptions) == 0 {
			return false, true, true, nil
		}
		for _, option := range correctOptions {
			if sameText(answer.TextAnswer, option.AnswerEn) || sameText(answer.TextAnswer, option.AnswerID) {
				return true, true, false, nil
			}
		}
		return false, true, false, nil
	case "multiple_choice":
		return sameIDSet(answer.AnswerIDs, optionIDs(correctOptions)), true, false, nil
	case "sorting", "matrix_sorting":
		expected := optionIDs(correctOptions)
		if len(expected) == 0 {
			expected = optionIDs(options)
		}
		return sameIDOrder(answer.AnswerIDs, expected), true, false, nil
	default:
		if len(correctOptions) == 0 {
			return false, true, false, nil
		}
		return answer.AnswerID == correctOptions[0].ID, true, false, nil
	}
}

func quizAnswerReview(ctx context.Context, db *gorm.DB, questions []models.QuizQuestion, answers map[uint]QuizAnswerInput) []map[string]any {
	review := make([]map[string]any, 0, len(questions))
	for _, question := range questions {
		answer := answers[question.ID]
		ok, gradable, manual, _ := gradeQuestion(ctx, db, question, answer)
		var correct []models.QuizAnswer
		_ = db.WithContext(ctx).Where("question_id = ? AND is_correct = ?", question.ID, true).Order("`order` asc, id asc").Find(&correct).Error
		review = append(review, map[string]any{
			"question_id":         question.ID,
			"type":                question.Type,
			"selected_answer_id":  answer.AnswerID,
			"selected_answer_ids": answer.AnswerIDs,
			"text_answer":         answer.TextAnswer,
			"correct":             ok,
			"gradable":            gradable,
			"manual_review":       manual,
			"correct_answers":     correct,
		})
	}
	return review
}

func optionIDs(options []models.QuizAnswer) []uint {
	ids := make([]uint, 0, len(options))
	for _, option := range options {
		ids = append(ids, option.ID)
	}
	return ids
}

func sameIDSet(a []uint, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[uint]int{}
	for _, id := range a {
		counts[id]++
	}
	for _, id := range b {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	return true
}

func sameIDOrder(a []uint, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameText(a string, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
