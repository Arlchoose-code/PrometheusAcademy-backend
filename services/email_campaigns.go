package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"academyprometheus/backend/models"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type MailerRecipient struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Language string `json:"language"`
}

type MailerFailure struct {
	Email string `json:"email"`
	Error string `json:"error"`
}

func SenderSettings(settings MailerSettings, name, email string) MailerSettings {
	if strings.TrimSpace(name) != "" {
		settings.FromName = strings.TrimSpace(name)
	}
	if strings.TrimSpace(email) != "" {
		settings.FromEmail = strings.TrimSpace(email)
	}
	return settings
}

func ResolveCampaignRecipients(ctx context.Context, db *gorm.DB, target string, userIDs []uint) ([]MailerRecipient, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not available")
	}
	recipients := []MailerRecipient{}
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "all_users":
		var users []models.User
		if err := db.WithContext(ctx).Where("email <> ''").Order("created_at desc").Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			recipients = append(recipients, MailerRecipient{ID: user.ID, Name: user.Name, Email: user.Email, Language: user.Language})
		}
	case "students":
		var users []models.User
		if err := db.WithContext(ctx).Where("is_student = ? AND email <> ''", true).Order("created_at desc").Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			recipients = append(recipients, MailerRecipient{ID: user.ID, Name: user.Name, Email: user.Email, Language: user.Language})
		}
	case "admins":
		var users []models.User
		if err := db.WithContext(ctx).Where("is_admin = ? AND email <> ''", true).Order("created_at desc").Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			recipients = append(recipients, MailerRecipient{ID: user.ID, Name: user.Name, Email: user.Email, Language: user.Language})
		}
	case "selected_users":
		if len(userIDs) == 0 {
			return nil, fmt.Errorf("Choose at least one user")
		}
		var users []models.User
		if err := db.WithContext(ctx).Where("id IN ? AND email <> ''", userIDs).Order("name asc").Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			recipients = append(recipients, MailerRecipient{ID: user.ID, Name: user.Name, Email: user.Email, Language: user.Language})
		}
	case "newsletter":
		var subscribers []models.NewsletterSubscriber
		if err := db.WithContext(ctx).Where("gdpr_consent = ? AND email <> ''", true).Order("subscribed_at desc").Find(&subscribers).Error; err != nil {
			return nil, err
		}
		for _, subscriber := range subscribers {
			recipients = append(recipients, MailerRecipient{Name: subscriber.FullName, Email: subscriber.Email, Language: "en"})
		}
	default:
		return nil, fmt.Errorf("Target must be all_users, students, admins, selected_users, or newsletter")
	}

	seen := map[string]bool{}
	deduped := make([]MailerRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		email := strings.ToLower(strings.TrimSpace(recipient.Email))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		recipient.Email = email
		recipient.Language = normalizeMailerLocale(recipient.Language)
		deduped = append(deduped, recipient)
	}
	return deduped, nil
}

func normalizeMailerLocale(locale string) string {
	if strings.EqualFold(strings.TrimSpace(locale), "id") {
		return "id"
	}
	return "en"
}

func RenderMailerRecipientVariables(value string, recipient MailerRecipient) string {
	return strings.NewReplacer(
		"{{name}}", recipient.Name,
		"{name}", recipient.Name,
		"{{email}}", recipient.Email,
		"{email}", recipient.Email,
	).Replace(value)
}

func RenderCampaignTemplateHTML(template models.EmailTemplate, locale string, subject string, content string, settings MailerSettings) string {
	source := template.BodyEn
	if normalizeMailerLocale(locale) == "id" && strings.TrimSpace(template.BodyID) != "" {
		source = template.BodyID
	}
	if strings.TrimSpace(source) == "" {
		source = defaultCampaignTemplateHTML()
	}
	siteName := strings.TrimSpace(settings.FromName)
	if siteName == "" {
		siteName = "Prometheus Academy"
	}
	footer := siteName + " - Europe x Asia learning bridge."
	rendered := replaceMailerLayoutTokens(source, map[string]string{
		"brand_name": siteName,
		"content":    content,
		"footer":     footer,
		"site_name":  siteName,
		"subject":    subject,
	})
	if containsMailerToken(source, "content") {
		return rendered
	}
	return rendered + `<div style="max-width:620px;margin:0 auto;padding:24px 28px;font-family:Arial,Helvetica,sans-serif;font-size:15px;line-height:1.7;color:#343A40;">` + content + `</div>`
}

func replaceMailerLayoutTokens(value string, replacements map[string]string) string {
	for token, replacement := range replacements {
		value = strings.NewReplacer("{{"+token+"}}", replacement, "{"+token+"}", replacement).Replace(value)
	}
	return value
}

func containsMailerToken(value string, token string) bool {
	return strings.Contains(value, "{"+token+"}") || strings.Contains(value, "{{"+token+"}}")
}

func defaultCampaignTemplateHTML() string {
	return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#F8F9FA;font-family:Arial,Helvetica,sans-serif;color:#212529;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#F8F9FA;padding:24px 12px;">
      <tr>
        <td align="center">
          <table role="presentation" width="620" cellspacing="0" cellpadding="0" style="width:620px;max-width:100%;background:#FFFFFF;border-radius:14px;overflow:hidden;border:1px solid #E9ECEF;">
            <tr><td style="height:4px;background:#C9A84C;font-size:0;line-height:0;">&nbsp;</td></tr>
            <tr><td style="padding:24px 28px;border-bottom:1px solid #E9ECEF;"><strong style="color:#0D1B2E;font-size:16px;">{site_name}</strong></td></tr>
            <tr><td style="padding:28px 28px 8px;"><h1 style="margin:0;color:#212529;font-size:26px;line-height:1.3;font-weight:800;">{subject}</h1></td></tr>
            <tr><td style="padding:12px 28px 32px;font-size:15px;line-height:1.7;color:#343A40;">{content}</td></tr>
            <tr><td style="padding:20px 28px;background:#F8F9FA;border-top:1px solid #E9ECEF;color:#6C757D;font-size:12px;line-height:1.6;"><strong style="color:#0D1B2E;">{site_name}</strong><br/>{footer}</td></tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`
}

func StartEmailCampaignWorker(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := ProcessNextQueuedEmailCampaign(ctx, db); err != nil {
				log.Warn().Err(err).Msg("email campaign worker failed")
			}
		case <-ctx.Done():
			return
		}
	}
}

func ProcessNextQueuedEmailCampaign(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var campaign models.EmailCampaign
	if err := db.WithContext(ctx).Where("status = ?", "queued").Order("queued_at asc, id asc").First(&campaign).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	now := time.Now()
	if err := db.WithContext(ctx).Model(&campaign).Updates(map[string]any{
		"status":     "sending",
		"started_at": &now,
	}).Error; err != nil {
		return err
	}

	var userIDs []uint
	if strings.TrimSpace(campaign.UserIDsJSON) != "" {
		_ = json.Unmarshal([]byte(campaign.UserIDsJSON), &userIDs)
	}
	recipients, err := ResolveCampaignRecipients(ctx, db, campaign.Target, userIDs)
	if err != nil {
		return finishCampaign(ctx, db, &campaign, "failed", 0, []MailerFailure{{Email: "-", Error: err.Error()}}, 0)
	}
	if len(recipients) == 0 {
		return finishCampaign(ctx, db, &campaign, "failed", 0, []MailerFailure{{Email: "-", Error: "No recipients found"}}, 0)
	}

	settings, err := LoadMailerSettings(ctx, db)
	if err != nil {
		return finishCampaign(ctx, db, &campaign, "failed", len(recipients), []MailerFailure{{Email: "-", Error: err.Error()}}, 0)
	}
	settings = SenderSettings(settings, campaign.SenderName, campaign.SenderEmail)
	template := models.EmailTemplate{}
	hasTemplate := false
	if strings.TrimSpace(campaign.TemplateKey) != "" {
		if err := db.WithContext(ctx).Where("`key` = ?", strings.TrimSpace(campaign.TemplateKey)).First(&template).Error; err != nil {
			return finishCampaign(ctx, db, &campaign, "failed", len(recipients), []MailerFailure{{Email: "-", Error: "Email template not found: " + campaign.TemplateKey}}, 0)
		}
		hasTemplate = true
		settings = SenderSettings(settings, template.SenderName, template.SenderEmail)
	}
	rateLimit := campaign.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = 30
	}
	delay := time.Minute / time.Duration(rateLimit)

	sent := 0
	failures := []MailerFailure{}
	for index, recipient := range recipients {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		subject := localizedCampaignSubject(campaign, recipient.Language)
		body := localizedCampaignHTML(campaign, recipient.Language)
		text := localizedCampaignText(campaign, recipient.Language)
		html := body
		if hasTemplate {
			html = RenderCampaignTemplateHTML(template, recipient.Language, subject, body, settings)
		}
		_, err := SendMailerEmail(ctx, settings, MailMessage{
			ToEmail: recipient.Email,
			ToName:  recipient.Name,
			Subject: RenderMailerRecipientVariables(subject, recipient),
			HTML:    RenderMailerRecipientVariables(html, recipient),
			Text:    RenderMailerRecipientVariables(text, recipient),
		})
		if err != nil {
			failures = append(failures, MailerFailure{Email: recipient.Email, Error: err.Error()})
		} else {
			sent++
		}
		_ = db.WithContext(ctx).Model(&campaign).Updates(map[string]any{
			"recipient_count": len(recipients),
			"sent_count":      sent,
			"failed_count":    len(failures),
		}).Error
		if index < len(recipients)-1 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
	}

	status := "sent"
	if sent == 0 {
		status = "failed"
	} else if len(failures) > 0 {
		status = "partial"
	}
	return finishCampaign(ctx, db, &campaign, status, len(recipients), failures, sent)
}

func localizedCampaignSubject(campaign models.EmailCampaign, locale string) string {
	if normalizeMailerLocale(locale) == "id" && strings.TrimSpace(campaign.SubjectID) != "" {
		return campaign.SubjectID
	}
	if strings.TrimSpace(campaign.SubjectEn) != "" {
		return campaign.SubjectEn
	}
	if strings.TrimSpace(campaign.SubjectID) != "" {
		return campaign.SubjectID
	}
	return campaign.Subject
}

func localizedCampaignHTML(campaign models.EmailCampaign, locale string) string {
	if normalizeMailerLocale(locale) == "id" && strings.TrimSpace(campaign.HTMLID) != "" {
		return campaign.HTMLID
	}
	if strings.TrimSpace(campaign.HTMLEn) != "" {
		return campaign.HTMLEn
	}
	if strings.TrimSpace(campaign.HTMLID) != "" {
		return campaign.HTMLID
	}
	return campaign.HTML
}

func localizedCampaignText(campaign models.EmailCampaign, locale string) string {
	if normalizeMailerLocale(locale) == "id" && strings.TrimSpace(campaign.TextID) != "" {
		return campaign.TextID
	}
	if strings.TrimSpace(campaign.TextEn) != "" {
		return campaign.TextEn
	}
	if strings.TrimSpace(campaign.TextID) != "" {
		return campaign.TextID
	}
	return campaign.Text
}

func finishCampaign(ctx context.Context, db *gorm.DB, campaign *models.EmailCampaign, status string, recipientCount int, failures []MailerFailure, sent int) error {
	rawFailures, _ := json.Marshal(failures)
	now := time.Now()
	return db.WithContext(ctx).Model(campaign).Updates(map[string]any{
		"status":          status,
		"recipient_count": recipientCount,
		"sent_count":      sent,
		"failed_count":    len(failures),
		"failed_json":     string(rawFailures),
		"finished_at":     &now,
	}).Error
}
