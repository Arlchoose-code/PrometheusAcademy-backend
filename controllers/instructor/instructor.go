package instructor

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	publiccontroller "academyprometheus/backend/controllers/public"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Controller struct {
	db                   *gorm.DB
	communicationService *services.CommunicationService
	uploadService        *services.UploadService
}

func NewController(db *gorm.DB, uploadService *services.UploadService) *Controller {
	return &Controller{db: db, communicationService: services.NewCommunicationService(db), uploadService: uploadService}
}

func (h *Controller) GetOverview(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	ctx := c.Request.Context()
	rows := make([]structs.InstructorCourseSummary, 0)
	if err := h.db.WithContext(ctx).Raw(`
		SELECT c.id, c.title_en, c.title_id, c.slug, c.thumbnail, c.status,
			(SELECT COUNT(DISTINCT ce.user_id) FROM course_enrollments ce WHERE ce.course_id = c.id) AS students,
			(SELECT COUNT(*) FROM course_conversations cc WHERE cc.course_id = c.id) AS conversations,
			(SELECT COALESCE(SUM(cc.staff_unread), 0) FROM course_conversations cc WHERE cc.course_id = c.id) AS unread
		FROM courses c
		WHERE c.instructor_id = ?
		ORDER BY c.updated_at DESC
	`, user.ID).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load instructor courses"})
		return
	}

	stats := structs.InstructorStats{AssignedCourses: int64(len(rows))}
	if err := h.db.WithContext(ctx).Table("course_enrollments AS enrollments").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("courses.instructor_id = ?", user.ID).
		Distinct("enrollments.user_id").Count(&stats.Students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count instructor students"})
		return
	}
	if err := h.db.WithContext(ctx).Table("course_conversations AS conversations").
		Joins("JOIN courses ON courses.id = conversations.course_id").
		Where("courses.instructor_id = ? AND conversations.status = ?", user.ID, "open").
		Count(&stats.OpenConversations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count conversations"})
		return
	}
	if err := h.db.WithContext(ctx).Table("course_conversations AS conversations").
		Joins("JOIN courses ON courses.id = conversations.course_id").
		Where("courses.instructor_id = ?", user.ID).
		Select("COALESCE(SUM(conversations.staff_unread), 0)").Scan(&stats.UnreadMessages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count unread messages"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Instructor overview loaded", Data: structs.InstructorOverviewResponse{Stats: stats, Courses: rows}})
}

func (h *Controller) ListCourses(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var courses []models.Course
	if err := h.db.WithContext(c.Request.Context()).Where("instructor_id = ?", user.ID).Order("created_at desc").Find(&courses).Error; err != nil {
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
	course, ok := h.loadOwnedCourseByID(c)
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
	addOns, err := publiccontroller.CourseAddonPayloads(c.Request.Context(), h.db, course.ID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load course add-ons"})
		return
	}
	detail["add_ons"] = addOns
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course loaded", Data: detail})
}

func (h *Controller) CreateCourse(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var req models.Course
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid course payload"})
		return
	}
	req.ID = 0
	req.InstructorID = user.ID
	req.IsFree = req.Price == 0
	req.Status = instructorCourseStatus(req.Status)
	if strings.TrimSpace(req.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "courses", req.TitleEn, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		req.Slug = slug
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create course"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course saved", Data: req})
}

func (h *Controller) UpdateCourse(c *gin.Context) {
	course, ok := h.loadOwnedCourseByID(c)
	if !ok {
		return
	}
	var req models.Course
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid course payload"})
		return
	}
	req.ID = course.ID
	req.InstructorID = course.InstructorID
	req.IsFree = req.Price == 0
	req.Status = instructorCourseStatus(req.Status)
	if course.Status == "open" || course.Status == "closed" {
		req.Status = course.Status
	}
	if strings.TrimSpace(req.Slug) == "" {
		slug, err := services.UniqueSlug(c.Request.Context(), h.db, "courses", req.TitleEn, course.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to generate slug"})
			return
		}
		req.Slug = slug
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Course{}).Where("id = ?", course.ID).Select("*").Omit("id", "created_at", "instructor_id").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save course"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course saved", Data: req})
}

func (h *Controller) DeleteCourse(c *gin.Context) {
	course, ok := h.loadOwnedCourseByID(c)
	if !ok {
		return
	}
	if course.Status != "draft" && course.Status != "pending" {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Published courses can only be deleted by admin"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.Course{}, course.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete course"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Course deleted"})
}

func (h *Controller) UpdateCourseThumbnail(c *gin.Context) {
	course, ok := h.loadOwnedCourseByID(c)
	if !ok {
		return
	}
	h.updateOwnedUpload(c, h.uploadService.SaveCourseThumbnail, &models.Course{}, course.ID, "thumbnail", "Course thumbnail uploaded")
}

func (h *Controller) ListMedia(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var rows []models.MediaFile
	if err := h.db.WithContext(c.Request.Context()).Where("uploaded_by = ?", user.ID).Order("created_at desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load media"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Media files loaded", Data: rows})
}

func (h *Controller) CreateMedia(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Media file is required"})
		return
	}
	media, err := h.uploadService.SaveMediaFile(c.Request.Context(), user.ID, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Media uploaded", Data: media})
}

func (h *Controller) DeleteMedia(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	result := h.db.WithContext(c.Request.Context()).Where("id = ? AND uploaded_by = ?", id, user.ID).Delete(&models.MediaFile{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete media"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Media not found"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Media deleted"})
}

func (h *Controller) ListConsultationSlots(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	ownerID := user.ID
	slots, err := services.ListConsultationSlots(c.Request.Context(), h.db, &ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load slots"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slots loaded", Data: slots})
}

func (h *Controller) CreateConsultationSlot(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var req services.ConsultationSlotPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot payload"})
		return
	}
	slot, err := services.ConsultationSlotFromPayload(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Date, start time, and end time are required"})
		return
	}
	slot.ID = 0
	slot.OwnerID = user.ID
	slot.Capacity = 1
	if err := services.CreateConsultationSlotRecord(c.Request.Context(), h.db, &slot); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot created", Data: slot})
}

func (h *Controller) UpdateConsultationSlot(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var current models.ConsultationSlot
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND owner_id = ?", id, user.ID).First(&current).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Slot not found"})
		return
	}
	var req services.ConsultationSlotPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid slot payload"})
		return
	}
	slot, err := services.ConsultationSlotFromPayload(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Date, start time, and end time are required"})
		return
	}
	slot.ID = current.ID
	slot.OwnerID = user.ID
	slot.Capacity = 1
	ownerID := user.ID
	if err := services.UpdateConsultationSlotRecord(c.Request.Context(), h.db, current.ID, &ownerID, slot); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot saved", Data: slot})
}

func (h *Controller) DeleteConsultationSlot(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	ownerID := user.ID
	if err := services.DeleteConsultationSlot(c.Request.Context(), h.db, id, &ownerID); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation slot deleted"})
}

func (h *Controller) ListConsultationBookings(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	ownerID := user.ID
	bookings, err := services.ConsultationBookingRowsForOwner(c.Request.Context(), h.db, 0, &ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load bookings"})
		return
	}
	services.ApplyConsultationRescheduleLimit(bookings, services.ConsultationRescheduleLimit(c.Request.Context(), h.db))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation bookings loaded", Data: bookings})
}

func (h *Controller) UpdateConsultationBooking(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid booking payload"})
		return
	}
	ownerID := user.ID
	if err := services.UpdateConsultationBookingByProvider(c.Request.Context(), h.db, id, &ownerID, strings.TrimSpace(req.Status), req.Notes); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Consultation booking saved"})
}

func (h *Controller) CreateCourseModule(c *gin.Context) {
	course, ok := h.loadOwnedCourseByID(c)
	if !ok {
		return
	}
	var req models.CourseModule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid module payload"})
		return
	}
	req.ID = 0
	req.CourseID = course.ID
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create module"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Module created", Data: req})
}

func (h *Controller) UpdateCourseModule(c *gin.Context) {
	module, ok := h.loadOwnedModule(c, "id")
	if !ok {
		return
	}
	var req models.CourseModule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid module payload"})
		return
	}
	req.ID = module.ID
	req.CourseID = module.CourseID
	if err := h.db.WithContext(c.Request.Context()).Model(&models.CourseModule{}).Where("id = ?", module.ID).Select("title_en", "title_id", "order").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save module"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Module saved", Data: req})
}

func (h *Controller) DeleteCourseModule(c *gin.Context) {
	module, ok := h.loadOwnedModule(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.CourseModule{}, module.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete module"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Module deleted"})
}

func (h *Controller) ReorderCourseModules(c *gin.Context) {
	var rows []struct {
		ID    uint `json:"id"`
		Order int  `json:"order"`
	}
	if err := c.ShouldBindJSON(&rows); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid reorder payload"})
		return
	}
	for _, row := range rows {
		module, ok := h.loadOwnedModuleByID(c, row.ID)
		if !ok {
			return
		}
		if err := h.db.WithContext(c.Request.Context()).Model(&module).Update("order", row.Order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reorder modules"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Modules reordered"})
}

func (h *Controller) CreateTopic(c *gin.Context) {
	module, ok := h.loadOwnedModule(c, "id")
	if !ok {
		return
	}
	var req models.Topic
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic payload"})
		return
	}
	req.ID = 0
	req.ModuleID = module.ID
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create topic"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Topic created", Data: req})
}

func (h *Controller) UpdateTopic(c *gin.Context) {
	topic, ok := h.loadOwnedTopic(c, "id")
	if !ok {
		return
	}
	var req models.Topic
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic payload"})
		return
	}
	req.ID = topic.ID
	req.ModuleID = topic.ModuleID
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Topic{}).Where("id = ?", topic.ID).Select("*").Omit("id", "created_at", "module_id").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save topic"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Topic saved", Data: req})
}

func (h *Controller) DeleteTopic(c *gin.Context) {
	topic, ok := h.loadOwnedTopic(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.Topic{}, topic.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete topic"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Topic deleted"})
}

func (h *Controller) ReorderTopics(c *gin.Context) {
	var rows []struct {
		ID    uint `json:"id"`
		Order int  `json:"order"`
	}
	if err := c.ShouldBindJSON(&rows); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid reorder payload"})
		return
	}
	for _, row := range rows {
		topic, ok := h.loadOwnedTopicByID(c, row.ID)
		if !ok {
			return
		}
		if err := h.db.WithContext(c.Request.Context()).Model(&topic).Update("order", row.Order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reorder topics"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Topics reordered"})
}

func (h *Controller) CreateTopicBlock(c *gin.Context) {
	topic, ok := h.loadOwnedTopic(c, "id")
	if !ok {
		return
	}
	var req models.TopicBlock
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid block payload"})
		return
	}
	req.ID = 0
	req.TopicID = topic.ID
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
	block, ok := h.loadOwnedBlock(c, "id")
	if !ok {
		return
	}
	var req models.TopicBlock
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid block payload"})
		return
	}
	req.ID = block.ID
	req.TopicID = block.TopicID
	if err := h.db.WithContext(c.Request.Context()).Model(&models.TopicBlock{}).Where("id = ?", block.ID).Select("*").Omit("id", "created_at", "topic_id").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save block"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Block saved", Data: req})
}

func (h *Controller) DeleteTopicBlock(c *gin.Context) {
	block, ok := h.loadOwnedBlock(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.TopicBlock{}, block.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete block"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Block deleted"})
}

func (h *Controller) ReorderTopicBlocks(c *gin.Context) {
	var rows []struct {
		ID    uint `json:"id"`
		Order int  `json:"order"`
	}
	if err := c.ShouldBindJSON(&rows); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid reorder payload"})
		return
	}
	for _, row := range rows {
		block, ok := h.loadOwnedBlockByID(c, row.ID)
		if !ok {
			return
		}
		if err := h.db.WithContext(c.Request.Context()).Model(&block).Update("order", row.Order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reorder blocks"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Blocks reordered"})
}

func (h *Controller) UpdateTopicBlockFile(c *gin.Context) {
	block, ok := h.loadOwnedBlock(c, "id")
	if !ok {
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
	if err := h.db.WithContext(c.Request.Context()).Model(&models.TopicBlock{}).Where("id = ?", block.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update block"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "File uploaded", Data: gin.H{"file_path": path, "file_name": originalName, "file_type": fileType}})
}

func (h *Controller) CreateAssignment(c *gin.Context) {
	topic, ok := h.loadOwnedTopic(c, "id")
	if !ok {
		return
	}
	var req models.Assignment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid assignment payload"})
		return
	}
	req.ID = 0
	req.TopicID = topic.ID
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create assignment"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Assignment created", Data: req})
}

func (h *Controller) UpdateAssignment(c *gin.Context) {
	assignment, ok := h.loadOwnedAssignment(c, "id")
	if !ok {
		return
	}
	var req models.Assignment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid assignment payload"})
		return
	}
	req.ID = assignment.ID
	req.TopicID = assignment.TopicID
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Assignment{}).Where("id = ?", assignment.ID).Select("title_en", "title_id", "description_en", "description_id").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save assignment"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Assignment saved", Data: req})
}

func (h *Controller) DeleteAssignment(c *gin.Context) {
	assignment, ok := h.loadOwnedAssignment(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.Assignment{}, assignment.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete assignment"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Assignment deleted"})
}

func (h *Controller) CreateQuiz(c *gin.Context) {
	module, ok := h.loadOwnedModule(c, "id")
	if !ok {
		return
	}
	var req models.Quiz
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz payload"})
		return
	}
	req.ID = 0
	req.ModuleID = module.ID
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

func (h *Controller) UpdateQuiz(c *gin.Context) {
	quiz, ok := h.loadOwnedQuiz(c, "id")
	if !ok {
		return
	}
	var req models.Quiz
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz payload"})
		return
	}
	req.ID = quiz.ID
	req.ModuleID = quiz.ModuleID
	if req.PassingScore == 0 {
		req.PassingScore = 70
	}
	if req.AttemptLimit == 0 {
		req.AttemptLimit = 3
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.Quiz{}).Where("id = ?", quiz.ID).Select("title_en", "title_id", "passing_score", "attempt_limit", "order").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save quiz"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz saved", Data: req})
}

func (h *Controller) DeleteQuiz(c *gin.Context) {
	quiz, ok := h.loadOwnedQuiz(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.Quiz{}, quiz.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete quiz"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz deleted"})
}

func (h *Controller) ReorderQuizzes(c *gin.Context) {
	var rows []struct {
		ID    uint `json:"id"`
		Order int  `json:"order"`
	}
	if err := c.ShouldBindJSON(&rows); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid reorder payload"})
		return
	}
	for _, row := range rows {
		quiz, ok := h.loadOwnedQuizByID(c, row.ID)
		if !ok {
			return
		}
		if err := h.db.WithContext(c.Request.Context()).Model(&quiz).Update("order", row.Order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to reorder quizzes"})
			return
		}
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quizzes reordered"})
}

func (h *Controller) CreateQuizQuestion(c *gin.Context) {
	quiz, ok := h.loadOwnedQuiz(c, "id")
	if !ok {
		return
	}
	var req models.QuizQuestion
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid question payload"})
		return
	}
	req.ID = 0
	req.QuizID = quiz.ID
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
	question, ok := h.loadOwnedQuizQuestion(c, "id")
	if !ok {
		return
	}
	var req models.QuizQuestion
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid question payload"})
		return
	}
	req.ID = question.ID
	req.QuizID = question.QuizID
	if strings.TrimSpace(req.Type) == "" {
		req.Type = question.Type
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.QuizQuestion{}).Where("id = ?", question.ID).Select("type", "question_en", "question_id", "order").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save question"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Question saved", Data: req})
}

func (h *Controller) DeleteQuizQuestion(c *gin.Context) {
	question, ok := h.loadOwnedQuizQuestion(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.QuizQuestion{}, question.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete question"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Question deleted"})
}

func (h *Controller) CreateQuizAnswer(c *gin.Context) {
	question, ok := h.loadOwnedQuizQuestion(c, "id")
	if !ok {
		return
	}
	var req models.QuizAnswer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid answer payload"})
		return
	}
	req.ID = 0
	req.QuestionID = question.ID
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create answer"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Answer created", Data: req})
}

func (h *Controller) UpdateQuizAnswer(c *gin.Context) {
	answer, ok := h.loadOwnedQuizAnswer(c, "id")
	if !ok {
		return
	}
	var req models.QuizAnswer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid answer payload"})
		return
	}
	req.ID = answer.ID
	req.QuestionID = answer.QuestionID
	if err := h.db.WithContext(c.Request.Context()).Model(&models.QuizAnswer{}).Where("id = ?", answer.ID).Select("answer_en", "answer_id", "is_correct", "order").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save answer"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Answer saved", Data: req})
}

func (h *Controller) DeleteQuizAnswer(c *gin.Context) {
	answer, ok := h.loadOwnedQuizAnswer(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.QuizAnswer{}, answer.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete answer"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Answer deleted"})
}

func (h *Controller) CreateCourseAddon(c *gin.Context) {
	course, ok := h.loadOwnedCourseByID(c)
	if !ok {
		return
	}
	var req models.CourseAddon
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid add-on payload"})
		return
	}
	req.ID = 0
	req.CourseID = course.ID
	if strings.TrimSpace(req.Type) == "" {
		req.Type = "resource"
	}
	req.IsActive = true
	if err := h.normalizeCourseAddon(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create add-on"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Add-on created", Data: req})
}

func (h *Controller) UpdateCourseAddon(c *gin.Context) {
	addon, ok := h.loadOwnedAddon(c, "id")
	if !ok {
		return
	}
	var req models.CourseAddon
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid add-on payload"})
		return
	}
	req.ID = addon.ID
	req.CourseID = addon.CourseID
	if strings.TrimSpace(req.Type) == "" {
		req.Type = addon.Type
	}
	if err := h.normalizeCourseAddon(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.CourseAddon{}).Where("id = ?", addon.ID).Select("product_category_id", "title_en", "title_id", "description_en", "description_id", "type", "file_path", "external_url", "order", "is_active").Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save add-on"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Add-on saved", Data: req})
}

func (h *Controller) DeleteCourseAddon(c *gin.Context) {
	addon, ok := h.loadOwnedAddon(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&models.CourseAddon{}, addon.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to delete add-on"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Add-on deleted"})
}

func (h *Controller) UpdateCourseAddonFile(c *gin.Context) {
	addon, ok := h.loadOwnedAddon(c, "id")
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "File is required"})
		return
	}
	path, originalName, err := h.uploadService.SaveCourseAddonFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&models.CourseAddon{}).Where("id = ?", addon.ID).Update("file_path", path).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save add-on file"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Add-on file uploaded", Data: gin.H{"file_path": path, "file_name": originalName, "file_type": file.Header.Get("Content-Type")}})
}

func (h *Controller) loadOwnedCourseByID(c *gin.Context) (models.Course, bool) {
	id, ok := paramID(c, "id")
	if !ok {
		return models.Course{}, false
	}
	user := c.MustGet("user").(models.User)
	var course models.Course
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND instructor_id = ?", id, user.ID).First(&course).Error; err != nil {
		status := http.StatusNotFound
		message := "Course not found"
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusInternalServerError
			message = "Failed to load course"
		}
		c.JSON(status, structs.Response{Success: false, Message: message})
		return models.Course{}, false
	}
	return course, true
}

func (h *Controller) loadOwnedModule(c *gin.Context, param string) (models.CourseModule, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.CourseModule{}, false
	}
	return h.loadOwnedModuleByID(c, id)
}

func (h *Controller) loadOwnedModuleByID(c *gin.Context, id uint) (models.CourseModule, bool) {
	user := c.MustGet("user").(models.User)
	var module models.CourseModule
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("course_modules.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&module).Error
	return ownedResult(c, module, err, "Module not found")
}

func (h *Controller) loadOwnedTopic(c *gin.Context, param string) (models.Topic, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.Topic{}, false
	}
	return h.loadOwnedTopicByID(c, id)
}

func (h *Controller) loadOwnedTopicByID(c *gin.Context, id uint) (models.Topic, bool) {
	user := c.MustGet("user").(models.User)
	var topic models.Topic
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN course_modules ON course_modules.id = topics.module_id").
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("topics.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&topic).Error
	return ownedResult(c, topic, err, "Topic not found")
}

func (h *Controller) loadOwnedBlock(c *gin.Context, param string) (models.TopicBlock, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.TopicBlock{}, false
	}
	return h.loadOwnedBlockByID(c, id)
}

func (h *Controller) loadOwnedBlockByID(c *gin.Context, id uint) (models.TopicBlock, bool) {
	user := c.MustGet("user").(models.User)
	var block models.TopicBlock
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN topics ON topics.id = topic_blocks.topic_id").
		Joins("JOIN course_modules ON course_modules.id = topics.module_id").
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("topic_blocks.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&block).Error
	return ownedResult(c, block, err, "Block not found")
}

func (h *Controller) loadOwnedAssignment(c *gin.Context, param string) (models.Assignment, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.Assignment{}, false
	}
	user := c.MustGet("user").(models.User)
	var assignment models.Assignment
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN topics ON topics.id = assignments.topic_id").
		Joins("JOIN course_modules ON course_modules.id = topics.module_id").
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("assignments.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&assignment).Error
	return ownedResult(c, assignment, err, "Assignment not found")
}

func (h *Controller) loadOwnedQuiz(c *gin.Context, param string) (models.Quiz, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.Quiz{}, false
	}
	return h.loadOwnedQuizByID(c, id)
}

func (h *Controller) loadOwnedQuizByID(c *gin.Context, id uint) (models.Quiz, bool) {
	user := c.MustGet("user").(models.User)
	var quiz models.Quiz
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN course_modules ON course_modules.id = quizzes.module_id").
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("quizzes.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&quiz).Error
	return ownedResult(c, quiz, err, "Quiz not found")
}

func (h *Controller) loadOwnedQuizQuestion(c *gin.Context, param string) (models.QuizQuestion, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.QuizQuestion{}, false
	}
	user := c.MustGet("user").(models.User)
	var question models.QuizQuestion
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN quizzes ON quizzes.id = quiz_questions.quiz_id").
		Joins("JOIN course_modules ON course_modules.id = quizzes.module_id").
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("quiz_questions.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&question).Error
	return ownedResult(c, question, err, "Question not found")
}

func (h *Controller) loadOwnedQuizAnswer(c *gin.Context, param string) (models.QuizAnswer, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.QuizAnswer{}, false
	}
	user := c.MustGet("user").(models.User)
	var answer models.QuizAnswer
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN quiz_questions ON quiz_questions.id = quiz_answers.question_id").
		Joins("JOIN quizzes ON quizzes.id = quiz_questions.quiz_id").
		Joins("JOIN course_modules ON course_modules.id = quizzes.module_id").
		Joins("JOIN courses ON courses.id = course_modules.course_id").
		Where("quiz_answers.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&answer).Error
	return ownedResult(c, answer, err, "Answer not found")
}

func (h *Controller) loadOwnedAddon(c *gin.Context, param string) (models.CourseAddon, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return models.CourseAddon{}, false
	}
	user := c.MustGet("user").(models.User)
	var addon models.CourseAddon
	err := h.db.WithContext(c.Request.Context()).
		Joins("JOIN courses ON courses.id = course_addons.course_id").
		Where("course_addons.id = ? AND courses.instructor_id = ?", id, user.ID).
		First(&addon).Error
	return ownedResult(c, addon, err, "Add-on not found")
}

func ownedResult[T any](c *gin.Context, value T, err error, missing string) (T, bool) {
	if err == nil {
		return value, true
	}
	status := http.StatusNotFound
	message := missing
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusInternalServerError
		message = "Failed to load resource"
	}
	c.JSON(status, structs.Response{Success: false, Message: message})
	var zero T
	return zero, false
}

func paramID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid id"})
		return 0, false
	}
	return uint(value), true
}

func instructorCourseStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "pending":
		return "pending"
	default:
		return "draft"
	}
}

func (h *Controller) normalizeCourseAddon(ctx context.Context, addon *models.CourseAddon) error {
	if addon.ProductCategoryID == 0 {
		if strings.TrimSpace(addon.Type) == "" {
			addon.Type = "resource"
		}
		return nil
	}
	var category models.ProductCategory
	if err := h.db.WithContext(ctx).First(&category, addon.ProductCategoryID).Error; err != nil {
		return fmt.Errorf("Product/service category is invalid")
	}
	addon.Type = category.Slug
	return nil
}

func (h *Controller) updateOwnedUpload(c *gin.Context, save func(*multipart.FileHeader) (string, error), model any, id uint, field string, message string) {
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
	if err := h.db.WithContext(c.Request.Context()).Model(model).Where("id = ?", id).Update(field, path).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update file"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: message, Data: gin.H{field: path}})
}

func (h *Controller) ListNotifications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	rows, unread, err := services.NotificationInbox(c.Request.Context(), h.db, user.ID, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to count notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications loaded", Data: gin.H{"items": rows, "unread_count": unread}})
}

func (h *Controller) MarkAllNotificationsRead(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if err := services.MarkAllNotificationsRead(c.Request.Context(), h.db, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update notifications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Notifications marked as read"})
}

func (h *Controller) ListCommunications(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	data, err := h.communicationService.AdminHub(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load communications"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Communications loaded", Data: data})
}

func (h *Controller) ListCommunicationMessages(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := communicationID(c)
	if !ok {
		return
	}
	messages, err := h.communicationService.Messages(c.Request.Context(), user, id)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Messages loaded", Data: messages})
}

func (h *Controller) ReplyCommunication(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := communicationID(c)
	if !ok {
		return
	}
	var input structs.CreateMessageRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Message is required"})
		return
	}
	if err := h.communicationService.Reply(c.Request.Context(), user, id, input.Message); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Reply sent"})
}

func (h *Controller) UpdateCommunicationStatus(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	id, ok := communicationID(c)
	if !ok {
		return
	}
	var input structs.UpdateConversationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Status must be open, resolved, or closed"})
		return
	}
	if err := h.communicationService.UpdateStatus(c.Request.Context(), user, id, input.Status); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Conversation status updated"})
}

func communicationID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid conversation ID"})
		return 0, false
	}
	return uint(value), true
}
