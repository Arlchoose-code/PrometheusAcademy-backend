package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/structs"

	"gorm.io/gorm"
)

var (
	ErrConversationForbidden  = errors.New("conversation access forbidden")
	ErrConversationNotFound   = errors.New("conversation not found")
	ErrCourseEnrollmentNeeded = errors.New("active course enrollment required")
)

type CommunicationService struct{ db *gorm.DB }

func NewCommunicationService(db *gorm.DB) *CommunicationService {
	return &CommunicationService{db: db}
}

func (s *CommunicationService) Hub(ctx context.Context, user models.User) (structs.CommunicationHubResponse, error) {
	conversations, err := s.listConversations(ctx, user)
	if err != nil {
		return structs.CommunicationHubResponse{}, err
	}

	var courses []structs.ConversationCourseResponse
	err = s.db.WithContext(ctx).Table("courses").
		Select("courses.id, courses.title_en, courses.title_id").
		Joins("JOIN course_enrollments ON course_enrollments.course_id = courses.id").
		Where("course_enrollments.user_id = ?", user.ID).
		Order("courses.title_en ASC").Scan(&courses).Error
	if err != nil {
		return structs.CommunicationHubResponse{}, err
	}

	var unread int64
	for _, conversation := range conversations {
		if conversation.CurrentUserIsStaff {
			unread += int64(conversation.StaffUnread)
		} else {
			unread += int64(conversation.StudentUnread)
		}
	}
	return structs.CommunicationHubResponse{Conversations: conversations, Courses: courses, UnreadCount: unread}, nil
}

func (s *CommunicationService) AdminHub(ctx context.Context, user models.User) (structs.CommunicationHubResponse, error) {
	conversations, err := s.listConversations(ctx, user)
	if err != nil {
		return structs.CommunicationHubResponse{}, err
	}
	var unread int64
	for _, conversation := range conversations {
		unread += int64(conversation.StaffUnread)
	}
	return structs.CommunicationHubResponse{Conversations: conversations, Courses: []structs.ConversationCourseResponse{}, UnreadCount: unread}, nil
}

func (s *CommunicationService) listConversations(ctx context.Context, user models.User) ([]structs.ConversationResponse, error) {
	query := s.db.WithContext(ctx).Table("course_conversations AS conversations").
		Select(`conversations.id, conversations.course_id, courses.title_en AS course_title_en,
			courses.title_id AS course_title_id, conversations.user_id AS student_id,
			users.name AS student_name, conversations.subject, conversations.status,
			conversations.student_unread, conversations.staff_unread, conversations.last_message_at,
			CASE WHEN conversations.user_id <> ? THEN TRUE ELSE FALSE END AS current_user_is_staff`, user.ID).
		Joins("JOIN courses ON courses.id = conversations.course_id").
		Joins("JOIN users ON users.id = conversations.user_id")
	if !user.IsAdmin {
		query = query.Where("conversations.user_id = ? OR courses.instructor_id = ?", user.ID, user.ID)
	}
	var rows []structs.ConversationResponse
	err := query.Order("conversations.last_message_at DESC").Scan(&rows).Error
	if rows == nil {
		rows = make([]structs.ConversationResponse, 0)
	}
	return rows, err
}

func (s *CommunicationService) CreateConversation(ctx context.Context, user models.User, input structs.CreateConversationRequest) (models.CourseConversation, error) {
	var enrollment models.CourseEnrollment
	if err := s.db.WithContext(ctx).Where("user_id = ? AND course_id = ?", user.ID, input.CourseID).First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.CourseConversation{}, ErrCourseEnrollmentNeeded
		}
		return models.CourseConversation{}, err
	}

	now := time.Now().UTC()
	conversation := models.CourseConversation{CourseID: input.CourseID, UserID: user.ID, Subject: strings.TrimSpace(input.Subject), Status: "open", StaffUnread: 1, LastMessageAt: now}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&conversation).Error; err != nil {
			return err
		}
		message := models.CourseMessage{ConversationID: conversation.ID, UserID: user.ID, Body: strings.TrimSpace(input.Message)}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		if err := AwardXP(ctx, tx, user.ID, XPEventDiscussionParticipated, "course_message", message.ID, XPDiscussion, "Participated in a course discussion", "Ikut diskusi course"); err != nil {
			return err
		}
		return s.notifyStaff(tx, conversation, user.Name)
	})
	return conversation, err
}

func (s *CommunicationService) Messages(ctx context.Context, user models.User, conversationID uint) ([]structs.ConversationMessageResponse, error) {
	conversation, isStaff, err := s.authorizedConversation(ctx, user, conversationID)
	if err != nil {
		return nil, err
	}
	field := "student_unread"
	if isStaff {
		field = "staff_unread"
	}
	if err := s.db.WithContext(ctx).Model(&conversation).Update(field, 0).Error; err != nil {
		return nil, err
	}

	var rows []structs.ConversationMessageResponse
	err = s.db.WithContext(ctx).Table("course_messages AS messages").
		Select("messages.id, messages.user_id, users.name AS user_name, messages.body, messages.is_instructor, messages.created_at").
		Joins("JOIN users ON users.id = messages.user_id").
		Where("messages.conversation_id = ?", conversationID).
		Order("messages.created_at ASC").Scan(&rows).Error
	if rows == nil {
		rows = make([]structs.ConversationMessageResponse, 0)
	}
	return rows, err
}

func (s *CommunicationService) Reply(ctx context.Context, user models.User, conversationID uint, body string) error {
	conversation, isStaff, err := s.authorizedConversation(ctx, user, conversationID)
	if err != nil {
		return err
	}
	if conversation.Status == "closed" {
		return errors.New("closed conversation cannot receive replies")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		message := models.CourseMessage{ConversationID: conversation.ID, UserID: user.ID, Body: strings.TrimSpace(body), IsInstructor: isStaff}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		if !isStaff {
			if err := AwardXP(ctx, tx, user.ID, XPEventDiscussionParticipated, "course_message", message.ID, XPDiscussion, "Participated in a course discussion", "Ikut diskusi course"); err != nil {
				return err
			}
		}
		updates := map[string]any{"last_message_at": now}
		if isStaff {
			updates["student_unread"] = gorm.Expr("student_unread + 1")
		} else {
			updates["staff_unread"] = gorm.Expr("staff_unread + 1")
		}
		if err := tx.Model(&conversation).Updates(updates).Error; err != nil {
			return err
		}
		if isStaff {
			return tx.Create(&models.Notification{UserID: conversation.UserID, TitleEn: "New course reply", TitleID: "Balasan course baru", MessageEn: user.Name + " replied to your course conversation.", MessageID: user.Name + " membalas percakapan course kamu.", Type: "course_communication", Link: "/dashboard/communications?thread=" + strconv.FormatUint(uint64(conversation.ID), 10)}).Error
		}
		return s.notifyStaff(tx, conversation, user.Name)
	})
}

func (s *CommunicationService) UpdateStatus(ctx context.Context, user models.User, conversationID uint, status string) error {
	conversation, isStaff, err := s.authorizedConversation(ctx, user, conversationID)
	if err != nil {
		return err
	}
	if !isStaff {
		return ErrConversationForbidden
	}
	return s.db.WithContext(ctx).Model(&conversation).Update("status", status).Error
}

func (s *CommunicationService) authorizedConversation(ctx context.Context, user models.User, id uint) (models.CourseConversation, bool, error) {
	var conversation models.CourseConversation
	if err := s.db.WithContext(ctx).First(&conversation, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation, false, ErrConversationNotFound
		}
		return conversation, false, err
	}
	if conversation.UserID == user.ID {
		return conversation, false, nil
	}
	if user.IsAdmin {
		return conversation, true, nil
	}
	var course models.Course
	if err := s.db.WithContext(ctx).Select("id", "instructor_id").First(&course, conversation.CourseID).Error; err != nil {
		return conversation, false, err
	}
	if course.InstructorID != user.ID {
		return conversation, false, ErrConversationForbidden
	}
	return conversation, true, nil
}

func (s *CommunicationService) notifyStaff(tx *gorm.DB, conversation models.CourseConversation, studentName string) error {
	var course models.Course
	if err := tx.Select("id", "instructor_id").First(&course, conversation.CourseID).Error; err != nil {
		return err
	}
	var recipients []models.User
	if course.InstructorID > 0 {
		if err := tx.Where("id = ?", course.InstructorID).Find(&recipients).Error; err != nil {
			return err
		}
	} else if err := tx.Where("is_admin = ?", true).Find(&recipients).Error; err != nil {
		return err
	}
	for _, recipient := range recipients {
		if recipient.ID == conversation.UserID {
			continue
		}
		link := "/instructor/communications?thread=" + strconv.FormatUint(uint64(conversation.ID), 10)
		if recipient.IsAdmin && !recipient.IsInstructor {
			link = "/admin/communications?thread=" + strconv.FormatUint(uint64(conversation.ID), 10)
		}
		notification := models.Notification{UserID: recipient.ID, TitleEn: "Course question", TitleID: "Pertanyaan course", MessageEn: studentName + " sent a course question.", MessageID: studentName + " mengirim pertanyaan course.", Type: "course_communication", Link: link}
		if err := tx.Create(&notification).Error; err != nil {
			return err
		}
	}
	return nil
}
