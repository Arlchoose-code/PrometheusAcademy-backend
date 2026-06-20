package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var defaultAutomationWorkflows = []models.AutomationWorkflow{
	{Key: "welcome-no-purchase-h1", Name: "Registered, no purchase — H+1", Category: "follow_up", TriggerType: "registered_no_purchase", DelayMinutes: 1440, SubjectEn: "Ready to start learning?", SubjectID: "Siap mulai belajar?", BodyEn: "<p>Hi {name}, explore the courses and services that can help you take the next step.</p>", BodyID: "<p>Halo {name}, lihat course dan layanan yang bisa membantu langkah berikutnya.</p>", IsEnabled: true},
	{Key: "welcome-no-purchase-h3", Name: "Registered, no purchase — H+3", Category: "follow_up", TriggerType: "registered_no_purchase", DelayMinutes: 4320, SubjectEn: "Find the right learning path", SubjectID: "Temukan jalur belajar yang tepat", BodyEn: "<p>Hi {name}, we can help you choose the right learning path.</p>", BodyID: "<p>Halo {name}, kami bisa membantu memilih jalur belajar yang tepat.</p>", IsEnabled: true},
	{Key: "welcome-no-purchase-h7", Name: "Registered, no purchase — H+7", Category: "follow_up", TriggerType: "registered_no_purchase", DelayMinutes: 10080, SubjectEn: "Your next step is waiting", SubjectID: "Langkah berikutnya menunggumu", BodyEn: "<p>Hi {name}, your Prometheus Academy account is ready whenever you are.</p>", BodyID: "<p>Halo {name}, akun Prometheus Academy kamu siap digunakan kapan saja.</p>", IsEnabled: true},
	{Key: "booking-reminder-24h", Name: "Booking reminder — 24 hours", Category: "booking", TriggerType: "booking_reminder", DelayMinutes: 1440, SubjectEn: "Your consultation is tomorrow", SubjectID: "Konsultasi kamu besok", BodyEn: "<p>Hi {name}, this is a reminder that your consultation is scheduled for {booking_time}.</p>", BodyID: "<p>Halo {name}, konsultasi kamu dijadwalkan pada {booking_time}.</p>", IsEnabled: true},
	{Key: "booking-reminder-1h", Name: "Booking reminder — 1 hour", Category: "booking", TriggerType: "booking_reminder", DelayMinutes: 60, SubjectEn: "Your consultation starts in one hour", SubjectID: "Konsultasi dimulai satu jam lagi", BodyEn: "<p>Hi {name}, your consultation starts at {booking_time}.</p>", BodyID: "<p>Halo {name}, konsultasi kamu dimulai pukul {booking_time}.</p>", IsEnabled: true},
	{Key: "abandoned-checkout-1h", Name: "Abandoned checkout — 1 hour", Category: "behavior", TriggerType: "abandoned_checkout", DelayMinutes: 60, SubjectEn: "Complete your order", SubjectID: "Selesaikan pesananmu", BodyEn: "<p>Hi {name}, your order is still waiting. Return to your dashboard to complete payment.</p>", BodyID: "<p>Halo {name}, pesananmu masih menunggu. Kembali ke dashboard untuk menyelesaikan pembayaran.</p>", IsEnabled: true},
	{Key: "course-inactive-7d", Name: "Course inactivity — 7 days", Category: "behavior", TriggerType: "course_inactive", DelayMinutes: 10080, SubjectEn: "Continue your course", SubjectID: "Lanjutkan course kamu", BodyEn: "<p>Hi {name}, continue where you left off in {course_name}.</p>", BodyID: "<p>Halo {name}, lanjutkan progres terakhir kamu di {course_name}.</p>", IsEnabled: true},
	{Key: "reengagement-30d", Name: "Re-engagement — 30 days", Category: "behavior", TriggerType: "reengagement", DelayMinutes: 43200, SubjectEn: "See what is new at Prometheus Academy", SubjectID: "Lihat yang baru di Prometheus Academy", BodyEn: "<p>Hi {name}, new learning opportunities are waiting for you.</p>", BodyID: "<p>Halo {name}, ada peluang belajar baru yang menunggumu.</p>", IsEnabled: true},
	{Key: "course-published", Name: "New course announcement", Category: "announcement", TriggerType: "course_published", SubjectEn: "New course: {item_name}", SubjectID: "Course baru: {item_name}", BodyEn: "<p>We have just published <strong>{item_name}</strong>. Explore it now on Prometheus Academy.</p>", BodyID: "<p>Kami baru menerbitkan <strong>{item_name}</strong>. Lihat sekarang di Prometheus Academy.</p>", IsEnabled: true},
	{Key: "product-published", Name: "New product/service announcement", Category: "announcement", TriggerType: "product_published", SubjectEn: "New: {item_name}", SubjectID: "Baru: {item_name}", BodyEn: "<p><strong>{item_name}</strong> is now available on Prometheus Academy.</p>", BodyID: "<p><strong>{item_name}</strong> sekarang tersedia di Prometheus Academy.</p>", IsEnabled: true},
}

var automationCTATokens = map[string]string{
	"welcome-no-purchase-h1": "courses_url", "welcome-no-purchase-h3": "courses_url", "welcome-no-purchase-h7": "dashboard_url",
	"booking-reminder-24h": "booking_url", "booking-reminder-1h": "booking_url", "abandoned-checkout-1h": "payment_url",
	"course-inactive-7d": "course_url", "reengagement-30d": "courses_url", "course-published": "item_url", "product-published": "item_url",
}

func automationCTA(key string, indonesia bool) string {
	token := automationCTATokens[key]
	labelsEn := map[string]string{"courses_url": "Explore Courses", "dashboard_url": "Open Dashboard", "booking_url": "View Booking", "payment_url": "Continue Payment", "course_url": "Continue Course", "item_url": "View Details"}
	labelsID := map[string]string{"courses_url": "Lihat Course", "dashboard_url": "Buka Dashboard", "booking_url": "Lihat Booking", "payment_url": "Lanjutkan Pembayaran", "course_url": "Lanjutkan Course", "item_url": "Lihat Detail"}
	label := labelsEn[token]
	if indonesia {
		label = labelsID[token]
	}
	return fmt.Sprintf(`<p><a href="{%s}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">%s</a></p>`, token, label)
}

func EnsureDefaultAutomationWorkflows(ctx context.Context, db *gorm.DB) error {
	for _, workflow := range defaultAutomationWorkflows {
		workflow.TemplateKey = defaultAutomationTemplateKey(workflow.Key)
		if err := db.WithContext(ctx).Where("`key` = ?", workflow.Key).FirstOrCreate(&workflow).Error; err != nil {
			return err
		}
		if strings.TrimSpace(workflow.TemplateKey) == "" {
			if err := db.WithContext(ctx).Model(&workflow).Update("template_key", defaultAutomationTemplateKey(workflow.Key)).Error; err != nil {
				return err
			}
		}
		token := automationCTATokens[workflow.Key]
		updates := map[string]any{}
		if token != "" && !strings.Contains(workflow.BodyEn, "{"+token+"}") {
			updates["body_en"] = workflow.BodyEn + automationCTA(workflow.Key, false)
		}
		if token != "" && !strings.Contains(workflow.BodyID, "{"+token+"}") {
			updates["body_id"] = workflow.BodyID + automationCTA(workflow.Key, true)
		}
		if len(updates) > 0 {
			if err := db.WithContext(ctx).Model(&workflow).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func defaultAutomationTemplateKey(workflowKey string) string {
	return "automation_" + strings.NewReplacer("-", "_").Replace(strings.TrimSpace(workflowKey))
}

func StartAutomationWorker(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	_ = EvaluateAutomations(ctx, db)
	for {
		select {
		case <-ticker.C:
			if err := EvaluateAutomations(ctx, db); err != nil {
				log.Warn().Err(err).Msg("automation worker failed")
			}
		case <-ctx.Done():
			return
		}
	}
}

func EvaluateAutomations(ctx context.Context, db *gorm.DB) error {
	cfg := config.Load()
	var workflows []models.AutomationWorkflow
	if err := db.WithContext(ctx).Where("is_enabled = ?", true).Find(&workflows).Error; err != nil {
		return err
	}
	for _, workflow := range workflows {
		switch workflow.TriggerType {
		case "registered_no_purchase":
			var users []models.User
			cutoff := time.Now().Add(-time.Duration(workflow.DelayMinutes) * time.Minute)
			query := db.WithContext(ctx).Where("users.created_at <= ?", cutoff)
			query = query.Where("NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = users.id AND o.status = 'success')")
			if err := query.Limit(500).Find(&users).Error; err == nil {
				for _, user := range users {
					scheduleAutomationRun(db, workflow, user, workflow.TriggerType, user.ID, cutoff, map[string]string{"dashboard_url": localizedFrontendURL(cfg, user.Language, "/dashboard"), "courses_url": localizedFrontendURL(cfg, user.Language, "/courses"), "services_url": localizedFrontendURL(cfg, user.Language, "/services")})
				}
			}
		case "reengagement":
			var users []models.User
			cutoff := time.Now().Add(-time.Duration(workflow.DelayMinutes) * time.Minute)
			if err := db.WithContext(ctx).Where("users.updated_at <= ?", cutoff).
				Where("NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = users.id AND o.updated_at > ?)", cutoff).
				Where("NOT EXISTS (SELECT 1 FROM course_enrollments ce WHERE ce.user_id = users.id AND ce.updated_at > ?)", cutoff).
				Where("NOT EXISTS (SELECT 1 FROM topic_progress tp WHERE tp.user_id = users.id AND tp.updated_at > ?)", cutoff).
				Limit(500).Find(&users).Error; err == nil {
				for _, user := range users {
					scheduleAutomationRun(db, workflow, user, "reengagement", user.ID, cutoff, map[string]string{"courses_url": localizedFrontendURL(cfg, user.Language, "/courses")})
				}
			}
		case "abandoned_checkout":
			var rows []struct {
				OrderID   uint
				UserID    uint
				CreatedAt time.Time
			}
			cutoff := time.Now().Add(-time.Duration(workflow.DelayMinutes) * time.Minute)
			_ = db.WithContext(ctx).Raw("SELECT id AS order_id, user_id, created_at FROM orders WHERE status = 'pending' AND created_at <= ? LIMIT 500", cutoff).Scan(&rows).Error
			for _, row := range rows {
				var user models.User
				if db.First(&user, row.UserID).Error == nil {
					scheduleAutomationRun(db, workflow, user, "order", row.OrderID, row.CreatedAt, map[string]string{"payment_url": localizedFrontendURL(cfg, user.Language, fmt.Sprintf("/dashboard?pay_order=%d", row.OrderID))})
				}
			}
		case "course_inactive":
			var rows []struct {
				EnrollmentID uint
				UserID       uint
				CourseName   string
				CourseSlug   string
				UpdatedAt    time.Time
			}
			cutoff := time.Now().Add(-time.Duration(workflow.DelayMinutes) * time.Minute)
			_ = db.WithContext(ctx).Raw(`SELECT ce.id enrollment_id, ce.user_id, c.title_en course_name, c.slug course_slug, GREATEST(ce.updated_at, COALESCE(MAX(tp.updated_at), ce.updated_at)) updated_at FROM course_enrollments ce JOIN courses c ON c.id=ce.course_id LEFT JOIN course_modules cm ON cm.course_id=ce.course_id LEFT JOIN topics t ON t.module_id=cm.id LEFT JOIN topic_progress tp ON tp.topic_id=t.id AND tp.user_id=ce.user_id WHERE ce.completed_at IS NULL GROUP BY ce.id,ce.user_id,c.title_en,c.slug,ce.updated_at HAVING updated_at <= ? LIMIT 500`, cutoff).Scan(&rows).Error
			for _, row := range rows {
				var user models.User
				if db.First(&user, row.UserID).Error == nil {
					scheduleAutomationRun(db, workflow, user, "enrollment", row.EnrollmentID, row.UpdatedAt, map[string]string{"course_name": row.CourseName, "course_url": localizedFrontendURL(cfg, user.Language, "/dashboard/courses/"+row.CourseSlug+"/learn")})
				}
			}
		case "booking_reminder":
			scheduleBookingReminders(ctx, db, workflow, cfg)
		}
	}
	return sendDueAutomationRuns(ctx, db)
}

func scheduleBookingReminders(ctx context.Context, db *gorm.DB, workflow models.AutomationWorkflow, cfg config.Config) {
	var rows []struct {
		BookingID, UserID     uint
		Email, Name, Language string
		StartsAt              time.Time
	}
	_ = db.WithContext(ctx).Raw(`SELECT b.id booking_id, u.id user_id, u.email, u.name, u.language, TIMESTAMP(s.date, s.time_start) starts_at FROM consultation_bookings b JOIN consultation_slots s ON s.id=b.slot_id JOIN users u ON u.id=b.user_id WHERE b.status IN ('pending','confirmed') AND TIMESTAMP(s.date, s.time_start) > NOW() AND TIMESTAMP(s.date, s.time_start) <= DATE_ADD(NOW(), INTERVAL ? MINUTE)`, workflow.DelayMinutes).Scan(&rows).Error
	for _, row := range rows {
		user := models.User{BaseModel: models.BaseModel{ID: row.UserID}, Name: row.Name, Email: row.Email, Language: row.Language}
		scheduleAutomationRun(db, workflow, user, "booking", row.BookingID, time.Now(), map[string]string{"booking_time": row.StartsAt.Format("02 Jan 2006 15:04"), "booking_url": localizedFrontendURL(cfg, user.Language, "/dashboard/bookings")})
	}
}

func scheduleAutomationRun(db *gorm.DB, workflow models.AutomationWorkflow, user models.User, entityType string, entityID uint, scheduledAt time.Time, variables map[string]string) {
	if strings.TrimSpace(user.Email) == "" {
		return
	}
	key := fmt.Sprintf("%s:%s:%d", workflow.Key, entityType, entityID)
	variablesJSON, _ := json.Marshal(variables)
	run := models.AutomationRun{WorkflowID: workflow.ID, UserID: user.ID, Email: strings.ToLower(user.Email), EntityType: entityType, EntityID: entityID, IdempotencyKey: key, Status: "scheduled", ScheduledAt: scheduledAt, VariablesJSON: string(variablesJSON)}
	_ = db.Where("idempotency_key = ?", key).FirstOrCreate(&run).Error
}

func sendDueAutomationRuns(ctx context.Context, db *gorm.DB) error {
	var runs []models.AutomationRun
	if err := db.WithContext(ctx).Where("status = 'scheduled' AND scheduled_at <= ?", time.Now()).Order("scheduled_at asc").Limit(100).Find(&runs).Error; err != nil {
		return err
	}
	settings, err := LoadMailerSettings(ctx, db)
	if err != nil {
		return err
	}
	for _, run := range runs {
		var suppression models.EmailSuppression
		if db.Where("email = ?", run.Email).First(&suppression).Error == nil {
			db.Model(&run).Updates(map[string]any{"status": "suppressed", "error_message": suppression.Reason})
			continue
		}
		var workflow models.AutomationWorkflow
		var user models.User
		if db.First(&workflow, run.WorkflowID).Error != nil || db.First(&user, run.UserID).Error != nil {
			continue
		}
		subject, body := workflow.SubjectEn, workflow.BodyEn
		if user.Language == "id" {
			subject, body = workflow.SubjectID, workflow.BodyID
		}
		subject = RenderMailerRecipientVariables(subject, MailerRecipient{ID: user.ID, Name: user.Name, Email: user.Email, Language: user.Language})
		body = RenderMailerRecipientVariables(body, MailerRecipient{ID: user.ID, Name: user.Name, Email: user.Email, Language: user.Language})
		variables := map[string]string{}
		_ = json.Unmarshal([]byte(run.VariablesJSON), &variables)
		subject = replaceMailerLayoutTokens(subject, variables)
		body = replaceMailerLayoutTokens(body, variables)
		templateKey := strings.TrimSpace(workflow.TemplateKey)
		if templateKey == "" {
			templateKey = defaultAutomationTemplateKey(workflow.Key)
		}
		var template models.EmailTemplate
		if err := db.WithContext(ctx).Where("`key` = ?", templateKey).First(&template).Error; err == nil {
			settings = SenderSettings(settings, template.SenderName, template.SenderEmail)
			body = RenderCampaignTemplateHTML(template, user.Language, subject, body, settings)
		} else {
			var fallbackTemplate models.EmailTemplate
			if err := db.WithContext(ctx).Where("`key` = ?", "campaign_simple").First(&fallbackTemplate).Error; err == nil {
				body = RenderCampaignTemplateHTML(fallbackTemplate, user.Language, subject, body, settings)
			}
		}
		messageID, sendErr := SendMailerEmail(ctx, settings, MailMessage{ToEmail: user.Email, ToName: user.Name, Subject: subject, HTML: body, Text: stripHTMLForEmail(body), Tags: []string{"automation", workflow.Key}})
		now := time.Now()
		if sendErr != nil {
			db.Model(&run).Updates(map[string]any{"status": "failed", "error_message": sendErr.Error()})
			db.Create(&models.EmailEvent{RunID: run.ID, Email: user.Email, EventType: "failed", OccurredAt: now})
			continue
		}
		db.Model(&run).Updates(map[string]any{"status": "sent", "sent_at": &now, "message_id": messageID})
		db.Create(&models.EmailEvent{RunID: run.ID, Email: user.Email, MessageID: messageID, EventType: "sent", OccurredAt: now})
	}
	return nil
}

func QueuePublishAnnouncement(ctx context.Context, db *gorm.DB, trigger string, entityID uint, itemName string) {
	var workflow models.AutomationWorkflow
	if db.WithContext(ctx).Where("trigger_type = ? AND is_enabled = ?", trigger, true).First(&workflow).Error != nil {
		return
	}
	var users []models.User
	_ = db.WithContext(ctx).Where("email <> ''").Find(&users).Error
	for _, user := range users {
		path, slug := "/courses/", ""
		if trigger == "product_published" {
			path = "/services/"
			_ = db.Model(&models.Product{}).Select("slug").Where("id = ?", entityID).Scan(&slug).Error
		} else {
			_ = db.Model(&models.Course{}).Select("slug").Where("id = ?", entityID).Scan(&slug).Error
		}
		variablesJSON, _ := json.Marshal(map[string]string{"item_name": itemName, "item_url": localizedFrontendURL(config.Load(), user.Language, path+slug)})
		key := fmt.Sprintf("%s:item:%d:user:%d", workflow.Key, entityID, user.ID)
		run := models.AutomationRun{WorkflowID: workflow.ID, UserID: user.ID, Email: user.Email, EntityType: trigger, EntityID: entityID, IdempotencyKey: key, Status: "scheduled", ScheduledAt: time.Now(), VariablesJSON: string(variablesJSON)}
		_ = db.Where("idempotency_key = ?", key).FirstOrCreate(&run).Error
	}
}
