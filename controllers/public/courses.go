package public

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) ListCourseCategories(c *gin.Context) {
	var categories []models.CourseCategory
	if err := h.db.WithContext(c.Request.Context()).Order("name_en asc").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load categories"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Categories loaded", Data: categories})
}

func (h *Controller) ListCourses(c *gin.Context) {
	var courses []models.Course
	query := h.db.WithContext(c.Request.Context()).Where("status IN ?", []string{"open", "closed"})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where("title_en LIKE ? OR title_id LIKE ? OR description_en LIKE ? OR description_id LIKE ?", like, like, like, like)
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" && category != "all" {
		var row models.CourseCategory
		if err := h.db.WithContext(c.Request.Context()).Where("slug = ?", category).First(&row).Error; err == nil {
			query = query.Where("category_id = ?", row.ID)
		}
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if price := strings.TrimSpace(c.Query("price")); price == "free" {
		query = query.Where("price = 0 OR is_free = ?", true)
	} else if price == "paid" {
		query = query.Where("price > 0 AND is_free = ?", false)
	}
	order := "created_at desc"
	switch c.Query("sort") {
	case "price_asc":
		order = "price asc"
	case "price_desc":
		order = "price desc"
	case "title":
		order = "title_en asc"
	}
	page := positiveInt(c.Query("page"), 1)
	perPage := positiveInt(c.Query("per_page"), 12)
	if perPage > 48 {
		perPage = 48
	}
	var total int64
	if err := query.Model(&models.Course{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count courses"})
		return
	}
	if err := query.Order(order).Limit(perPage).Offset((page - 1) * perPage).Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load courses"})
		return
	}
	items, err := BuildCourseSummaries(c.Request.Context(), h.db, courses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to format courses"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Courses loaded", Data: gin.H{"items": items, "page": page, "per_page": perPage, "total": total, "total_pages": totalPages(total, perPage)}})
}

func (h *Controller) GetCourse(c *gin.Context) {
	var course models.Course
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status IN ?", c.Param("slug"), []string{"open", "closed"}).First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return
	}
	reviewsPage := positiveInt(c.Query("reviews_page"), 1)
	detail, err := courseDetail(c.Request.Context(), h.db, course, 0, reviewsPage, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load course"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course loaded", Data: detail})
}

func BuildCourseDetail(ctx context.Context, db *gorm.DB, course models.Course, userID uint, reviewsPage int, reviewsPerPage int) (gin.H, error) {
	return courseDetail(ctx, db, course, userID, reviewsPage, reviewsPerPage)
}

func BuildCourseSummaries(ctx context.Context, db *gorm.DB, courses []models.Course) ([]gin.H, error) {
	items := make([]gin.H, 0, len(courses))
	for _, course := range courses {
		category, instructor, modulesCount, topicsCount, rating, reviews, err := courseMeta(ctx, db, course)
		if err != nil {
			return nil, err
		}
		items = append(items, gin.H{
			"id":                   course.ID,
			"title_en":             course.TitleEn,
			"title_id":             course.TitleID,
			"slug":                 course.Slug,
			"description_en":       course.DescriptionEn,
			"description_id":       course.DescriptionID,
			"learning_outcomes_en": course.LearningOutcomesEn,
			"learning_outcomes_id": course.LearningOutcomesID,
			"thumbnail":            course.Thumbnail,
			"price":                course.Price,
			"is_free":              course.IsFree || course.Price == 0,
			"status":               course.Status,
			"category":             category,
			"instructor":           instructor,
			"modules_count":        modulesCount,
			"topics_count":         topicsCount,
			"rating":               rating,
			"reviews_count":        reviews,
			"min_quiz_score":       course.MinQuizScore,
			"quiz_attempt_limit":   course.QuizAttemptLimit,
		})
	}
	return items, nil
}

func courseMeta(ctx context.Context, db *gorm.DB, course models.Course) (gin.H, gin.H, int64, int64, float64, int64, error) {
	var category models.CourseCategory
	if course.CategoryID != 0 {
		_ = db.WithContext(ctx).First(&category, course.CategoryID).Error
	}
	var instructor models.User
	if course.InstructorID != 0 {
		_ = db.WithContext(ctx).First(&instructor, course.InstructorID).Error
	}
	var instructorProfile models.UserProfile
	if instructor.ID != 0 {
		_ = db.WithContext(ctx).Where("user_id = ?", instructor.ID).First(&instructorProfile).Error
	}
	var modulesCount, topicsCount, reviews int64
	if err := db.WithContext(ctx).Model(&models.CourseModule{}).Where("course_id = ?", course.ID).Count(&modulesCount).Error; err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	if err := db.WithContext(ctx).
		Model(&models.Topic{}).
		Joins("JOIN course_modules m ON m.id = topics.module_id").
		Where("m.course_id = ?", course.ID).
		Count(&topicsCount).Error; err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	var ratingRow struct {
		Average float64
		Count   int64
	}
	if err := db.WithContext(ctx).
		Model(&models.Review{}).
		Select("COALESCE(AVG(rating), 0) AS average, COUNT(*) AS count").
		Where("reviewable_type = ? AND reviewable_id = ?", "course", course.ID).
		Scan(&ratingRow).Error; err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	reviews = ratingRow.Count
	return gin.H{"id": category.ID, "name_en": category.NameEn, "name_id": category.NameID, "slug": category.Slug},
		gin.H{
			"id":            instructor.ID,
			"name":          instructor.Name,
			"avatar":        instructor.Avatar,
			"bio_en":        instructorProfile.BioEn,
			"bio_id":        instructorProfile.BioID,
			"linkedin_url":  instructorProfile.LinkedinURL,
			"portfolio_url": instructorProfile.PortfolioURL,
			"skills":        instructorProfile.Skills,
		},
		modulesCount, topicsCount, ratingRow.Average, reviews, nil
}

func courseDetail(ctx context.Context, db *gorm.DB, course models.Course, userID uint, reviewsPage int, reviewsPerPage int) (gin.H, error) {
	summaries, err := BuildCourseSummaries(ctx, db, []models.Course{course})
	if err != nil {
		return nil, err
	}
	state, err := learningStateForCourse(ctx, db, course, userID)
	if err != nil {
		return nil, err
	}
	var modules []models.CourseModule
	if err := db.WithContext(ctx).Where("course_id = ?", course.ID).Order("`order` asc, id asc").Find(&modules).Error; err != nil {
		return nil, err
	}
	moduleItems := make([]gin.H, 0, len(modules))
	totalTopics := 0
	doneTopics := 0
	for _, module := range modules {
		var topics []models.Topic
		if err := db.WithContext(ctx).Where("module_id = ?", module.ID).Order("`order` asc, id asc").Find(&topics).Error; err != nil {
			return nil, err
		}
		var quizzes []models.Quiz
		_ = db.WithContext(ctx).Where("module_id = ?", module.ID).Order("`order` asc, id asc").Find(&quizzes).Error
		quizItems := make([]gin.H, 0, len(quizzes))
		for _, qz := range quizzes {
			item := quizPayload(ctx, db, qz, userID)
			item["locked"] = !state.QuizUnlocked[qz.ID]
			item["unlock_at"] = state.QuizUnlockAt[qz.ID]
			quizItems = append(quizItems, item)
		}
		topicItems := make([]gin.H, 0, len(topics))
		for _, topic := range topics {
			totalTopics++
			var progress models.TopicProgress
			done := false
			if userID != 0 {
				done = db.WithContext(ctx).Where(models.TopicProgress{UserID: userID, TopicID: topic.ID}).Where("completed_at IS NOT NULL").First(&progress).Error == nil
			}
			if done {
				doneTopics++
			}
			var attachments []models.TopicAttachment
			if err := db.WithContext(ctx).Where("topic_id = ?", topic.ID).Order("id asc").Find(&attachments).Error; err != nil {
				return nil, err
			}
			var blocks []models.TopicBlock
			if err := db.WithContext(ctx).Where("topic_id = ?", topic.ID).Order("`order` asc, id asc").Find(&blocks).Error; err != nil {
				return nil, err
			}
			assignments, err := assignmentPayload(ctx, db, topic.ID, userID)
			if err != nil {
				return nil, err
			}
			topicItems = append(topicItems, gin.H{
				"id":               topic.ID,
				"title_en":         topic.TitleEn,
				"title_id":         topic.TitleID,
				"content_en":       topic.ContentEn,
				"content_id":       topic.ContentID,
				"video_url":        topic.VideoURL,
				"duration_seconds": topic.DurationSeconds,
				"order":            topic.Order,
				"done":             done,
				"locked":           !state.TopicUnlocked[topic.ID],
				"unlock_at":        state.TopicUnlockAt[topic.ID],
				"attachments":      attachments,
				"blocks":           blocks,
				"assignments":      assignments,
			})
		}
		unlockAt := state.ModuleUnlockAt[module.ID]
		moduleItems = append(moduleItems, gin.H{
			"id":        module.ID,
			"title_en":  module.TitleEn,
			"title_id":  module.TitleID,
			"order":     module.Order,
			"locked":    !state.ModuleUnlocked[module.ID],
			"unlock_at": unlockAt,
			"topics":    topicItems,
			"quizzes":   quizItems,
		})
	}
	var reviews []models.Review
	if reviewsPage < 1 {
		reviewsPage = 1
	}
	if reviewsPerPage < 1 {
		reviewsPerPage = 5
	}
	var reviewsTotal int64
	if err := db.WithContext(ctx).Model(&models.Review{}).Where("reviewable_type = ? AND reviewable_id = ?", "course", course.ID).Count(&reviewsTotal).Error; err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Where("reviewable_type = ? AND reviewable_id = ?", "course", course.ID).Order("created_at desc").Limit(reviewsPerPage).Offset((reviewsPage - 1) * reviewsPerPage).Find(&reviews).Error; err != nil {
		return nil, err
	}
	reviewItems, err := formatReviews(ctx, db, reviews)
	if err != nil {
		return nil, err
	}
	progress := 0
	if state.TotalItems > 0 {
		progress = (state.CompletedItems * 100) / state.TotalItems
	} else if totalTopics > 0 {
		progress = (doneTopics * 100) / totalTopics
	}
	payload := summaries[0]
	addOns, err := courseAddonPayloads(ctx, db, course.ID, true, state.Enrolled)
	if err != nil {
		return nil, err
	}
	payload["modules"] = moduleItems
	payload["add_ons"] = addOns
	payload["reviews"] = reviewItems
	payload["reviews_page"] = reviewsPage
	payload["reviews_per_page"] = reviewsPerPage
	payload["reviews_total"] = reviewsTotal
	payload["reviews_total_pages"] = totalPages(reviewsTotal, reviewsPerPage)
	payload["reviews_breakdown"] = reviewBreakdown(ctx, db, course.ID)
	payload["progress"] = progress
	payload["enrolled"] = state.Enrolled
	payload["course_completed"] = state.CourseCompleted
	if userID != 0 {
		var certificate models.Certificate
		if err := db.WithContext(ctx).Where(models.Certificate{UserID: userID, CourseID: course.ID}).First(&certificate).Error; err == nil {
			if err := services.EnsureCertificateUUID(ctx, db, &certificate); err != nil {
				return nil, err
			}
			certificate.CertificateURL = services.CertificateDownloadURL(certificate)
			payload["certificate"] = certificate
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return payload, nil
}

func CourseAddonPayloads(ctx context.Context, db *gorm.DB, courseID uint, activeOnly bool) ([]gin.H, error) {
	return courseAddonPayloads(ctx, db, courseID, activeOnly, true)
}

func courseAddonPayloads(ctx context.Context, db *gorm.DB, courseID uint, activeOnly bool, includeDelivery bool) ([]gin.H, error) {
	var rows []models.CourseAddon
	query := db.WithContext(ctx).Where("course_id = ?", courseID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	if err := query.Order("`order` asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		var category models.ProductCategory
		if row.ProductCategoryID != 0 {
			_ = db.WithContext(ctx).First(&category, row.ProductCategoryID).Error
		}
		filePath := ""
		externalURL := ""
		if includeDelivery {
			filePath = row.FilePath
			externalURL = row.ExternalURL
		}
		items = append(items, gin.H{
			"id":                    row.ID,
			"course_id":             row.CourseID,
			"product_category_id":   row.ProductCategoryID,
			"title_en":              row.TitleEn,
			"title_id":              row.TitleID,
			"description_en":        row.DescriptionEn,
			"description_id":        row.DescriptionID,
			"type":                  row.Type,
			"file_path":             filePath,
			"external_url":          externalURL,
			"order":                 row.Order,
			"is_active":             row.IsActive,
			"category_slug":         category.Slug,
			"category_name_en":      category.NameEn,
			"category_name_id":      category.NameID,
			"requires_booking_time": category.RequiresBookingTime,
		})
	}
	return items, nil
}

type learningState struct {
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

func learningStateForCourse(ctx context.Context, db *gorm.DB, course models.Course, userID uint) (learningState, error) {
	state := learningState{
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

func assignmentPayload(ctx context.Context, db *gorm.DB, topicID, userID uint) ([]gin.H, error) {
	var assignments []models.Assignment
	if err := db.WithContext(ctx).Where("topic_id = ?", topicID).Order("id asc").Find(&assignments).Error; err != nil {
		return nil, err
	}
	items := make([]gin.H, 0, len(assignments))
	for _, assignment := range assignments {
		row := gin.H{
			"id":             assignment.ID,
			"topic_id":       assignment.TopicID,
			"title_en":       assignment.TitleEn,
			"title_id":       assignment.TitleID,
			"description_en": assignment.DescriptionEn,
			"description_id": assignment.DescriptionID,
		}
		if userID != 0 {
			var submission models.AssignmentSubmission
			if err := db.WithContext(ctx).Where(models.AssignmentSubmission{AssignmentID: assignment.ID, UserID: userID}).Order("submitted_at desc, id desc").First(&submission).Error; err == nil {
				row["submission"] = submission
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
		items = append(items, row)
	}
	return items, nil
}

func reviewBreakdown(ctx context.Context, db *gorm.DB, courseID uint) gin.H {
	breakdown := gin.H{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	type ratingCount struct {
		Rating int
		Count  int64
	}
	var rows []ratingCount
	_ = db.WithContext(ctx).
		Model(&models.Review{}).
		Select("rating, COUNT(*) AS count").
		Where("reviewable_type = ? AND reviewable_id = ?", "course", courseID).
		Group("rating").
		Scan(&rows).Error
	for _, row := range rows {
		breakdown[strconv.Itoa(row.Rating)] = row.Count
	}
	return breakdown
}

func quizPayload(ctx context.Context, db *gorm.DB, quiz models.Quiz, userID uint) gin.H {
	if quiz.ID == 0 {
		return nil
	}
	var questions []models.QuizQuestion
	_ = db.WithContext(ctx).Where("quiz_id = ?", quiz.ID).Order("`order` asc, id asc").Find(&questions).Error
	questionItems := make([]gin.H, 0, len(questions))
	for _, question := range questions {
		var answers []models.QuizAnswer
		_ = db.WithContext(ctx).Where("question_id = ?", question.ID).Order("`order` asc, id asc").Find(&answers).Error
		questionItems = append(questionItems, gin.H{
			"id":          question.ID,
			"quiz_id":     question.QuizID,
			"type":        question.Type,
			"question_en": question.QuestionEn,
			"question_id": question.QuestionID,
			"order":       question.Order,
			"answers":     answers,
		})
	}
	var attempts int64
	passed := false
	bestScore := 0
	if userID != 0 {
		_ = db.WithContext(ctx).Model(&models.QuizSubmission{}).Where("quiz_id = ? AND user_id = ?", quiz.ID, userID).Count(&attempts).Error
		var latest models.QuizSubmission
		latestLoaded := db.WithContext(ctx).
			Where("quiz_id = ? AND user_id = ?", quiz.ID, userID).
			Order("submitted_at desc, id desc").
			First(&latest).Error == nil
		var best models.QuizSubmission
		if err := db.WithContext(ctx).
			Where("quiz_id = ? AND user_id = ?", quiz.ID, userID).
			Order("score desc, submitted_at desc").
			First(&best).Error; err == nil {
			bestScore = best.Score
		}
		var passedCount int64
		_ = db.WithContext(ctx).Model(&models.QuizSubmission{}).Where("quiz_id = ? AND user_id = ? AND passed = ?", quiz.ID, userID, true).Count(&passedCount).Error
		passed = passedCount > 0
		if latestLoaded {
			return gin.H{
				"id":                   quiz.ID,
				"module_id":            quiz.ModuleID,
				"title_en":             quiz.TitleEn,
				"title_id":             quiz.TitleID,
				"passing_score":        quiz.PassingScore,
				"attempt_limit":        quiz.AttemptLimit,
				"order":                quiz.Order,
				"attempts":             attempts,
				"passed":               passed,
				"best_score":           bestScore,
				"latest_score":         latest.Score,
				"latest_attempt":       latest.AttemptNumber,
				"latest_passed":        latest.Passed,
				"pending_review":       latest.ManualReview && latest.ReviewedAt == nil,
				"latest_manual_review": latest.ManualReview,
				"questions":            questionItems,
			}
		}
	}
	return gin.H{
		"id":            quiz.ID,
		"module_id":     quiz.ModuleID,
		"title_en":      quiz.TitleEn,
		"title_id":      quiz.TitleID,
		"passing_score": quiz.PassingScore,
		"attempt_limit": quiz.AttemptLimit,
		"order":         quiz.Order,
		"attempts":      attempts,
		"passed":        passed,
		"best_score":    bestScore,
		"questions":     questionItems,
	}
}

func formatReviews(ctx context.Context, db *gorm.DB, reviews []models.Review) ([]gin.H, error) {
	userIDs := make([]uint, 0, len(reviews))
	for _, review := range reviews {
		if review.UserID != 0 {
			userIDs = append(userIDs, review.UserID)
		}
	}
	usersByID := map[uint]models.User{}
	if len(userIDs) > 0 {
		var users []models.User
		if err := db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			usersByID[user.ID] = user
		}
	}

	items := make([]gin.H, 0, len(reviews))
	for _, review := range reviews {
		user := usersByID[review.UserID]
		items = append(items, gin.H{
			"id":         review.ID,
			"user_id":    review.UserID,
			"rating":     review.Rating,
			"comment":    review.Comment,
			"created_at": review.CreatedAt,
			"reviewer": gin.H{
				"id":     user.ID,
				"name":   user.Name,
				"avatar": user.Avatar,
			},
		})
	}
	return items, nil
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func totalPages(total int64, perPage int) int {
	if perPage <= 0 || total == 0 {
		return 1
	}
	return int((total + int64(perPage) - 1) / int64(perPage))
}
