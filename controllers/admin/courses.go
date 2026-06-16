package admin

import (
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	publiccontroller "academyprometheus/backend/controllers/public"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
)

func (h *Controller) CreateCourseCategory(c *gin.Context) {
	var row models.CourseCategory
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid category payload"})
		return
	}
	if strings.TrimSpace(row.Slug) == "" {
		row.Slug = services.GenerateSlug(row.NameEn)
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create category"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Category created", Data: row})
}

func (h *Controller) UpdateCourseCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid category id"})
		return
	}
	var row models.CourseCategory
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid category payload"})
		return
	}
	if strings.TrimSpace(row.Slug) == "" {
		row.Slug = services.GenerateSlug(row.NameEn)
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.CourseCategory{}).Where("id = ?", uint(id)).Select("name_en", "name_id", "slug").Updates(row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save category"})
		return
	}
	row.ID = uint(id)
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Category saved", Data: row})
}

func (h *Controller) ListCourseCategories(c *gin.Context) {
	listRows[models.CourseCategory](h.db, "name_en asc", "Course categories loaded")(c)
}

func (h *Controller) DeleteCourseCategory(c *gin.Context) {
	deleteRow[models.CourseCategory](h.db, "Category deleted")(c)
}

func (h *Controller) ListCourses(c *gin.Context) {
	var courses []models.Course
	query := h.db.WithContext(c.Request.Context()).Model(&models.Course{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where("title_en LIKE ? OR title_id LIKE ?", like, like)
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" && category != "all" {
		query = query.Where("category_id = ?", category)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if err := query.Order("created_at desc").Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load courses"})
		return
	}
	items, err := publiccontroller.BuildCourseSummaries(c.Request.Context(), h.db, courses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to format courses"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Courses loaded", Data: items})
}

func (h *Controller) GetCourse(c *gin.Context) {
	course, ok := h.loadCourseByID(c)
	if !ok {
		return
	}
	detail, err := publiccontroller.BuildCourseDetail(c.Request.Context(), h.db, course, 0, 1, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load course"})
		return
	}
	var drips []models.DripSchedule
	_ = h.db.WithContext(c.Request.Context()).Where("course_id = ?", course.ID).Order("id asc").Find(&drips).Error
	detail["drip_schedules"] = drips
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course loaded", Data: detail})
}

func (h *Controller) CreateCourse(c *gin.Context) {
	h.saveCourse(c, true)
}

func (h *Controller) UpdateCourse(c *gin.Context) {
	h.saveCourse(c, false)
}

func (h *Controller) DeleteCourse(c *gin.Context) {
	deleteRow[models.Course](h.db, "Course deleted")(c)
}

func (h *Controller) UpdateCourseThumbnail(c *gin.Context) {
	h.updateUploadField(c, h.uploadService.SaveCourseThumbnail, &models.Course{}, "thumbnail", "Course thumbnail uploaded")
}

func (h *Controller) CreateCourseModule(c *gin.Context) {
	course, ok := h.loadCourseByID(c)
	if !ok {
		return
	}
	var req models.CourseModule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid module payload"})
		return
	}
	req.CourseID = course.ID
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create module"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Module created", Data: req})
}

func (h *Controller) UpdateCourseModule(c *gin.Context) {
	updateRow[models.CourseModule](h.db, "Module saved")(c)
}

func (h *Controller) DeleteCourseModule(c *gin.Context) {
	deleteRow[models.CourseModule](h.db, "Module deleted")(c)
}

func (h *Controller) ReorderCourseModules(c *gin.Context) {
	reorderRows[models.CourseModule](h.db, "Modules reordered")(c)
}

func (h *Controller) CreateTopic(c *gin.Context) {
	moduleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || moduleID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid module id"})
		return
	}
	var req models.Topic
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic payload"})
		return
	}
	req.ModuleID = uint(moduleID)
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create topic"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Topic created", Data: req})
}

func (h *Controller) UpdateTopic(c *gin.Context) {
	updateRow[models.Topic](h.db, "Topic saved")(c)
}

func (h *Controller) DeleteTopic(c *gin.Context) {
	deleteRow[models.Topic](h.db, "Topic deleted")(c)
}

func (h *Controller) ReorderTopics(c *gin.Context) {
	reorderRows[models.Topic](h.db, "Topics reordered")(c)
}

func (h *Controller) CreateTopicAttachment(c *gin.Context) {
	topicID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || topicID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic id"})
		return
	}
	var req models.TopicAttachment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid attachment payload"})
		return
	}
	req.TopicID = uint(topicID)
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create attachment"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Attachment created", Data: req})
}

func (h *Controller) DeleteTopicAttachment(c *gin.Context) {
	deleteRow[models.TopicAttachment](h.db, "Attachment deleted")(c)
}

func (h *Controller) CreateAssignment(c *gin.Context) {
	topicID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || topicID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic id"})
		return
	}
	var req models.Assignment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid assignment payload"})
		return
	}
	req.TopicID = uint(topicID)
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create assignment"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Assignment created", Data: req})
}

func (h *Controller) UpdateAssignment(c *gin.Context) {
	updateRow[models.Assignment](h.db, "Assignment saved")(c)
}

func (h *Controller) DeleteAssignment(c *gin.Context) {
	deleteRow[models.Assignment](h.db, "Assignment deleted")(c)
}

func (h *Controller) ListAssignmentSubmissions(c *gin.Context) {
	assignmentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || assignmentID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid assignment id"})
		return
	}
	var rows []models.AssignmentSubmission
	if err := h.db.WithContext(c.Request.Context()).Where("assignment_id = ?", uint(assignmentID)).Order("submitted_at desc, id desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load submissions"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Assignment submissions loaded", Data: rows})
}

func (h *Controller) UpdateAssignmentSubmission(c *gin.Context) {
	updateRow[models.AssignmentSubmission](h.db, "Assignment submission graded")(c)
}

func (h *Controller) CreateTopicBlock(c *gin.Context) {
	topicID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || topicID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic id"})
		return
	}
	var req models.TopicBlock
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid block payload"})
		return
	}
	req.TopicID = uint(topicID)
	if strings.TrimSpace(req.Type) == "" {
		req.Type = "text"
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create block"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Block created", Data: req})
}

func (h *Controller) UpdateTopicBlock(c *gin.Context) {
	updateRow[models.TopicBlock](h.db, "Block saved")(c)
}

func (h *Controller) DeleteTopicBlock(c *gin.Context) {
	deleteRow[models.TopicBlock](h.db, "Block deleted")(c)
}

func (h *Controller) ReorderTopicBlocks(c *gin.Context) {
	reorderRows[models.TopicBlock](h.db, "Blocks reordered")(c)
}

func (h *Controller) UpdateTopicBlockFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid block id"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "File is required"})
		return
	}
	path, originalName, err := h.uploadService.SaveTopicBlockFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	fileType := file.Header.Get("Content-Type")
	updates := map[string]any{"file_path": path, "file_name": originalName, "file_type": fileType}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.TopicBlock{}).Where("id = ?", uint(id)).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update block"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "File uploaded", Data: gin.H{"file_path": path, "file_name": originalName, "file_type": fileType}})
}

func (h *Controller) CreateQuiz(c *gin.Context) {
	moduleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || moduleID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid module id"})
		return
	}
	var req models.Quiz
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz payload"})
		return
	}
	req.ID = 0
	req.ModuleID = uint(moduleID)
	if req.PassingScore == 0 {
		req.PassingScore = 70
	}
	if req.AttemptLimit == 0 {
		req.AttemptLimit = 3
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create quiz"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz created", Data: req})
}

func (h *Controller) UpdateModuleQuiz(c *gin.Context) {
	moduleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || moduleID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid module id"})
		return
	}
	var req models.Quiz
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz payload"})
		return
	}
	req.ModuleID = uint(moduleID)
	if req.PassingScore == 0 {
		req.PassingScore = 70
	}
	if req.AttemptLimit == 0 {
		req.AttemptLimit = 3
	}
	if err := h.db.WithContext(c.Request.Context()).Where(models.Quiz{ModuleID: uint(moduleID)}).Assign(req).FirstOrCreate(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save quiz"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz saved", Data: req})
}

func (h *Controller) UpdateQuiz(c *gin.Context) {
	updateRow[models.Quiz](h.db, "Quiz saved")(c)
}

func (h *Controller) DeleteQuiz(c *gin.Context) {
	deleteRow[models.Quiz](h.db, "Quiz deleted")(c)
}

func (h *Controller) ReorderQuizzes(c *gin.Context) {
	reorderRows[models.Quiz](h.db, "Quizzes reordered")(c)
}

func (h *Controller) CreateQuizQuestion(c *gin.Context) {
	quizID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || quizID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz id"})
		return
	}
	var req models.QuizQuestion
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid question payload"})
		return
	}
	req.QuizID = uint(quizID)
	if strings.TrimSpace(req.Type) == "" {
		req.Type = "single_choice"
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create question"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Question created", Data: req})
}

func (h *Controller) UpdateQuizQuestion(c *gin.Context) {
	updateRow[models.QuizQuestion](h.db, "Question saved")(c)
}

func (h *Controller) DeleteQuizQuestion(c *gin.Context) {
	deleteRow[models.QuizQuestion](h.db, "Question deleted")(c)
}

func (h *Controller) CreateQuizAnswer(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || questionID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid question id"})
		return
	}
	var req models.QuizAnswer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid answer payload"})
		return
	}
	req.QuestionID = uint(questionID)
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create answer"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Answer created", Data: req})
}

func (h *Controller) UpdateQuizAnswer(c *gin.Context) {
	updateRow[models.QuizAnswer](h.db, "Answer saved")(c)
}

func (h *Controller) DeleteQuizAnswer(c *gin.Context) {
	deleteRow[models.QuizAnswer](h.db, "Answer deleted")(c)
}

func (h *Controller) ListQuizSubmissions(c *gin.Context) {
	quizID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || quizID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz id"})
		return
	}
	var rows []models.QuizSubmission
	query := h.db.WithContext(c.Request.Context()).Where("quiz_id = ?", uint(quizID))
	if c.Query("manual") == "1" {
		query = query.Where("manual_review = ?", true)
	}
	if err := query.Order("submitted_at desc, id desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load quiz submissions"})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		var user models.User
		_ = h.db.WithContext(c.Request.Context()).First(&user, row.UserID).Error
		var answers []models.QuizSubmissionAnswer
		_ = h.db.WithContext(c.Request.Context()).Where("submission_id = ?", row.ID).Order("id asc").Find(&answers).Error
		answerItems := make([]gin.H, 0, len(answers))
		for _, answer := range answers {
			var question models.QuizQuestion
			_ = h.db.WithContext(c.Request.Context()).First(&question, answer.QuestionID).Error
			var selected models.QuizAnswer
			if answer.AnswerID != 0 {
				_ = h.db.WithContext(c.Request.Context()).First(&selected, answer.AnswerID).Error
			}
			selectedAnswers := []models.QuizAnswer{}
			if strings.TrimSpace(answer.TextAnswer) != "" {
				ids := []uint{}
				for _, rawID := range strings.Split(answer.TextAnswer, ",") {
					id, err := strconv.ParseUint(strings.TrimSpace(rawID), 10, 64)
					if err == nil && id > 0 {
						ids = append(ids, uint(id))
					}
				}
				if len(ids) > 0 {
					_ = h.db.WithContext(c.Request.Context()).Where("id IN ?", ids).Find(&selectedAnswers).Error
				}
			}
			answerItems = append(answerItems, gin.H{
				"id":               answer.ID,
				"question_id":      answer.QuestionID,
				"answer_id":        answer.AnswerID,
				"text_answer":      answer.TextAnswer,
				"question":         question,
				"answer":           selected,
				"selected_answers": selectedAnswers,
			})
		}
		items = append(items, gin.H{
			"id":             row.ID,
			"quiz_id":        row.QuizID,
			"user_id":        row.UserID,
			"user_name":      user.Name,
			"user_email":     user.Email,
			"score":          row.Score,
			"passed":         row.Passed,
			"attempt_number": row.AttemptNumber,
			"submitted_at":   row.SubmittedAt,
			"manual_review":  row.ManualReview,
			"reviewed_at":    row.ReviewedAt,
			"reviewed_by":    row.ReviewedBy,
			"feedback":       row.Feedback,
			"answers":        answerItems,
		})
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz submissions loaded", Data: items})
}

func (h *Controller) ReviewQuizSubmission(c *gin.Context) {
	submissionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || submissionID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid submission id"})
		return
	}
	var req struct {
		Score    int    `json:"score"`
		Passed   bool   `json:"passed"`
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid review payload"})
		return
	}
	if req.Score < 0 {
		req.Score = 0
	}
	if req.Score > 100 {
		req.Score = 100
	}
	var submission models.QuizSubmission
	if err := h.db.WithContext(c.Request.Context()).First(&submission, uint(submissionID)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Submission not found"})
		return
	}
	var quiz models.Quiz
	if err := h.db.WithContext(c.Request.Context()).First(&quiz, submission.QuizID).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Quiz not found"})
		return
	}
	now := time.Now()
	reviewerID := uint(0)
	if value, exists := c.Get("user_id"); exists {
		if id, ok := value.(uint); ok {
			reviewerID = id
		}
	}
	updates := map[string]any{
		"score":       req.Score,
		"passed":      req.Passed,
		"reviewed_at": &now,
		"reviewed_by": reviewerID,
		"feedback":    strings.TrimSpace(req.Feedback),
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&submission).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save review"})
		return
	}
	submission.Score = req.Score
	submission.Passed = req.Passed
	submission.ReviewedAt = &now
	submission.ReviewedBy = reviewerID
	submission.Feedback = strings.TrimSpace(req.Feedback)

	var certificate *models.Certificate
	completed := false
	if submission.Passed {
		if course, err := services.CourseForModuleItem(c.Request.Context(), h.db, quiz.ModuleID); err == nil {
			certificate, completed, err = services.SyncCourseCompletion(c.Request.Context(), h.db, h.cfg, course, submission.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Review saved, but course completion sync failed"})
				return
			}
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz submission reviewed", Data: gin.H{"submission": submission, "course_completed": completed, "certificate": certificate}})
}

func (h *Controller) UpdateCourseDripSchedules(c *gin.Context) {
	course, ok := h.loadCourseByID(c)
	if !ok {
		return
	}
	var req []models.DripSchedule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid drip schedule payload"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Where("course_id = ?", course.ID).Delete(&models.DripSchedule{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to clear drip schedules"})
		return
	}
	for _, item := range req {
		item.CourseID = course.ID
		if item.ModuleID == 0 {
			continue
		}
		if err := h.db.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save drip schedules"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Drip schedules saved"})
}

func (h *Controller) saveCourse(c *gin.Context, creating bool) {
	var req models.Course
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid course payload"})
		return
	}
	ignoreID := uint(0)
	if !creating {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid course id"})
			return
		}
		ignoreID = uint(id)
		req.ID = uint(id)
	}
	if strings.TrimSpace(req.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "courses", req.TitleEn, ignoreID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		req.Slug = slug
	}
	req.IsFree = req.Price == 0
	if strings.TrimSpace(req.Status) == "" {
		req.Status = "draft"
	}
	if creating {
		if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create course"})
			return
		}
	} else if err := h.db.WithContext(c.Request.Context()).Model(&models.Course{}).Where("id = ?", req.ID).Select("*").Omit("id", "created_at").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save course"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course saved", Data: req})
}

func (h *Controller) loadCourseByID(c *gin.Context) (models.Course, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid course id"})
		return models.Course{}, false
	}
	var course models.Course
	if err := h.db.WithContext(c.Request.Context()).First(&course, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return models.Course{}, false
	}
	return course, true
}

func (h *Controller) updateUploadField(c *gin.Context, save func(*multipart.FileHeader) (string, error), model any, field string, message string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "File is required"})
		return
	}
	path, err := save(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(model).Where("id = ?", uint(id)).Update(field, path).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update file"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: message, Data: gin.H{field: path}})
}
