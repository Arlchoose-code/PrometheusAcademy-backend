package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	publiccontroller "academyprometheus/backend/controllers/public"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Controller) CreateCourseEnrollment(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	_ = services.CancelExpiredPendingOrders(c.Request.Context(), h.db)
	var req structs.PurchaseRequest
	_ = c.ShouldBindJSON(&req)
	var course models.Course
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status = ?", c.Param("slug"), "open").First(&course).Error; err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Course is not open"})
		return
	}
	var existing models.CourseEnrollment
	if err := h.db.WithContext(c.Request.Context()).Where(models.CourseEnrollment{UserID: user.ID, CourseID: course.ID}).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Already enrolled", Data: gin.H{"enrolled": true, "status": "success"}})
		return
	}
	if !course.IsFree && course.Price > 0 {
		amount, coupon, err := services.DiscountedAmount(c.Request.Context(), h.db, course.Price, req.CouponCode, "course")
		if err != nil {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
			return
		}
		if existingOrder, ok := services.PendingOrderForItem(c.Request.Context(), h.db, user.ID, "course", course.ID); ok && coupon == nil {
			c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Pending payment loaded", Data: services.OrderPaymentResponse(existingOrder, false)})
			return
		}
		orderID := fmt.Sprintf("PROM-COURSE-%d-%d", user.ID, time.Now().UnixNano())
		order := models.Order{UserID: user.ID, TotalAmount: amount, Status: "pending", MidtransOrderID: orderID}
		if coupon != nil {
			order.AppliedCouponID = coupon.ID
		}
		if amount == 0 || h.cfg.MidtransServerKey == "" {
			order.Status = "success"
		}
		if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.OrderItem{OrderID: order.ID, ItemType: "course", ItemID: course.ID, Price: amount}).Error; err != nil {
				return err
			}
			if order.Status == "success" {
				if err := services.FulfillSuccessfulOrder(c.Request.Context(), tx, order); err != nil {
					return err
				}
				_, err := services.EnsureInvoice(c.Request.Context(), tx, h.cfg, order)
				return err
			}
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to create payment order"})
			return
		}
		data := services.OrderPaymentResponse(order, order.Status == "success")
		c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Payment order created", Data: data})
		return
	}
	enrollment := models.CourseEnrollment{UserID: user.ID, CourseID: course.ID, EnrolledAt: time.Now()}
	if err := h.db.WithContext(c.Request.Context()).Where(models.CourseEnrollment{UserID: user.ID, CourseID: course.ID}).Attrs(enrollment).FirstOrCreate(&enrollment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to enroll"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Enrolled", Data: enrollment})
}

func (h *Controller) GetCoursePlayer(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var course models.Course
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status <> ?", c.Param("slug"), "draft").First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return
	}
	var enrollment models.CourseEnrollment
	if err := h.db.WithContext(c.Request.Context()).Where(models.CourseEnrollment{UserID: user.ID, CourseID: course.ID}).First(&enrollment).Error; err != nil {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Enroll first"})
		return
	}
	reviewsPage := positiveInt(c.Query("reviews_page"), 1)
	detail, err := publiccontroller.BuildCourseDetail(c.Request.Context(), h.db, course, user.ID, reviewsPage, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to load player"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Player loaded", Data: detail})
}

func (h *Controller) CreateCourseReview(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var course models.Course
	if err := h.db.WithContext(c.Request.Context()).Where("slug = ? AND status <> ?", c.Param("slug"), "draft").First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return
	}

	var enrollment models.CourseEnrollment
	if err := h.db.WithContext(c.Request.Context()).Where(models.CourseEnrollment{UserID: user.ID, CourseID: course.ID}).First(&enrollment).Error; err != nil || (!user.IsAdmin && enrollment.CompletedAt == nil) {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Complete this course before leaving a review"})
		return
	}

	review, err := saveReview(c.Request.Context(), h.db, user.ID, course.ID, "course", c)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Review saved", Data: review})
}

func (h *Controller) CompleteTopic(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	topicID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || topicID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid topic"})
		return
	}
	var topic models.Topic
	if err := h.db.WithContext(c.Request.Context()).First(&topic, uint(topicID)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Topic not found"})
		return
	}
	course, err := services.CourseForModuleItem(c.Request.Context(), h.db, topic.ModuleID)
	if err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return
	}
	state, err := services.LearningStateForCourse(c.Request.Context(), h.db, course, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check access"})
		return
	}
	if !state.TopicUnlocked[uint(topicID)] {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Topic is locked"})
		return
	}
	now := time.Now()
	progress := models.TopicProgress{UserID: user.ID, TopicID: uint(topicID), CompletedAt: &now, VideoWatched: true}
	if err := h.db.WithContext(c.Request.Context()).Where(models.TopicProgress{UserID: user.ID, TopicID: uint(topicID)}).Assign(progress).FirstOrCreate(&progress).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to save progress"})
		return
	}
	certificate, completed, err := services.SyncCourseCompletion(c.Request.Context(), h.db, h.cfg, course, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to finalize progress"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Topic completed", Data: gin.H{"progress": progress, "course_completed": completed, "certificate": certificate}})
}

func (h *Controller) SubmitQuiz(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	quizID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || quizID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz"})
		return
	}
	var req struct {
		Answers []services.QuizAnswerInput `json:"answers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid quiz payload"})
		return
	}
	var quiz models.Quiz
	if err := h.db.WithContext(c.Request.Context()).First(&quiz, uint(quizID)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Quiz not found"})
		return
	}
	course, err := services.CourseForModuleItem(c.Request.Context(), h.db, quiz.ModuleID)
	if err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return
	}
	state, err := services.LearningStateForCourse(c.Request.Context(), h.db, course, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check access"})
		return
	}
	if !state.QuizUnlocked[uint(quizID)] {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Quiz is locked"})
		return
	}
	result, err := services.SubmitQuiz(c.Request.Context(), h.db, uint(quizID), user.ID, req.Answers)
	if err != nil {
		if errors.Is(err, services.ErrQuizAttemptLimitReached) {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Quiz attempt limit reached"})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit quiz"})
		return
	}
	certificate, completed, err := services.SyncCourseCompletion(c.Request.Context(), h.db, h.cfg, course, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to finalize quiz"})
		return
	}
	result["course_completed"] = completed
	result["certificate"] = certificate
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Quiz submitted", Data: result})
}

func (h *Controller) SubmitAssignment(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	assignmentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || assignmentID == 0 {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid assignment"})
		return
	}
	var req struct {
		FilePath string `json:"file_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid assignment payload"})
		return
	}
	var assignment models.Assignment
	if err := h.db.WithContext(c.Request.Context()).First(&assignment, uint(assignmentID)).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Assignment not found"})
		return
	}
	var topic models.Topic
	if err := h.db.WithContext(c.Request.Context()).First(&topic, assignment.TopicID).Error; err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Topic not found"})
		return
	}
	course, err := services.CourseForModuleItem(c.Request.Context(), h.db, topic.ModuleID)
	if err != nil {
		c.JSON(http.StatusNotFound, structs.Response{Success: false, Message: "Course not found"})
		return
	}
	state, err := services.LearningStateForCourse(c.Request.Context(), h.db, course, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to check access"})
		return
	}
	if !state.TopicUnlocked[topic.ID] {
		c.JSON(http.StatusForbidden, structs.Response{Success: false, Message: "Assignment is locked"})
		return
	}
	now := time.Now()
	submission := models.AssignmentSubmission{AssignmentID: assignment.ID, UserID: user.ID, FilePath: strings.TrimSpace(req.FilePath), SubmittedAt: &now}
	if err := h.db.WithContext(c.Request.Context()).Where(models.AssignmentSubmission{AssignmentID: assignment.ID, UserID: user.ID}).Assign(submission).FirstOrCreate(&submission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to submit assignment"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Assignment submitted", Data: submission})
}

func saveReview(ctx context.Context, db *gorm.DB, userID uint, reviewableID uint, reviewableType string, c *gin.Context) (models.Review, error) {
	var req structs.ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return models.Review{}, errors.New("Invalid review payload")
	}
	comment := strings.TrimSpace(req.Comment)
	if req.Rating < 1 || req.Rating > 5 {
		return models.Review{}, errors.New("Rating must be between 1 and 5")
	}
	if comment == "" {
		return models.Review{}, errors.New("Review comment is required")
	}
	review := models.Review{
		UserID:         userID,
		ReviewableID:   reviewableID,
		ReviewableType: reviewableType,
		Rating:         req.Rating,
		Comment:        comment,
	}
	err := db.WithContext(ctx).
		Where(models.Review{UserID: userID, ReviewableID: reviewableID, ReviewableType: reviewableType}).
		Assign(review).
		FirstOrCreate(&review).Error
	return review, err
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
