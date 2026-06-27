package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

const (
	EmailTemplateRegister          = "email_template_register"
	EmailTemplateEmailVerification = "email_template_email_verification"
	EmailTemplateLogin             = "email_template_login"
	// #nosec G101 - this is an email template key, not a credential.
	EmailTemplatePasswordReset        = "email_template_password_reset"
	EmailTemplateInvoice              = "email_template_invoice"
	EmailTemplatePaymentSuccess       = "email_template_payment_success"
	EmailTemplateDepositConfirmation  = "email_template_deposit_confirmation"
	EmailTemplateCertificate          = "email_template_certificate"
	EmailTemplateBookingConfirmation  = "email_template_booking_confirmation"
	EmailTemplateCampaignNewsletter   = "email_template_campaign_newsletter"
	EmailTemplateCampaignAnnouncement = "email_template_campaign_announcement"
	EmailTemplateCampaignDefault      = "email_template_campaign_default_wrapper"
	EmailTemplateTalentReviewInvite   = "email_template_talent_review_invitation"
	EmailTemplateTalentApplication    = "email_template_talent_application_received"
	EmailTemplateTalentStatusUpdate   = "email_template_talent_status_update"
	EmailTemplateHiringInquiry        = "email_template_hiring_inquiry_received"
	EmailTemplateCompanyPortalInvite  = "email_template_company_portal_invite"
	EmailTemplatePartnerApplication   = "email_template_partner_application_received"
)

type TransactionalTemplateMapping struct {
	Key         string
	Label       string
	DefaultKey  string
	Description string
}

func TransactionalTemplateMappings() []TransactionalTemplateMapping {
	return []TransactionalTemplateMapping{
		{Key: EmailTemplateRegister, Label: "Register / welcome", DefaultKey: "welcome", Description: "Sent after successful registration."},
		{Key: EmailTemplateEmailVerification, Label: "Email verification OTP", DefaultKey: "email_verification", Description: "Sent before completing registration."},
		{Key: EmailTemplateLogin, Label: "Login OTP", DefaultKey: "otp_login", Description: "Sent when login needs OTP verification."},
		{Key: EmailTemplatePasswordReset, Label: "Password reset", DefaultKey: "password_reset", Description: "Sent when a user requests password reset."},
		{Key: EmailTemplateInvoice, Label: "Invoice", DefaultKey: "invoice", Description: "Sent when an invoice is generated after payment."},
		{Key: EmailTemplatePaymentSuccess, Label: "Payment success", DefaultKey: "payment_success", Description: "Sent when an order payment becomes successful."},
		{Key: EmailTemplateDepositConfirmation, Label: "Deposit confirmation", DefaultKey: "deposit_confirmation", Description: "Reserved for deposit confirmation flow."},
		{Key: EmailTemplateCertificate, Label: "Certificate ready", DefaultKey: "certificate", Description: "Sent when a course certificate is issued."},
		{Key: EmailTemplateBookingConfirmation, Label: "Booking confirmation", DefaultKey: "booking_confirmation", Description: "Sent after consultation booking is confirmed."},
		{Key: EmailTemplateCampaignNewsletter, Label: "Campaign newsletter", DefaultKey: "campaign_newsletter", Description: "Default newsletter campaign template."},
		{Key: EmailTemplateCampaignAnnouncement, Label: "Campaign announcement", DefaultKey: "campaign_announcement", Description: "Default announcement campaign template."},
		{Key: EmailTemplateCampaignDefault, Label: "Default simple campaign wrapper", DefaultKey: "campaign_simple", Description: "Editable wrapper used when Campaign Composer uses Default simple email."},
		{Key: EmailTemplateTalentReviewInvite, Label: "Talent review invitation", DefaultKey: "talent_review_invitation", Description: "Sent when an eligible Talent Bridge applicant is invited to leave a review."},
		{Key: EmailTemplateTalentApplication, Label: "Talent application received", DefaultKey: "talent_application_received", Description: "Sent after a Talent Bridge job or Talent Bridge+ application is submitted."},
		{Key: EmailTemplateTalentStatusUpdate, Label: "Talent application status update", DefaultKey: "talent_status_update", Description: "Sent when a Talent Bridge application status changes."},
		{Key: EmailTemplateHiringInquiry, Label: "Hiring inquiry received", DefaultKey: "hiring_inquiry_received", Description: "Sent after an employer submits an I'm Hiring inquiry."},
		{Key: EmailTemplateCompanyPortalInvite, Label: "Company portal invite", DefaultKey: "company_portal_invite", Description: "Sent after an employer submits I'm Hiring so they can set a password and open the company dashboard."},
		{Key: EmailTemplatePartnerApplication, Label: "Partner application received", DefaultKey: "partner_application_received", Description: "Sent after a university partner application is submitted."},
	}
}

func TransactionalTemplateDefaults() map[string]string {
	defaults := map[string]string{}
	for _, item := range TransactionalTemplateMappings() {
		defaults[item.Key] = item.DefaultKey
	}
	return defaults
}

func SendTransactionalTemplateEmail(ctx context.Context, db *gorm.DB, settingKey string, fallbackTemplateKey string, user models.User, variables map[string]string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	settings, err := LoadMailerSettings(ctx, db)
	if err != nil {
		return err
	}
	templateKey := strings.TrimSpace(fallbackTemplateKey)
	var row models.Setting
	if err := db.WithContext(ctx).Where("`key` = ?", settingKey).First(&row).Error; err == nil && strings.TrimSpace(row.Value) != "" {
		templateKey = strings.TrimSpace(row.Value)
	}
	if templateKey == "" {
		return fmt.Errorf("email template is not configured")
	}
	var template models.EmailTemplate
	if err := db.WithContext(ctx).Where("`key` = ?", templateKey).First(&template).Error; err != nil {
		return fmt.Errorf("email template %s not found", templateKey)
	}
	settings = SenderSettings(settings, template.SenderName, template.SenderEmail)
	locale := normalizeMailerLocale(user.Language)
	subject := template.SubjectEn
	html := template.BodyEn
	if locale == "id" {
		subject = fallback(template.SubjectID, subject)
		html = fallback(template.BodyID, html)
	}
	replacements := map[string]string{
		"name":      user.Name,
		"email":     user.Email,
		"site_name": settings.FromName,
		"subject":   subject,
	}
	for key, value := range variables {
		replacements[key] = value
	}
	subject = replaceMailerLayoutTokens(subject, replacements)
	html = replaceMailerLayoutTokens(html, replacements)
	messageID, err := SendMailerEmail(ctx, settings, MailMessage{
		ToEmail: user.Email,
		ToName:  user.Name,
		Subject: subject,
		HTML:    html,
		Text:    strings.TrimSpace(stripHTMLForEmail(html)),
		Tags:    []string{"prometheus-platform-user"},
	})
	if err != nil {
		return err
	}
	log.Printf("transactional email sent: setting=%s template=%s user_id=%d email=%s message_id=%s", settingKey, templateKey, user.ID, user.Email, messageID)
	return nil
}

func stripHTMLForEmail(value string) string {
	var builder strings.Builder
	inTag := false
	for _, char := range value {
		switch char {
		case '<':
			inTag = true
		case '>':
			inTag = false
			builder.WriteRune(' ')
		default:
			if !inTag {
				builder.WriteRune(char)
			}
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}
