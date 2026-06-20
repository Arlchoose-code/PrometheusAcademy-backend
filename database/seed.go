package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"academyprometheus/backend/models"
	"academyprometheus/backend/services"

	"gorm.io/gorm"
)

func Seed(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	ctx := context.Background()
	if err := cleanupRemovedCompanySeedData(ctx, db); err != nil {
		return err
	}
	if err := normalizeInstructorStudentRoles(ctx, db); err != nil {
		return err
	}

	admin, student, err := seedUsers(ctx, db)
	if err != nil {
		return err
	}
	if err := seedSettings(ctx, db); err != nil {
		return err
	}
	if err := services.SeedGamificationAchievements(ctx, db); err != nil {
		return err
	}
	if err := seedCourses(ctx, db, admin.ID); err != nil {
		return err
	}
	if err := seedCommerce(ctx, db, student.ID); err != nil {
		return err
	}
	if err := seedCMS(ctx, db); err != nil {
		return err
	}
	if err := seedTalentAndPartners(ctx, db); err != nil {
		return err
	}
	if err := seedDevelopmentRelations(ctx, db); err != nil {
		return err
	}

	return nil
}

func normalizeInstructorStudentRoles(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Model(&models.User{}).
		Where("is_instructor = ? AND is_admin = ? AND is_student = ?", true, false, true).
		Update("is_student", false).Error; err != nil {
		return fmt.Errorf("normalize instructor student roles: %w", err)
	}

	return nil
}

func cleanupRemovedCompanySeedData(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Where("email = ?", "marta@example.com").Delete(&models.User{}).Error; err != nil {
		return fmt.Errorf("delete removed seed company user: %w", err)
	}

	return nil
}

func seedUsers(ctx context.Context, db *gorm.DB) (models.User, models.User, error) {
	hash, err := services.HashPassword("Prometheus123!")
	if err != nil {
		return models.User{}, models.User{}, fmt.Errorf("seed admin password: %w", err)
	}
	verifiedAt := time.Now()

	admin := models.User{
		Name:            "Prometheus Admin",
		Email:           "admin@academyprometheus.com",
		Password:        hash,
		Phone:           "+62 812 0000 0000",
		IsStudent:       false,
		IsAdmin:         true,
		IsInstructor:    true,
		Language:        "en",
		EmailVerifiedAt: &verifiedAt,
	}
	if err := db.WithContext(ctx).Where(models.User{Email: admin.Email}).Assign(admin).FirstOrCreate(&admin).Error; err != nil {
		return admin, models.User{}, fmt.Errorf("seed admin user: %w", err)
	}

	student := models.User{Name: "Nadia Putri", Email: "nadia@example.com", Password: hash, IsStudent: true, Language: "id", EmailVerifiedAt: &verifiedAt}
	if err := db.WithContext(ctx).Where(models.User{Email: student.Email}).Assign(student).FirstOrCreate(&student).Error; err != nil {
		return admin, student, fmt.Errorf("seed student user: %w", err)
	}

	return admin, student, nil
}

func seedSettings(ctx context.Context, db *gorm.DB) error {
	settings := map[string]string{
		"site_name":                         "Prometheus Academy",
		"copyright_text":                    "Academy Prometheus 2026. All rights reserved",
		"phone":                             "+62 000 0000 0000",
		"email":                             "hello@academyprometheus.com",
		"facebook_url":                      "https://facebook.com",
		"instagram_url":                     "https://instagram.com",
		"tiktok_url":                        "https://tiktok.com",
		"home_students_stat":                "10K+",
		"home_courses_stat":                 "4",
		"home_regions_stat":                 "2",
		"home_support_stat":                 "7/24",
		"monthly_enrollment_limit":          "100",
		"monthly_enrollment_banner_enabled": "true",
		"talent_review_invite_hours":        "168",
		"mailer_provider":                   "gohighlevel",
		"mailer_from_email":                 "hello@academyprometheus.com",
		"mailer_from_name":                  "Prometheus Academy",
		"mailer_reply_to":                   "hello@academyprometheus.com",
		"ghl_access_token":                  "",
		"ghl_location_id":                   "",
		"ghl_api_base_url":                  "https://services.leadconnectorhq.com",
		"ghl_newsletter_tag":                "prometheus-newsletter",
		"ghl_contact_lead_tag":              "prometheus-website-lead",
		"brevo_api_key":                     "",
		"brevo_api_base_url":                "https://api.brevo.com/v3",
		"google_reviews_api_key":            "",
		"google_reviews_place_id":           "",
		"mailer_campaign_rate_per_minute":   "30",
	}
	for key, value := range services.TransactionalTemplateDefaults() {
		settings[key] = value
	}

	for key, value := range settings {
		setting := models.Setting{Key: key, Value: value}
		if err := db.WithContext(ctx).Where(models.Setting{Key: key}).Attrs(setting).FirstOrCreate(&setting).Error; err != nil {
			return fmt.Errorf("seed setting %s: %w", key, err)
		}
	}
	if err := db.WithContext(ctx).Model(&models.Setting{}).
		Where("`key` = ? AND value = ?", services.EmailTemplateLogin, "login_notification").
		Update("value", "otp_login").Error; err != nil {
		return fmt.Errorf("migrate login otp setting: %w", err)
	}

	return nil
}

func seedCourses(ctx context.Context, db *gorm.DB, instructorID uint) error {
	categories := []models.CourseCategory{
		{NameEn: "UI/UX Design", NameID: "Desain UI/UX", Slug: "ui-ux-design"},
		{NameEn: "Digital Marketing", NameID: "Digital Marketing", Slug: "digital-marketing"},
		{NameEn: "Financial Literacy", NameID: "Literasi Keuangan", Slug: "financial-literacy"},
		{NameEn: "AI & Machine Learning", NameID: "AI & Machine Learning", Slug: "ai-machine-learning"},
	}

	categoryIDs := map[string]uint{}
	for _, item := range categories {
		category := item
		if err := db.WithContext(ctx).Where(models.CourseCategory{Slug: category.Slug}).Attrs(category).FirstOrCreate(&category).Error; err != nil {
			return fmt.Errorf("seed course category %s: %w", item.Slug, err)
		}
		categoryIDs[item.Slug] = category.ID
	}

	courses := []models.Course{
		{
			TitleEn: "UI/UX Design Masterclass", TitleID: "Masterclass Desain UI/UX", Slug: "ui-ux-design-masterclass",
			DescriptionEn: "Design practical product experiences with research, wireframes, prototypes, and portfolio-ready case studies.",
			DescriptionID: "Bangun pengalaman produk melalui riset, wireframe, prototipe, dan studi kasus siap portofolio.",
			Thumbnail:     "/uploads/courses/ui-ux-design-masterclass.webp", Price: 750000, Status: "open", InstructorID: instructorID, CategoryID: categoryIDs["ui-ux-design"],
		},
		{
			TitleEn: "Digital Marketing Growth Lab", TitleID: "Lab Growth Digital Marketing", Slug: "digital-marketing-growth-lab",
			DescriptionEn: "Learn campaign strategy, analytics, funnel optimization, and content systems for measurable growth.",
			DescriptionID: "Pelajari strategi campaign, analitik, optimasi funnel, dan sistem konten untuk growth terukur.",
			Thumbnail:     "/uploads/courses/digital-marketing-growth-lab.webp", Price: 650000, Status: "open", InstructorID: instructorID, CategoryID: categoryIDs["digital-marketing"],
		},
		{
			TitleEn: "AI & Machine Learning Foundations", TitleID: "Dasar AI & Machine Learning", Slug: "ai-machine-learning-foundations",
			DescriptionEn: "Build AI literacy with data workflows, model concepts, prompt patterns, and applied mini projects.",
			DescriptionID: "Bangun literasi AI lewat workflow data, konsep model, pola prompt, dan mini project aplikatif.",
			Thumbnail:     "/uploads/courses/ai-machine-learning-foundations.webp", Price: 900000, Status: "open", InstructorID: instructorID, CategoryID: categoryIDs["ai-machine-learning"],
		},
	}

	for _, item := range courses {
		course := item
		if err := db.WithContext(ctx).Where(models.Course{Slug: course.Slug}).Attrs(course).FirstOrCreate(&course).Error; err != nil {
			return fmt.Errorf("seed course %s: %w", item.Slug, err)
		}

		module := models.CourseModule{CourseID: course.ID, TitleEn: "Foundation", TitleID: "Fondasi", Order: 1}
		if err := db.WithContext(ctx).Where(models.CourseModule{CourseID: course.ID, TitleEn: module.TitleEn}).Attrs(module).FirstOrCreate(&module).Error; err != nil {
			return fmt.Errorf("seed course module %s: %w", item.Slug, err)
		}

		topic := models.Topic{
			ModuleID:        module.ID,
			TitleEn:         "Orientation and roadmap",
			TitleID:         "Orientasi dan roadmap",
			ContentEn:       "A guided overview of outcomes, tools, practice rhythm, and portfolio expectations.",
			ContentID:       "Ikhtisar terarah tentang outcome, tools, ritme praktik, dan ekspektasi portofolio.",
			DurationSeconds: 900,
			Order:           1,
		}
		if err := db.WithContext(ctx).Where(models.Topic{ModuleID: module.ID, TitleEn: topic.TitleEn}).Attrs(topic).FirstOrCreate(&topic).Error; err != nil {
			return fmt.Errorf("seed course topic %s: %w", item.Slug, err)
		}
	}

	return nil
}

func seedCommerce(ctx context.Context, db *gorm.DB, userID uint) error {
	categories := []models.ProductCategory{
		{NameEn: "E-books", NameID: "E-book", Slug: "ebook", ShowInKnowledgeBase: true},
		{NameEn: "Consultation", NameID: "Konsultasi", Slug: "consultation", RequiresBookingTime: true},
		{NameEn: "Guides", NameID: "Panduan", Slug: "blueprint", ShowInKnowledgeBase: true},
		{NameEn: "Learning Resources", NameID: "Resource Belajar", Slug: "learning-resources", ShowInKnowledgeBase: true},
	}
	categoryIDs := map[string]uint{}
	for _, item := range categories {
		category := item
		if err := db.WithContext(ctx).Where(models.ProductCategory{Slug: category.Slug}).Attrs(category).FirstOrCreate(&category).Error; err != nil {
			return fmt.Errorf("seed product category %s: %w", item.Slug, err)
		}
		if err := db.WithContext(ctx).Model(&category).Updates(map[string]any{
			"name_en":                item.NameEn,
			"name_id":                item.NameID,
			"requires_booking_time":  item.RequiresBookingTime,
			"show_in_knowledge_base": item.ShowInKnowledgeBase && !item.RequiresBookingTime,
		}).Error; err != nil {
			return fmt.Errorf("seed product category booking flag %s: %w", item.Slug, err)
		}
		categoryIDs[item.Slug] = category.ID
	}
	if err := syncProductCategoriesFromType(ctx, db, categoryIDs); err != nil {
		return err
	}

	products := []models.Product{
		{
			TitleEn: "TOEFL & IELTS Mastery E-Book", TitleID: "E-Book Mastery TOEFL & IELTS", Slug: "toefl-ielts-mastery-ebook",
			DescriptionEn: "A practical study guide with scoring strategy, templates, and weekly practice plans.",
			DescriptionID: "Panduan belajar praktis dengan strategi skor, template, dan rencana latihan mingguan.",
			Thumbnail:     "/uploads/products/toefl-ielts-mastery-ebook.webp", Price: 149000, Type: "ebook", CategoryID: categoryIDs["ebook"], IsPublished: true,
		},
		{
			TitleEn: "Scholarship Consultation Call", TitleID: "Konsultasi Beasiswa", Slug: "scholarship-consultation-call",
			DescriptionEn: "A focused one-on-one session to review goals, documents, and application strategy.",
			DescriptionID: "Sesi 1-on-1 untuk review target, dokumen, dan strategi aplikasi.",
			Thumbnail:     "/uploads/products/scholarship-consultation-call.webp", Price: 399000, Type: "consultation", CategoryID: categoryIDs["consultation"], IsPublished: true,
		},
		{
			TitleEn: "Scholarship Application Blueprint", TitleID: "Blueprint Aplikasi Beasiswa", Slug: "scholarship-application-blueprint",
			DescriptionEn: "A complete structure for essays, timeline, recommendation letters, and interview prep.",
			DescriptionID: "Struktur lengkap untuk esai, timeline, surat rekomendasi, dan persiapan interview.",
			Thumbnail:     "/uploads/products/scholarship-application-blueprint.webp", Price: 249000, Type: "blueprint", CategoryID: categoryIDs["blueprint"], IsPublished: true,
		},
		{
			TitleEn: "Study Abroad Resource Kit", TitleID: "Resource Kit Studi Luar Negeri", Slug: "study-abroad-resource-kit",
			DescriptionEn: "Checklists, planning sheets, and practical references for preparing an international study plan.",
			DescriptionID: "Checklist, lembar rencana, dan referensi praktis untuk menyiapkan rencana studi internasional.",
			Thumbnail:     "/uploads/products/study-abroad-resource-kit.webp", Price: 99000, Type: "learning-resources", CategoryID: categoryIDs["learning-resources"], IsPublished: true,
		},
	}

	var firstProduct models.Product
	for index, item := range products {
		product := item
		if err := db.WithContext(ctx).Where(models.Product{Slug: product.Slug}).Attrs(product).FirstOrCreate(&product).Error; err != nil {
			return fmt.Errorf("seed product %s: %w", item.Slug, err)
		}
		if index == 0 {
			firstProduct = product
		}
	}

	file := models.ProductFile{ProductID: firstProduct.ID, FilePath: "/uploads/products/files/toefl-ielts-mastery.pdf", FileName: "TOEFL IELTS Mastery.pdf"}
	if err := db.WithContext(ctx).Where(models.ProductFile{ProductID: file.ProductID, FileName: file.FileName}).Attrs(file).FirstOrCreate(&file).Error; err != nil {
		return fmt.Errorf("seed product file: %w", err)
	}

	expiresAt := time.Now().AddDate(0, 3, 0)
	coupon := models.Coupon{Code: "PROMETHEUS25", DiscountType: "percent", DiscountValue: 25, MaxUses: 100, ExpiresAt: &expiresAt, AppliesTo: "all"}
	if err := db.WithContext(ctx).Where(models.Coupon{Code: coupon.Code}).Attrs(coupon).FirstOrCreate(&coupon).Error; err != nil {
		return fmt.Errorf("seed coupon: %w", err)
	}

	slotDate := time.Now().AddDate(0, 0, 7)
	slot := models.ConsultationSlot{Date: slotDate, TimeStart: "10:00", TimeEnd: "11:00", IsAvailable: true}
	if err := db.WithContext(ctx).Where(models.ConsultationSlot{Date: slotDate, TimeStart: "10:00"}).Attrs(slot).FirstOrCreate(&slot).Error; err != nil {
		return fmt.Errorf("seed consultation slot: %w", err)
	}

	order := models.Order{UserID: userID, TotalAmount: 149000, Status: "pending", MidtransOrderID: "SEED-ORDER-001"}
	if err := db.WithContext(ctx).Where(models.Order{MidtransOrderID: order.MidtransOrderID}).Assign(order).FirstOrCreate(&order).Error; err != nil {
		return fmt.Errorf("seed order: %w", err)
	}

	return nil
}

func syncProductCategoriesFromType(ctx context.Context, db *gorm.DB, categoryIDs map[string]uint) error {
	var categories []models.ProductCategory
	if err := db.WithContext(ctx).Find(&categories).Error; err != nil {
		return fmt.Errorf("load product categories for sync: %w", err)
	}
	for _, category := range categories {
		categoryIDs[category.Slug] = category.ID
	}
	for slug, id := range categoryIDs {
		if slug == "" || id == 0 {
			continue
		}
		if err := db.WithContext(ctx).
			Model(&models.Product{}).
			Where("type = ? AND category_id <> ?", slug, id).
			Update("category_id", id).Error; err != nil {
			return fmt.Errorf("sync product category %s: %w", slug, err)
		}
	}
	return nil
}

func seedCMS(ctx context.Context, db *gorm.DB) error {
	pages := []models.Page{
		{
			Slug:          "about",
			TitleEn:       "About Prometheus Academy",
			TitleID:       "Tentang Prometheus Academy",
			DescriptionEn: "Digital independence, practical European-standard education, and global collaboration for ambitious learners across Asia.",
			DescriptionID: "Kemandirian digital, edukasi praktis berstandar Eropa, dan kolaborasi global untuk learner ambisius di Asia.",
			ImagePath:     "/uploads/pages/about-founder.webp",
			ContentEn:     "<h2>About Us</h2><p>At Prometheus Academy, we believe digital independence is the foundation of true freedom in the 21st century.</p><h2>About Prometheus Academy</h2><p>Founded by David Nagy, Prometheus Academy is built to go beyond ordinary learning. Students can grow practical skills, join global collaborations, and prepare for real opportunities across Europe and Asia.</p><h3>Professors from European universities</h3><p>High-quality education delivered by experienced educators.</p><h3>Life-oriented courses</h3><p>Focused on real-world skills and practical application.</p>",
			ContentID:     "<h2>Tentang Kami</h2><p>Di Prometheus Academy, kami percaya kemandirian digital adalah dasar kebebasan sejati di abad ke-21.</p><h2>Tentang Prometheus Academy</h2><p>Didirikan oleh David Nagy, Prometheus Academy dibangun untuk melampaui pembelajaran biasa. Student dapat mengembangkan skill praktis, mengikuti kolaborasi global, dan bersiap untuk peluang nyata di Eropa dan Asia.</p><h3>Profesor dari universitas Eropa</h3><p>Pendidikan berkualitas dari pengajar berpengalaman.</p><h3>Kursus berorientasi kehidupan nyata</h3><p>Berfokus pada skill dunia nyata dan penerapan praktis.</p>",
		},
		{Slug: "privacy-policy", TitleEn: "Privacy Policy", TitleID: "Kebijakan Privasi", DescriptionEn: "How Prometheus Academy handles personal data.", DescriptionID: "Cara Prometheus Academy mengelola data pribadi.", ContentEn: "<p>We process data responsibly for learning, hiring, and partnership services.</p>", ContentID: "<p>Kami memproses data secara bertanggung jawab untuk layanan belajar, hiring, dan partnership.</p>"},
		{Slug: "terms", TitleEn: "Terms of Service", TitleID: "Syarat Layanan", DescriptionEn: "Terms for using Prometheus Academy.", DescriptionID: "Syarat penggunaan Prometheus Academy.", ContentEn: "<p>These terms explain how learners, companies, and partners use the platform.</p>", ContentID: "<p>Syarat ini menjelaskan penggunaan platform oleh learner, perusahaan, dan partner.</p>"},
	}
	for _, item := range pages {
		page := item
		if err := db.WithContext(ctx).Where(models.Page{Slug: page.Slug}).Attrs(page).FirstOrCreate(&page).Error; err != nil {
			return fmt.Errorf("seed page %s: %w", item.Slug, err)
		}
	}

	faqs := []models.FAQ{
		{QuestionEn: "Are courses available in English and Indonesian?", QuestionID: "Apakah kursus tersedia dalam bahasa Inggris dan Indonesia?", AnswerEn: "Yes. Public content and course materials are prepared bilingually.", AnswerID: "Ya. Konten publik dan materi kursus disiapkan bilingual.", Order: 1},
		{QuestionEn: "Can companies hire through Talent Bridge?", QuestionID: "Apakah perusahaan bisa hiring lewat Talent Bridge?", AnswerEn: "Yes. Companies can request managed staffing support for Asia-based talent.", AnswerID: "Bisa. Perusahaan dapat meminta dukungan managed staffing untuk talenta berbasis Asia.", Order: 2},
	}
	for _, item := range faqs {
		faq := item
		if err := db.WithContext(ctx).Where(models.FAQ{QuestionEn: faq.QuestionEn}).Attrs(faq).FirstOrCreate(&faq).Error; err != nil {
			return fmt.Errorf("seed faq: %w", err)
		}
	}

	testimonials := []models.Testimonial{
		{Name: "Nadia Putri", Role: "Scholarship Applicant", Company: "Indonesia", ContentEn: "The roadmap made my IELTS preparation and scholarship documents feel structured.", ContentID: "Roadmap-nya bikin persiapan IELTS dan dokumen beasiswa jadi jauh lebih terarah.", Rating: 5, ReviewSource: "student", DisplayContext: "all", ReviewStatus: "approved", IsActive: true},
		{Name: "Marta Schneider", Role: "Talent Partner", Company: "Berlin SaaS Studio", ContentEn: "Talent Bridge helped us understand Asia-based hiring without adding operational noise.", ContentID: "Talent Bridge membantu kami memahami hiring talenta Asia tanpa menambah beban operasional.", Rating: 5, ReviewSource: "google", DisplayContext: "talent_bridge", ReviewStatus: "approved", IsActive: true},
		{Name: "Raka Wibowo", Role: "UI/UX Learner", Company: "Jakarta", ContentEn: "The course pushed me to finish a portfolio case study that recruiters could actually read.", ContentID: "Kursusnya mendorong saya menyelesaikan studi kasus portofolio yang benar-benar bisa dibaca recruiter.", Rating: 5, ReviewSource: "student", DisplayContext: "all", ReviewStatus: "approved", IsActive: true},
		{Name: "Elena Kovacs", Role: "People Operations", Company: "Budapest Growth Lab", ContentEn: "The Talent Bridge process gave us a clearer view of Asia-based remote candidates and the support needed to onboard them.", ContentID: "Proses Talent Bridge memberi gambaran lebih jelas tentang kandidat remote berbasis Asia dan dukungan yang dibutuhkan untuk onboarding.", Rating: 5, ReviewSource: "google", DisplayContext: "talent_bridge", ReviewStatus: "approved", IsActive: true},
	}
	for _, item := range testimonials {
		testimonial := item
		if err := db.WithContext(ctx).Where(models.Testimonial{Name: testimonial.Name}).Assign(testimonial).FirstOrCreate(&testimonial).Error; err != nil {
			return fmt.Errorf("seed testimonial %s: %w", item.Name, err)
		}
	}

	banner := models.Banner{TitleEn: "Europe x Asia learning bridge", TitleID: "Jembatan belajar Eropa x Asia", LinkURL: "/", IsActive: true, Order: 1}
	if err := db.WithContext(ctx).Where(models.Banner{TitleEn: banner.TitleEn}).Attrs(banner).FirstOrCreate(&banner).Error; err != nil {
		return fmt.Errorf("seed banner: %w", err)
	}

	defaultEmailDesign := models.EmailDesign{
		Name:            "Default Academy Email",
		Description:     "Brand email wrapper with logo, heading, body, CTA, and footer blocks.",
		BackgroundColor: "#F8F9FA",
		ContentColor:    "#FFFFFF",
		AccentColor:     "#C9A84C",
		TextColor:       "#212529",
		Width:           620,
		BlocksJSON:      `[{"id":"logo","type":"logo","content":"Prometheus Academy"},{"id":"heading","type":"heading","content":"{{subject}}"},{"id":"body","type":"body","content":"{{content}}"},{"id":"button","type":"button","content":"Visit Dashboard","href":"{{dashboard_url}}"},{"id":"footer","type":"footer","content":"Prometheus Academy<br/>Europe x Asia learning bridge."}]`,
		IsDefault:       true,
	}
	if err := db.WithContext(ctx).Where(models.EmailDesign{Name: defaultEmailDesign.Name}).Attrs(defaultEmailDesign).FirstOrCreate(&defaultEmailDesign).Error; err != nil {
		return fmt.Errorf("seed email design: %w", err)
	}

	templates := []models.EmailTemplate{
		{DesignID: defaultEmailDesign.ID, Key: "campaign_simple", SubjectEn: "{subject}", SubjectID: "{subject}", PreheaderEn: "Simple editable campaign wrapper.", PreheaderID: "Wrapper campaign sederhana yang dapat diedit.", BodyEn: seedEmailTemplateHTML("{subject}", "{content}", "You receive this email from Prometheus Academy."), BodyID: seedEmailTemplateHTML("{subject}", "{content}", "Kamu menerima email ini dari Prometheus Academy."), FooterEn: "<p>You receive this email from Prometheus Academy.</p>", FooterID: "<p>Kamu menerima email ini dari Prometheus Academy.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "campaign_newsletter", SubjectEn: "Prometheus Academy update", SubjectID: "Update Prometheus Academy", PreheaderEn: "Latest learning and program updates.", PreheaderID: "Update terbaru seputar pembelajaran dan program.", BodyEn: seedEmailTemplateHTML("Prometheus Academy newsletter", "{content}", "You receive this because you subscribed to Prometheus Academy updates."), BodyID: seedEmailTemplateHTML("Newsletter Prometheus Academy", "{content}", "Kamu menerima email ini karena berlangganan update Prometheus Academy."), FooterEn: "<p>You receive this because you subscribed to Prometheus Academy updates.</p>", FooterID: "<p>Kamu menerima email ini karena berlangganan update Prometheus Academy.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "campaign_announcement", SubjectEn: "Important announcement", SubjectID: "Pengumuman penting", PreheaderEn: "A new announcement from Prometheus Academy.", PreheaderID: "Pengumuman terbaru dari Prometheus Academy.", BodyEn: seedEmailTemplateHTML("Important announcement", "{content}", "Prometheus Academy - Europe x Asia learning bridge."), BodyID: seedEmailTemplateHTML("Pengumuman penting", "{content}", "Prometheus Academy - jembatan belajar Eropa x Asia."), FooterEn: "<p>Prometheus Academy - Europe x Asia learning bridge.</p>", FooterID: "<p>Prometheus Academy - jembatan belajar Eropa x Asia.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "welcome", SubjectEn: "Welcome to Prometheus Academy", SubjectID: "Selamat datang di Prometheus Academy", PreheaderEn: "Your account is ready.", PreheaderID: "Akun kamu sudah siap.", BodyEn: seedEmailTemplateHTML("Welcome to Prometheus Academy", `<p>Hi {name},</p><p>Your account is ready. You can now explore courses, services, and dashboard tools.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Open Dashboard</a></p>`, "Prometheus Academy - Europe x Asia learning bridge."), BodyID: seedEmailTemplateHTML("Selamat datang di Prometheus Academy", `<p>Hi {name},</p><p>Akun kamu sudah siap. Kamu bisa mulai membuka kursus, layanan, dan dashboard.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Buka Dashboard</a></p>`, "Prometheus Academy - jembatan belajar Eropa x Asia."), FooterEn: "<p>Prometheus Academy - Europe x Asia learning bridge.</p>", FooterID: "<p>Prometheus Academy - jembatan belajar Eropa x Asia.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "email_verification", SubjectEn: "Verify your email", SubjectID: "Verifikasi email kamu", PreheaderEn: "Use the verification code to finish registration.", PreheaderID: "Gunakan kode verifikasi untuk menyelesaikan registrasi.", BodyEn: seedEmailTemplateHTML("Verify your email", `<p>Hi {name},</p><p>Use this verification code to finish your registration:</p><p style="font-size:28px;font-weight:800;letter-spacing:4px;color:#0D1B2E;">{otp}</p>`, "This code expires soon. Do not share it with anyone."), BodyID: seedEmailTemplateHTML("Verifikasi email kamu", `<p>Hi {name},</p><p>Gunakan kode verifikasi ini untuk menyelesaikan registrasi:</p><p style="font-size:28px;font-weight:800;letter-spacing:4px;color:#0D1B2E;">{otp}</p>`, "Kode ini akan kedaluwarsa. Jangan bagikan ke siapa pun."), FooterEn: "<p>This code expires soon. Do not share it with anyone.</p>", FooterID: "<p>Kode ini akan kedaluwarsa. Jangan bagikan ke siapa pun.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "login_notification", SubjectEn: "New login to your account", SubjectID: "Login baru ke akun kamu", PreheaderEn: "Your Prometheus Academy account was just used to sign in.", PreheaderID: "Akun Prometheus Academy kamu baru saja digunakan untuk login.", BodyEn: seedEmailTemplateHTML("New login to your account", `<p>Hi {name},</p><p>Your account was just used to sign in.</p><p>Login time: <strong>{login_time}</strong></p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Open Dashboard</a></p>`, "If this was not you, reset your password immediately."), BodyID: seedEmailTemplateHTML("Login baru ke akun kamu", `<p>Hi {name},</p><p>Akun kamu baru saja digunakan untuk login.</p><p>Waktu login: <strong>{login_time}</strong></p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Buka Dashboard</a></p>`, "Kalau ini bukan kamu, segera reset password."), FooterEn: "<p>If this was not you, reset your password immediately.</p>", FooterID: "<p>Kalau ini bukan kamu, segera reset password.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "otp_login", SubjectEn: "Your login code", SubjectID: "Kode login kamu", PreheaderEn: "Use this code to continue signing in.", PreheaderID: "Gunakan kode ini untuk melanjutkan login.", BodyEn: seedEmailTemplateHTML("Your login code", `<p>Hi {name},</p><p>Here is your login code:</p><p style="font-size:28px;font-weight:800;letter-spacing:4px;color:#0D1B2E;">{otp}</p>`, "If this was not you, ignore this email."), BodyID: seedEmailTemplateHTML("Kode login kamu", `<p>Hi {name},</p><p>Ini kode login kamu:</p><p style="font-size:28px;font-weight:800;letter-spacing:4px;color:#0D1B2E;">{otp}</p>`, "Kalau bukan kamu, abaikan email ini."), FooterEn: "<p>If this was not you, ignore this email.</p>", FooterID: "<p>Kalau bukan kamu, abaikan email ini.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "password_reset", SubjectEn: "Reset your password", SubjectID: "Reset password kamu", PreheaderEn: "Use the secure link to set a new password.", PreheaderID: "Gunakan link aman untuk membuat password baru.", BodyEn: seedEmailTemplateHTML("Reset your password", `<p>Hi {name},</p><p>Click the button below to set a new password.</p><p><a href="{reset_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Reset Password</a></p>`, "If you did not request this, you can ignore this email."), BodyID: seedEmailTemplateHTML("Reset password kamu", `<p>Hi {name},</p><p>Klik tombol di bawah untuk membuat password baru.</p><p><a href="{reset_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Reset Password</a></p>`, "Kalau kamu tidak meminta ini, abaikan email ini."), FooterEn: "<p>If you did not request this, you can ignore this email.</p>", FooterID: "<p>Kalau kamu tidak meminta ini, abaikan email ini.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "invoice", SubjectEn: "Your invoice is ready", SubjectID: "Invoice kamu sudah siap", PreheaderEn: "Invoice {invoice_number} is attached.", PreheaderID: "Invoice {invoice_number} terlampir.", BodyEn: seedEmailTemplateHTML("Your invoice is ready", `<p>Thanks for your purchase, {name}.</p><p>Invoice number: <strong>{invoice_number}</strong></p><p><a href="{invoice_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Open Invoice</a></p>`, "Keep this invoice for your records."), BodyID: seedEmailTemplateHTML("Invoice kamu sudah siap", `<p>Terima kasih atas pembeliannya, {name}.</p><p>Nomor invoice: <strong>{invoice_number}</strong></p><p><a href="{invoice_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Buka Invoice</a></p>`, "Simpan invoice ini untuk arsip kamu."), FooterEn: "<p>Keep this invoice for your records.</p>", FooterID: "<p>Simpan invoice ini untuk arsip kamu.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "payment_success", SubjectEn: "Payment confirmed", SubjectID: "Pembayaran terkonfirmasi", PreheaderEn: "Your order has been paid successfully.", PreheaderID: "Pesanan kamu berhasil dibayar.", BodyEn: seedEmailTemplateHTML("Payment confirmed", `<p>Hi {name},</p><p>Your payment for <strong>{product}</strong> has been confirmed.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Open Dashboard</a></p>`, "Thank you for learning with Prometheus Academy."), BodyID: seedEmailTemplateHTML("Pembayaran terkonfirmasi", `<p>Hi {name},</p><p>Pembayaran untuk <strong>{product}</strong> sudah terkonfirmasi.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Buka Dashboard</a></p>`, "Terima kasih sudah belajar bersama Prometheus Academy."), FooterEn: "<p>Thank you for learning with Prometheus Academy.</p>", FooterID: "<p>Terima kasih sudah belajar bersama Prometheus Academy.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "deposit_confirmation", SubjectEn: "Deposit confirmed", SubjectID: "Deposit terkonfirmasi", PreheaderEn: "Your deposit has been received.", PreheaderID: "Deposit kamu sudah diterima.", BodyEn: seedEmailTemplateHTML("Deposit confirmed", `<p>Hi {name},</p><p>Your deposit of <strong>{amount}</strong> has been received.</p><p>Reference: <strong>{transaction_id}</strong></p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">View Transaction</a></p>`, "You can check the transaction from your dashboard."), BodyID: seedEmailTemplateHTML("Deposit terkonfirmasi", `<p>Hi {name},</p><p>Deposit sebesar <strong>{amount}</strong> sudah diterima.</p><p>Referensi: <strong>{transaction_id}</strong></p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Lihat Transaksi</a></p>`, "Kamu bisa cek transaksi dari dashboard."), FooterEn: "<p>You can check the transaction from your dashboard.</p>", FooterID: "<p>Kamu bisa cek transaksi dari dashboard.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "certificate", SubjectEn: "Your certificate is ready", SubjectID: "Sertifikat kamu sudah siap", PreheaderEn: "Congratulations on completing {course}.", PreheaderID: "Selamat menyelesaikan {course}.", BodyEn: seedEmailTemplateHTML("Your certificate is ready", `<p>Congratulations {name},</p><p>Your certificate for <strong>{course}</strong> is ready.</p><p><a href="{certificate_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Download Certificate</a></p>`, "Share your certificate with your network."), BodyID: seedEmailTemplateHTML("Sertifikat kamu sudah siap", `<p>Selamat {name},</p><p>Sertifikat untuk <strong>{course}</strong> sudah siap.</p><p><a href="{certificate_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Download Sertifikat</a></p>`, "Bagikan sertifikat kamu ke network kamu."), FooterEn: "<p>Share your certificate with your network.</p>", FooterID: "<p>Bagikan sertifikat kamu ke network kamu.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "booking_confirmation", SubjectEn: "Booking confirmed", SubjectID: "Booking terkonfirmasi", PreheaderEn: "Your consultation schedule is confirmed.", PreheaderID: "Jadwal konsultasi kamu sudah terkonfirmasi.", BodyEn: seedEmailTemplateHTML("Booking confirmed", `<p>Your booking is confirmed, {name}.</p><p>Schedule: <strong>{booking_time}</strong></p><p>Consultation: <strong>{service}</strong></p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Manage Booking</a></p>`, "Need help? Reply to this email."), BodyID: seedEmailTemplateHTML("Booking terkonfirmasi", `<p>Booking kamu terkonfirmasi, {name}.</p><p>Jadwal: <strong>{booking_time}</strong></p><p>Konsultasi: <strong>{service}</strong></p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Kelola Booking</a></p>`, "Butuh bantuan? Balas email ini."), FooterEn: "<p>Need help? Reply to this email.</p>", FooterID: "<p>Butuh bantuan? Balas email ini.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "talent_review_invitation", SubjectEn: "Share your Talent Bridge experience", SubjectID: "Bagikan pengalaman Talent Bridge kamu", PreheaderEn: "Your private review invitation is ready.", PreheaderID: "Undangan review privat kamu sudah siap.", BodyEn: seedEmailTemplateHTML("Share your Talent Bridge experience", `<p>Hi {name},</p><p>Thank you for being part of Talent Bridge. Use your private one-time link to share your experience.</p><p><a href="{review_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Write a Review</a></p><p>This invitation expires on <strong>{expires_at}</strong>.</p>`, "Your review will be moderated before it appears publicly."), BodyID: seedEmailTemplateHTML("Bagikan pengalaman Talent Bridge kamu", `<p>Hai {name},</p><p>Terima kasih sudah menjadi bagian dari Talent Bridge. Gunakan link privat sekali pakai untuk membagikan pengalaman kamu.</p><p><a href="{review_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Tulis Review</a></p><p>Undangan ini berlaku sampai <strong>{expires_at}</strong>.</p>`, "Review kamu akan dimoderasi sebelum tampil di publik."), FooterEn: "<p>Your review will be moderated before it appears publicly.</p>", FooterID: "<p>Review kamu akan dimoderasi sebelum tampil di publik.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "talent_application_received", SubjectEn: "Your Talent Bridge application was received", SubjectID: "Aplikasi Talent Bridge kamu diterima", PreheaderEn: "We received your application and will review it soon.", PreheaderID: "Kami menerima aplikasi kamu dan akan segera meninjaunya.", BodyEn: seedEmailTemplateHTML("Application received", `<p>Hi {name},</p><p>We received your <strong>{application_type}</strong> application. Our team will review it and update you by email.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Track from Dashboard</a></p><p>If you do not have an account yet, you can create one with this email to track future updates.</p>`, "Prometheus Academy Talent Bridge team."), BodyID: seedEmailTemplateHTML("Aplikasi diterima", `<p>Hai {name},</p><p>Kami menerima aplikasi <strong>{application_type}</strong> kamu. Tim kami akan meninjau dan mengirim update lewat email.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Pantau dari Dashboard</a></p><p>Kalau belum punya akun, kamu bisa daftar memakai email ini untuk memantau update berikutnya.</p>`, "Tim Prometheus Academy Talent Bridge."), FooterEn: "<p>Prometheus Academy Talent Bridge team.</p>", FooterID: "<p>Tim Prometheus Academy Talent Bridge.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "talent_status_update", SubjectEn: "Talent Bridge status update", SubjectID: "Update status Talent Bridge", PreheaderEn: "Your application status has changed.", PreheaderID: "Status aplikasi kamu berubah.", BodyEn: seedEmailTemplateHTML("Talent Bridge update", `<p>Hi {name},</p><p>Your <strong>{application_type}</strong> application status is now <strong>{status}</strong>.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Open Dashboard</a></p>`, "We will contact you if we need more details."), BodyID: seedEmailTemplateHTML("Update Talent Bridge", `<p>Hai {name},</p><p>Status aplikasi <strong>{application_type}</strong> kamu sekarang <strong>{status}</strong>.</p><p><a href="{dashboard_url}" style="display:inline-block;background:#C9A84C;color:#0D1B2E;text-decoration:none;border-radius:10px;padding:12px 18px;font-weight:800;">Buka Dashboard</a></p>`, "Kami akan menghubungi kamu jika butuh detail tambahan."), FooterEn: "<p>We will contact you if we need more details.</p>", FooterID: "<p>Kami akan menghubungi kamu jika butuh detail tambahan.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "hiring_inquiry_received", SubjectEn: "We received your hiring inquiry", SubjectID: "Permintaan hiring kamu diterima", PreheaderEn: "Our Talent Bridge team will follow up soon.", PreheaderID: "Tim Talent Bridge akan segera menindaklanjuti.", BodyEn: seedEmailTemplateHTML("Hiring inquiry received", `<p>Hi {name},</p><p>Thanks for contacting Prometheus Academy. We received your hiring inquiry for <strong>{company_name}</strong>.</p><p>Roles needed: <strong>{roles_needed}</strong></p><p>Our team will follow up with next steps.</p>`, "Prometheus Academy Talent Bridge team."), BodyID: seedEmailTemplateHTML("Permintaan hiring diterima", `<p>Hai {name},</p><p>Terima kasih sudah menghubungi Prometheus Academy. Kami menerima permintaan hiring untuk <strong>{company_name}</strong>.</p><p>Role yang dibutuhkan: <strong>{roles_needed}</strong></p><p>Tim kami akan menghubungi untuk langkah berikutnya.</p>`, "Tim Prometheus Academy Talent Bridge."), FooterEn: "<p>Prometheus Academy Talent Bridge team.</p>", FooterID: "<p>Tim Prometheus Academy Talent Bridge.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
		{DesignID: defaultEmailDesign.ID, Key: "partner_application_received", SubjectEn: "Your partner application was received", SubjectID: "Aplikasi partner kamu diterima", PreheaderEn: "We received your university partnership application.", PreheaderID: "Kami menerima aplikasi kerja sama universitas kamu.", BodyEn: seedEmailTemplateHTML("Partner application received", `<p>Hi {name},</p><p>We received the partnership application for <strong>{university_name}</strong>.</p><p>Our team will review it and contact you with next steps.</p>`, "Prometheus Academy Partnership team."), BodyID: seedEmailTemplateHTML("Aplikasi partner diterima", `<p>Hai {name},</p><p>Kami menerima aplikasi kerja sama untuk <strong>{university_name}</strong>.</p><p>Tim kami akan meninjau dan menghubungi kamu untuk langkah berikutnya.</p>`, "Tim Partnership Prometheus Academy."), FooterEn: "<p>Prometheus Academy Partnership team.</p>", FooterID: "<p>Tim Partnership Prometheus Academy.</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"},
	}
	for _, workflow := range []struct {
		Key       string
		SubjectEn string
		SubjectID string
		TitleEn   string
		TitleID   string
		FooterEn  string
		FooterID  string
	}{
		{"automation_welcome_no_purchase_h1", "Ready to start learning?", "Siap mulai belajar?", "Ready to start learning?", "Siap mulai belajar?", "This is an automated follow-up from Prometheus Academy.", "Ini adalah follow-up otomatis dari Prometheus Academy."},
		{"automation_welcome_no_purchase_h3", "Find the right learning path", "Temukan jalur belajar yang tepat", "Find the right learning path", "Temukan jalur belajar yang tepat", "You can explore courses and services from your dashboard.", "Kamu bisa membuka course dan layanan dari dashboard."},
		{"automation_welcome_no_purchase_h7", "Your next step is waiting", "Langkah berikutnya menunggumu", "Your next step is waiting", "Langkah berikutnya menunggumu", "Prometheus Academy is ready whenever you are.", "Prometheus Academy siap kapan pun kamu siap."},
		{"automation_booking_reminder_24h", "Your consultation is tomorrow", "Konsultasi kamu besok", "Consultation reminder", "Pengingat konsultasi", "Need to reschedule? Open your dashboard.", "Perlu ubah jadwal? Buka dashboard kamu."},
		{"automation_booking_reminder_1h", "Your consultation starts in one hour", "Konsultasi dimulai satu jam lagi", "Consultation starts soon", "Konsultasi segera dimulai", "Please be ready before the session starts.", "Pastikan kamu sudah siap sebelum sesi dimulai."},
		{"automation_abandoned_checkout_1h", "Complete your order", "Selesaikan pesananmu", "Complete your order", "Selesaikan pesananmu", "Your pending order is still saved.", "Pesanan tertunda kamu masih tersimpan."},
		{"automation_course_inactive_7d", "Continue your course", "Lanjutkan course kamu", "Continue your course", "Lanjutkan course kamu", "Small progress still counts.", "Progress kecil tetap berarti."},
		{"automation_reengagement_30d", "See what is new at Prometheus Academy", "Lihat yang baru di Prometheus Academy", "See what is new", "Lihat yang baru", "New learning opportunities are waiting.", "Peluang belajar baru menunggu kamu."},
		{"automation_course_published", "New course: {item_name}", "Course baru: {item_name}", "New course published", "Course baru terbit", "Explore this course on Prometheus Academy.", "Lihat course ini di Prometheus Academy."},
		{"automation_product_published", "New: {item_name}", "Baru: {item_name}", "New product/service published", "Produk/layanan baru terbit", "This update is available now.", "Update ini sudah tersedia sekarang."},
	} {
		templates = append(templates, models.EmailTemplate{DesignID: defaultEmailDesign.ID, Key: workflow.Key, SubjectEn: workflow.SubjectEn, SubjectID: workflow.SubjectID, PreheaderEn: workflow.FooterEn, PreheaderID: workflow.FooterID, BodyEn: seedEmailTemplateHTML(workflow.TitleEn, "{content}", workflow.FooterEn), BodyID: seedEmailTemplateHTML(workflow.TitleID, "{content}", workflow.FooterID), FooterEn: "<p>" + workflow.FooterEn + "</p>", FooterID: "<p>" + workflow.FooterID + "</p>", BackgroundColor: "#F8F9FA", AccentColor: "#C9A84C"})
	}
	for _, item := range templates {
		template := item
		if err := db.WithContext(ctx).Where(models.EmailTemplate{Key: template.Key}).Assign(template).FirstOrCreate(&template).Error; err != nil {
			return fmt.Errorf("seed email template %s: %w", item.Key, err)
		}
	}

	return nil
}

func seedEmailTemplateHTML(title string, content string, footer string) string {
	return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#F8F9FA;font-family:Arial,Helvetica,sans-serif;color:#212529;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#F8F9FA;padding:24px 12px;">
      <tr>
        <td align="center">
          <table role="presentation" width="620" cellspacing="0" cellpadding="0" style="width:620px;max-width:100%;background:#FFFFFF;border-radius:14px;overflow:hidden;border:1px solid #E9ECEF;">
            <tr><td style="height:4px;background:#C9A84C;font-size:0;line-height:0;">&nbsp;</td></tr>
            <tr>
              <td style="padding:24px 28px;border-bottom:1px solid #E9ECEF;">
                <table role="presentation" cellspacing="0" cellpadding="0" style="width:auto;">
                  <tr>
                    <td style="vertical-align:middle;width:42px;">
                      <span style="display:inline-block;width:42px;height:42px;border-radius:999px;background:#C9A84C;color:#0D1B2E;font-weight:800;line-height:42px;text-align:center;">P</span>
                    </td>
                    <td style="vertical-align:middle;padding-left:12px;font-size:16px;font-weight:800;color:#0D1B2E;white-space:nowrap;">{site_name}</td>
                  </tr>
                </table>
              </td>
            </tr>
            <tr>
              <td style="padding:28px 28px 8px;">
                <h1 style="margin:0;color:#212529;font-size:26px;line-height:1.3;font-weight:800;">` + title + `</h1>
              </td>
            </tr>
            <tr>
              <td style="padding:12px 28px 32px;font-size:15px;line-height:1.7;color:#343A40;">
                ` + content + `
              </td>
            </tr>
            <tr>
              <td style="padding:20px 28px;background:#F8F9FA;border-top:1px solid #E9ECEF;color:#6C757D;font-size:12px;line-height:1.6;">
                <strong style="color:#0D1B2E;">{site_name}</strong><br/>
                ` + footer + `
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`
}

func seedTalentAndPartners(ctx context.Context, db *gorm.DB) error {
	jobs := []models.TalentJob{
		{TitleEn: "Remote UI Designer", TitleID: "Remote UI Designer", Slug: "remote-ui-designer", DescriptionEn: "Design dashboards and web products for a European SaaS team.", DescriptionID: "Mendesain dashboard dan produk web untuk tim SaaS Eropa.", OpenPositions: 2, Status: "open"},
		{TitleEn: "Growth Marketing Associate", TitleID: "Growth Marketing Associate", Slug: "growth-marketing-associate", DescriptionEn: "Support campaign planning, analytics, and lifecycle marketing.", DescriptionID: "Mendukung perencanaan campaign, analitik, dan lifecycle marketing.", OpenPositions: 3, Status: "open"},
	}
	for _, item := range jobs {
		job := item
		if err := db.WithContext(ctx).Where(models.TalentJob{Slug: job.Slug}).Attrs(job).FirstOrCreate(&job).Error; err != nil {
			return fmt.Errorf("seed talent job %s: %w", item.Slug, err)
		}
	}

	partners := []models.Partner{
		{PartnerType: "university", Name: "Jakarta Tech Institute", Country: "Indonesia", Logo: "/uploads/partners/jakarta-tech-institute.webp", Website: "https://example.com", ContactInfo: "international-office@jti.example", DescriptionEn: "University partner for digital skills and global employability programs.", DescriptionID: "Partner universitas untuk digital skills dan program employability global.", Status: "active", Notes: "Seed partner for homepage university showcase.", IsActive: true},
		{PartnerType: "university", Name: "Budapest Global University", Country: "Hungary", Logo: "/uploads/partners/budapest-global-university.webp", Website: "https://example.com", ContactInfo: "partnerships@bgu.example", DescriptionEn: "European partner for academic exchange and talent development.", DescriptionID: "Partner Eropa untuk pertukaran akademik dan pengembangan talenta.", Status: "active", Notes: "Seed partner for homepage university showcase.", IsActive: true},
		{PartnerType: "company", Name: "Berlin SaaS Studio", Country: "Germany", Logo: "/uploads/partners/berlin-saas-studio.webp", Website: "https://example.com", ContactInfo: "talent@berlinsaas.example", DescriptionEn: "Hiring partner for remote product and growth roles.", DescriptionID: "Partner hiring untuk role remote product dan growth.", Status: "active", Notes: "Seed partner for trusted companies showcase.", IsActive: true},
	}
	for _, item := range partners {
		partner := item
		if partner.Status == "" {
			partner.Status = "active"
		}
		if partner.PartnerType == "" {
			partner.PartnerType = "university"
		}
		if err := db.WithContext(ctx).Where(models.Partner{Name: partner.Name}).Assign(map[string]any{
			"partner_type": partner.PartnerType,
			"status":       partner.Status,
			"is_active":    partner.IsActive,
		}).Attrs(partner).FirstOrCreate(&partner).Error; err != nil {
			return fmt.Errorf("seed partner %s: %w", item.Name, err)
		}
	}

	event := models.Event{
		TitleEn:       "Scholarship Readiness Webinar",
		TitleID:       "Webinar Kesiapan Beasiswa",
		DescriptionEn: "A live session on IELTS strategy, essay positioning, and university fit.",
		DescriptionID: "Sesi live tentang strategi IELTS, positioning esai, dan kecocokan universitas.",
		StartDate:     time.Now().AddDate(0, 0, 14),
		EndDate:       time.Now().AddDate(0, 0, 14).Add(2 * time.Hour),
		IsActive:      true,
	}
	if err := db.WithContext(ctx).Where(models.Event{TitleEn: event.TitleEn}).Attrs(event).FirstOrCreate(&event).Error; err != nil {
		return fmt.Errorf("seed event: %w", err)
	}

	return nil
}

func seedDevelopmentRelations(ctx context.Context, db *gorm.DB) error {
	var admin, student models.User
	if err := db.WithContext(ctx).Where(models.User{Email: "admin@academyprometheus.com"}).First(&admin).Error; err != nil {
		return fmt.Errorf("seed relation admin lookup: %w", err)
	}
	if err := db.WithContext(ctx).Where(models.User{Email: "nadia@example.com"}).First(&student).Error; err != nil {
		return fmt.Errorf("seed relation student lookup: %w", err)
	}

	profile := models.UserProfile{UserID: student.ID, BioEn: "Scholarship applicant preparing for Europe study pathways.", BioID: "Pendaftar beasiswa yang menyiapkan jalur studi ke Eropa.", LinkedinURL: "https://linkedin.com", PortfolioURL: "https://example.com", Skills: "IELTS, UX Research, Product Design"}
	if err := db.WithContext(ctx).Where(models.UserProfile{UserID: student.ID}).Attrs(profile).FirstOrCreate(&profile).Error; err != nil {
		return fmt.Errorf("seed user profile: %w", err)
	}

	var course models.Course
	if err := db.WithContext(ctx).Where(models.Course{Slug: "ui-ux-design-masterclass"}).First(&course).Error; err != nil {
		return fmt.Errorf("seed relation course lookup: %w", err)
	}
	var module models.CourseModule
	if err := db.WithContext(ctx).Where(models.CourseModule{CourseID: course.ID, TitleEn: "Foundation"}).First(&module).Error; err != nil {
		return fmt.Errorf("seed relation module lookup: %w", err)
	}
	var topic models.Topic
	if err := db.WithContext(ctx).Where(models.Topic{ModuleID: module.ID, TitleEn: "Orientation and roadmap"}).First(&topic).Error; err != nil {
		return fmt.Errorf("seed relation topic lookup: %w", err)
	}

	attachment := models.TopicAttachment{TopicID: topic.ID, FilePath: "/uploads/courses/files/ui-ux-roadmap.pdf", FileType: "application/pdf", NameEn: "UI/UX roadmap", NameID: "Roadmap UI/UX"}
	if err := db.WithContext(ctx).Where(models.TopicAttachment{TopicID: topic.ID, NameEn: attachment.NameEn}).Attrs(attachment).FirstOrCreate(&attachment).Error; err != nil {
		return fmt.Errorf("seed topic attachment: %w", err)
	}

	enrollment := models.CourseEnrollment{UserID: student.ID, CourseID: course.ID, EnrolledAt: time.Now().AddDate(0, -1, 0)}
	if err := db.WithContext(ctx).Where(models.CourseEnrollment{UserID: student.ID, CourseID: course.ID}).Attrs(enrollment).FirstOrCreate(&enrollment).Error; err != nil {
		return fmt.Errorf("seed course enrollment: %w", err)
	}

	completedAt := time.Now().AddDate(0, 0, -3)
	progress := models.TopicProgress{UserID: student.ID, TopicID: topic.ID, CompletedAt: &completedAt, VideoWatched: true}
	if err := db.WithContext(ctx).Where(models.TopicProgress{UserID: student.ID, TopicID: topic.ID}).Attrs(progress).FirstOrCreate(&progress).Error; err != nil {
		return fmt.Errorf("seed topic progress: %w", err)
	}

	assignment := models.Assignment{TopicID: topic.ID, TitleEn: "Portfolio brief", TitleID: "Brief portofolio", DescriptionEn: "Create a concise product problem statement.", DescriptionID: "Buat problem statement produk yang ringkas."}
	if err := db.WithContext(ctx).Where(models.Assignment{TopicID: topic.ID, TitleEn: assignment.TitleEn}).Attrs(assignment).FirstOrCreate(&assignment).Error; err != nil {
		return fmt.Errorf("seed assignment: %w", err)
	}

	submission := models.AssignmentSubmission{AssignmentID: assignment.ID, UserID: student.ID, FilePath: "/uploads/submissions/portfolio-brief.pdf", Score: 88, Feedback: "Clear structure and practical scope.", SubmittedAt: &completedAt}
	if err := db.WithContext(ctx).Where(models.AssignmentSubmission{AssignmentID: assignment.ID, UserID: student.ID}).Attrs(submission).FirstOrCreate(&submission).Error; err != nil {
		return fmt.Errorf("seed assignment submission: %w", err)
	}

	certificate := models.Certificate{UserID: student.ID, CourseID: course.ID, IssuedAt: time.Now(), CertificateURL: ""}
	if err := db.WithContext(ctx).Where(models.Certificate{UserID: student.ID, CourseID: course.ID}).Attrs(certificate).FirstOrCreate(&certificate).Error; err != nil {
		return fmt.Errorf("seed certificate: %w", err)
	}
	if err := services.EnsureCertificateUUID(ctx, db, &certificate); err != nil {
		return fmt.Errorf("seed certificate uuid: %w", err)
	}
	if certificate.CertificateURL == "" || strings.HasPrefix(certificate.CertificateURL, "/uploads/certificates/") || strings.HasPrefix(certificate.CertificateURL, "/api/certificates/") {
		protectedURL := services.CertificateDownloadURL(certificate)
		if err := db.WithContext(ctx).Model(&certificate).Update("certificate_url", protectedURL).Error; err != nil {
			return fmt.Errorf("seed certificate url: %w", err)
		}
		certificate.CertificateURL = protectedURL
	}
	if err := ensureSeedCertificatePDF(services.CertificateFilePublicPath(certificate)); err != nil {
		return err
	}

	drip := models.DripSchedule{CourseID: course.ID, ModuleID: module.ID, AvailableAfterDays: 0}
	if err := db.WithContext(ctx).Where(models.DripSchedule{CourseID: course.ID, ModuleID: module.ID}).Attrs(drip).FirstOrCreate(&drip).Error; err != nil {
		return fmt.Errorf("seed drip schedule: %w", err)
	}

	review := models.Review{UserID: student.ID, ReviewableID: course.ID, ReviewableType: "course", Rating: 5, Comment: "Practical lessons with clear portfolio outcomes."}
	if err := db.WithContext(ctx).Where(models.Review{UserID: student.ID, ReviewableID: course.ID, ReviewableType: "course"}).Attrs(review).FirstOrCreate(&review).Error; err != nil {
		return fmt.Errorf("seed review: %w", err)
	}

	if err := seedQuizRelations(ctx, db, module.ID, student.ID); err != nil {
		return err
	}
	if err := seedCommerceRelations(ctx, db, student.ID); err != nil {
		return err
	}
	if err := seedLeadRelations(ctx, db, admin.ID); err != nil {
		return err
	}
	if err := seedApplications(ctx, db); err != nil {
		return err
	}

	for _, item := range defaultSEOMetaRows() {
		seo := item
		if err := db.WithContext(ctx).Where(models.SEOMeta{PageSlug: seo.PageSlug}).Attrs(seo).FirstOrCreate(&seo).Error; err != nil {
			return fmt.Errorf("seed seo meta %s: %w", seo.PageSlug, err)
		}
	}

	notification := models.Notification{UserID: student.ID, TitleEn: "Course unlocked", TitleID: "Kursus terbuka", MessageEn: "Your UI/UX Design Masterclass is ready.", MessageID: "Masterclass Desain UI/UX kamu sudah siap.", Type: "course", Link: "/dashboard/courses"}
	if err := db.WithContext(ctx).Where(models.Notification{UserID: student.ID, TitleEn: notification.TitleEn}).Attrs(notification).FirstOrCreate(&notification).Error; err != nil {
		return fmt.Errorf("seed notification: %w", err)
	}

	return nil
}

func seedQuizRelations(ctx context.Context, db *gorm.DB, moduleID, studentID uint) error {
	quiz := models.Quiz{ModuleID: moduleID, TitleEn: "Foundation quiz", TitleID: "Kuis fondasi", PassingScore: 70, AttemptLimit: 3}
	if err := db.WithContext(ctx).Where(models.Quiz{ModuleID: moduleID, TitleEn: quiz.TitleEn}).Attrs(quiz).FirstOrCreate(&quiz).Error; err != nil {
		return fmt.Errorf("seed quiz: %w", err)
	}

	question := models.QuizQuestion{QuizID: quiz.ID, Type: "multiple_choice", QuestionEn: "What should a portfolio case study explain first?", QuestionID: "Apa yang harus dijelaskan pertama dalam studi kasus portofolio?", Order: 1}
	if err := db.WithContext(ctx).Where(models.QuizQuestion{QuizID: quiz.ID, QuestionEn: question.QuestionEn}).Attrs(question).FirstOrCreate(&question).Error; err != nil {
		return fmt.Errorf("seed quiz question: %w", err)
	}

	correct := models.QuizAnswer{QuestionID: question.ID, AnswerEn: "The problem and user context", AnswerID: "Masalah dan konteks user", IsCorrect: true, Order: 1}
	if err := db.WithContext(ctx).Where(models.QuizAnswer{QuestionID: question.ID, AnswerEn: correct.AnswerEn}).Attrs(correct).FirstOrCreate(&correct).Error; err != nil {
		return fmt.Errorf("seed quiz answer correct: %w", err)
	}
	incorrect := models.QuizAnswer{QuestionID: question.ID, AnswerEn: "Only the final UI screens", AnswerID: "Hanya tampilan UI akhir", IsCorrect: false, Order: 2}
	if err := db.WithContext(ctx).Where(models.QuizAnswer{QuestionID: question.ID, AnswerEn: incorrect.AnswerEn}).Attrs(incorrect).FirstOrCreate(&incorrect).Error; err != nil {
		return fmt.Errorf("seed quiz answer incorrect: %w", err)
	}

	submission := models.QuizSubmission{QuizID: quiz.ID, UserID: studentID, Score: 100, Passed: true, AttemptNumber: 1, SubmittedAt: time.Now()}
	if err := db.WithContext(ctx).Where(models.QuizSubmission{QuizID: quiz.ID, UserID: studentID, AttemptNumber: 1}).Attrs(submission).FirstOrCreate(&submission).Error; err != nil {
		return fmt.Errorf("seed quiz submission: %w", err)
	}

	submissionAnswer := models.QuizSubmissionAnswer{SubmissionID: submission.ID, QuestionID: question.ID, AnswerID: correct.ID}
	if err := db.WithContext(ctx).Where(models.QuizSubmissionAnswer{SubmissionID: submission.ID, QuestionID: question.ID}).Attrs(submissionAnswer).FirstOrCreate(&submissionAnswer).Error; err != nil {
		return fmt.Errorf("seed quiz submission answer: %w", err)
	}

	return nil
}

func defaultSEOMetaRows() []models.SEOMeta {
	return []models.SEOMeta{
		{PageSlug: "about", TitleEn: "About", TitleID: "Tentang", DescriptionEn: "Learn about Prometheus Academy, our mission, and the Europe x Asia education bridge.", DescriptionID: "Kenali Prometheus Academy, misi kami, dan jembatan edukasi Eropa x Asia."},
		{PageSlug: "become-a-partner", TitleEn: "Become a Partner", TitleID: "Jadi Mitra", DescriptionEn: "Partner with Prometheus Academy for university programs and global academic collaboration.", DescriptionID: "Bermitra dengan Prometheus Academy untuk program universitas dan kolaborasi akademik global."},
		{PageSlug: "contact", TitleEn: "Contact", TitleID: "Kontak", DescriptionEn: "Contact Prometheus Academy for courses, services, Talent Bridge, and partnership inquiries.", DescriptionID: "Hubungi Prometheus Academy untuk kursus, layanan, Talent Bridge, dan kemitraan."},
		{PageSlug: "courses", TitleEn: "Courses", TitleID: "Kursus", DescriptionEn: "Browse Prometheus Academy online courses in UI/UX, digital marketing, financial literacy, AI, and career preparation.", DescriptionID: "Jelajahi kursus online Prometheus Academy di UI/UX, digital marketing, literasi finansial, AI, dan persiapan karier."},
		{PageSlug: "home", TitleEn: "Prometheus Academy - Europe Asia Learning Bridge", TitleID: "Prometheus Academy - Jembatan Belajar Eropa Asia", DescriptionEn: "Courses, digital products, Talent Bridge, and university partnerships across Europe and Asia.", DescriptionID: "Kursus, produk digital, Talent Bridge, dan partner universitas di Eropa dan Asia.", OGImage: "/uploads/seo/home-og.webp"},
		{PageSlug: "privacy-policy", TitleEn: "Privacy Policy", TitleID: "Kebijakan Privasi", DescriptionEn: "How Prometheus Academy handles personal data.", DescriptionID: "Cara Prometheus Academy mengelola data pribadi."},
		{PageSlug: "services", TitleEn: "Services", TitleID: "Layanan", DescriptionEn: "Explore Prometheus Academy digital products, scholarship blueprints, e-books, and consultation services.", DescriptionID: "Jelajahi produk digital, blueprint beasiswa, e-book, dan layanan konsultasi Prometheus Academy."},
		{PageSlug: "talent-bridge", TitleEn: "Talent Bridge", TitleID: "Jembatan Talenta", DescriptionEn: "Managed staffing services connecting Asia-based talent with European companies.", DescriptionID: "Layanan staffing terkelola yang menghubungkan talenta Asia dengan perusahaan Eropa."},
		{PageSlug: "terms", TitleEn: "Terms of Service", TitleID: "Syarat Layanan", DescriptionEn: "Terms for using Prometheus Academy services, courses, and digital products.", DescriptionID: "Syarat penggunaan layanan, kursus, dan produk digital Prometheus Academy."},
	}
}

func seedCommerceRelations(ctx context.Context, db *gorm.DB, studentID uint) error {
	var product models.Product
	if err := db.WithContext(ctx).Where(models.Product{Slug: "toefl-ielts-mastery-ebook"}).First(&product).Error; err != nil {
		return fmt.Errorf("seed commerce product lookup: %w", err)
	}
	var order models.Order
	if err := db.WithContext(ctx).Where(models.Order{MidtransOrderID: "SEED-ORDER-001"}).First(&order).Error; err != nil {
		return fmt.Errorf("seed commerce order lookup: %w", err)
	}

	item := models.OrderItem{OrderID: order.ID, ItemType: "product", ItemID: product.ID, Price: product.Price}
	if err := db.WithContext(ctx).Where(models.OrderItem{OrderID: order.ID, ItemType: "product", ItemID: product.ID}).Attrs(item).FirstOrCreate(&item).Error; err != nil {
		return fmt.Errorf("seed order item: %w", err)
	}

	transaction := models.Transaction{OrderID: order.ID, MidtransTransactionID: "SEED-TRX-001", PaymentType: "bank_transfer", Status: "pending", RawResponse: "{}"}
	if err := db.WithContext(ctx).Where(models.Transaction{MidtransTransactionID: transaction.MidtransTransactionID}).Attrs(transaction).FirstOrCreate(&transaction).Error; err != nil {
		return fmt.Errorf("seed transaction: %w", err)
	}

	invoice := models.Invoice{OrderID: order.ID, InvoiceNumber: "INV-SEED-001", FilePath: "/uploads/invoices/inv-seed-001.pdf", IssuedAt: time.Now()}
	if err := db.WithContext(ctx).Where(models.Invoice{OrderID: order.ID}).Attrs(invoice).FirstOrCreate(&invoice).Error; err != nil {
		return fmt.Errorf("seed invoice: %w", err)
	}

	var slot models.ConsultationSlot
	if err := db.WithContext(ctx).Where("time_start = ?", "10:00").First(&slot).Error; err != nil {
		return fmt.Errorf("seed slot lookup: %w", err)
	}
	booking := models.ConsultationBooking{SlotID: slot.ID, UserID: studentID, OrderID: order.ID, Status: "pending", Notes: "Review scholarship timeline and IELTS target."}
	if err := db.WithContext(ctx).Where(models.ConsultationBooking{SlotID: slot.ID, UserID: studentID, OrderID: order.ID}).Attrs(booking).FirstOrCreate(&booking).Error; err != nil {
		return fmt.Errorf("seed consultation booking: %w", err)
	}

	return nil
}

func seedLeadRelations(ctx context.Context, db *gorm.DB, adminID uint) error {
	lead := models.ContactLead{Name: "Sarah Tan", Email: "sarah@example.com", Subject: "Partnership inquiry", Message: "We want to explore a university partnership.", GDPRConsent: true, Status: "new"}
	if err := db.WithContext(ctx).Where(models.ContactLead{Email: lead.Email, Subject: lead.Subject}).Attrs(lead).FirstOrCreate(&lead).Error; err != nil {
		return fmt.Errorf("seed contact lead: %w", err)
	}

	note := models.LeadNote{LeadID: lead.ID, LeadType: "contact", Note: "Follow up with partner program deck.", CreatedBy: adminID}
	if err := db.WithContext(ctx).Where(models.LeadNote{LeadID: lead.ID, LeadType: note.LeadType}).Attrs(note).FirstOrCreate(&note).Error; err != nil {
		return fmt.Errorf("seed lead note: %w", err)
	}

	subscriber := models.NewsletterSubscriber{FullName: "Nadia Putri", Email: "newsletter-nadia@example.com", GDPRConsent: true, SubscribedAt: time.Now()}
	if err := db.WithContext(ctx).Where(models.NewsletterSubscriber{Email: subscriber.Email}).Attrs(subscriber).FirstOrCreate(&subscriber).Error; err != nil {
		return fmt.Errorf("seed newsletter subscriber: %w", err)
	}

	return nil
}

func seedApplications(ctx context.Context, db *gorm.DB) error {
	var job models.TalentJob
	if err := db.WithContext(ctx).Where(models.TalentJob{Slug: "remote-ui-designer"}).First(&job).Error; err != nil {
		return fmt.Errorf("seed applications job lookup: %w", err)
	}

	jobApplication := models.TalentJobApplication{JobID: job.ID, Name: "Raka Wibowo", Email: "raka@example.com", CVPath: "/uploads/cv/raka-wibowo.pdf", Status: "new", AppliedAt: time.Now()}
	if err := db.WithContext(ctx).Where(models.TalentJobApplication{JobID: job.ID, Email: jobApplication.Email}).Attrs(jobApplication).FirstOrCreate(&jobApplication).Error; err != nil {
		return fmt.Errorf("seed job application: %w", err)
	}

	hiringInquiry := models.HiringInquiry{FirstName: "Marta", LastName: "Schneider", WorkEmail: "marta@berlinsaas.example", CompanyName: "Berlin SaaS Studio", CompanySize: "11-50", RolesNeeded: "Product designer and growth marketer", Headcount: 2, Challenge: "Need reliable remote hiring pipeline.", GDPRConsent: true, Status: "new"}
	if err := db.WithContext(ctx).Where(models.HiringInquiry{WorkEmail: hiringInquiry.WorkEmail}).Attrs(hiringInquiry).FirstOrCreate(&hiringInquiry).Error; err != nil {
		return fmt.Errorf("seed hiring inquiry: %w", err)
	}

	talentPlus := models.TalentPlusApplication{FirstName: "Nadia", LastName: "Putri", Email: "talentplus-nadia@example.com", Phone: "+62 812 1111 2222", Country: "Indonesia", CurrentStatus: "Student", JobField: "Product Design", ProgrammeInterest: "Talent Bridge+", TargetCountries: "Germany, Netherlands", CareerGoals: "Build a remote product career in Europe.", GDPRConsent: true, Status: "new"}
	if err := db.WithContext(ctx).Where(models.TalentPlusApplication{Email: talentPlus.Email}).Attrs(talentPlus).FirstOrCreate(&talentPlus).Error; err != nil {
		return fmt.Errorf("seed talent plus application: %w", err)
	}

	partnerApplication := models.PartnerApplication{UniversityName: "Jakarta Tech Institute", Country: "Indonesia", ContactPerson: "Sarah Tan", RolePosition: "International Office", Email: "partner@jti.example", Phone: "+62 812 3333 4444", CurrentQSRanking: "Top 1000", PartnershipGoals: "Improve global exposure and student employability.", Status: "new"}
	if err := db.WithContext(ctx).Where(models.PartnerApplication{Email: partnerApplication.Email}).Attrs(partnerApplication).FirstOrCreate(&partnerApplication).Error; err != nil {
		return fmt.Errorf("seed partner application: %w", err)
	}

	return nil
}

func ensureSeedCertificatePDF(publicPath string) error {
	storageRoot := os.Getenv("STORAGE_PATH")
	if storageRoot == "" {
		storageRoot = "storage"
	}
	relativePath := strings.TrimPrefix(publicPath, "/")
	fullPath := filepath.Join(storageRoot, relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("seed certificate directory: %w", err)
	}
	body := seedCertificatePDFBody()
	if err := os.WriteFile(fullPath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("seed certificate file: %w", err)
	}
	return nil
}

func seedCertificatePDFBody() string {
	content := "BT\n/F1 32 Tf\n88 410 Td\n(Prometheus Academy) Tj\n0 -50 Td\n/F1 22 Tf\n(Certificate of Completion) Tj\n0 -40 Td\n/F1 16 Tf\n(Awarded to Nadia Putri) Tj\n0 -28 Td\n(UI UX Portfolio Sprint) Tj\nET\n"
	stream := fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 842 595] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj\n",
		stream,
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
	}
	pdf := "%PDF-1.4\n"
	offsets := []int{0}
	for _, obj := range objects {
		offsets = append(offsets, len(pdf))
		pdf += obj
	}
	xrefOffset := len(pdf)
	pdf += fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i < len(offsets); i++ {
		pdf += fmt.Sprintf("%010d 00000 n \n", offsets[i])
	}
	pdf += fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return pdf
}
