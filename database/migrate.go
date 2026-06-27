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
		&models.TalentTrustPhoto{},
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

	if err := cleanupRemovedTalentSubscriptionSchema(db); err != nil {
		return err
	}
	if err := backfillTalentAccountLinks(db); err != nil {
		return err
	}

	return nil
}

func backfillTalentAccountLinks(db *gorm.DB) error {
	queries := []string{
		`UPDATE users SET is_student = FALSE, is_company = FALSE WHERE is_admin = FALSE AND is_instructor = TRUE`,
		`UPDATE users SET is_student = FALSE WHERE is_admin = FALSE AND is_company = TRUE`,
	}
	for _, query := range queries {
		if err := db.Exec(query).Error; err != nil {
			return fmt.Errorf("backfill talent account links: %w", err)
		}
	}
	return nil
}

func cleanupRemovedTalentSubscriptionSchema(db *gorm.DB) error {
	if err := db.Exec("DROP TABLE IF EXISTS talent_subscriptions").Error; err != nil {
		return fmt.Errorf("drop removed talent_subscriptions table: %w", err)
	}
	return nil
}
