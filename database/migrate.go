package database

import (
	"fmt"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.AuthEmailOTP{},
		&models.PasswordResetToken{},
		&models.UserProfile{},
		&models.CourseCategory{},
		&models.Course{},
		&models.CourseAddon{},
		&models.CourseModule{},
		&models.Topic{},
		&models.TopicAttachment{},
		&models.TopicBlock{},
		&models.CourseEnrollment{},
		&models.TopicProgress{},
		&models.XPLedger{},
		&models.Achievement{},
		&models.UserAchievement{},
		&models.CourseConversation{},
		&models.CourseMessage{},
		&models.Assignment{},
		&models.AssignmentSubmission{},
		&models.Certificate{},
		&models.DripSchedule{},
		&models.Review{},
		&models.Quiz{},
		&models.QuizQuestion{},
		&models.QuizAnswer{},
		&models.QuizSubmission{},
		&models.QuizSubmissionAnswer{},
		&models.ProductCategory{},
		&models.Product{},
		&models.ProductFile{},
		&models.Order{},
		&models.OrderItem{},
		&models.Transaction{},
		&models.Coupon{},
		&models.CouponUsage{},
		&models.Invoice{},
		&models.ConsultationSlot{},
		&models.ConsultationBooking{},
		&models.TalentJob{},
		&models.TalentJobApplication{},
		&models.HiringInquiry{},
		&models.TalentPlusApplication{},
		&models.TalentReviewInvitation{},
		&models.PartnerApplication{},
		&models.Partner{},
		&models.ContactLead{},
		&models.LeadNote{},
		&models.NewsletterSubscriber{},
		&models.Setting{},
		&models.SEOMeta{},
		&models.Page{},
		&models.PageSection{},
		&models.FAQ{},
		&models.Testimonial{},
		&models.Banner{},
		&models.MediaFile{},
		&models.EmailDesign{},
		&models.EmailTemplate{},
		&models.MailerSender{},
		&models.EmailCampaign{},
		&models.AutomationWorkflow{},
		&models.AutomationRun{},
		&models.EmailSuppression{},
		&models.EmailEvent{},
		&models.Notification{},
		&models.Event{},
		&models.EventAttendance{},
		&models.JWTBlacklist{},
		&models.StoredObject{},
		&models.StorageMigrationJob{},
		&models.StorageMigrationItem{},
		&models.StorageBackup{},
		&models.DocumentTemplate{},
		&models.DocumentTemplateVersion{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	if err := cleanupRemovedCompanyAccountSchema(db); err != nil {
		return err
	}

	return nil
}

func cleanupRemovedCompanyAccountSchema(db *gorm.DB) error {
	var isCompanyColumnCount int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'users'
			AND COLUMN_NAME = 'is_company'
	`).Scan(&isCompanyColumnCount).Error; err != nil {
		return fmt.Errorf("check removed users.is_company column: %w", err)
	}

	if isCompanyColumnCount > 0 {
		if err := db.Exec("ALTER TABLE users DROP COLUMN is_company").Error; err != nil {
			return fmt.Errorf("drop removed users.is_company column: %w", err)
		}
	}

	if err := db.Exec("DROP TABLE IF EXISTS company_profiles").Error; err != nil {
		return fmt.Errorf("drop removed company_profiles table: %w", err)
	}

	return nil
}
