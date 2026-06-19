package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

const (
	XPEventCourseEnrolled         = "course_enrolled"
	XPEventLessonCompleted        = "lesson_completed"
	XPEventCourseCompleted        = "course_completed"
	XPEventEventAttended          = "event_attended"
	XPEventDiscussionParticipated = "discussion_participated"
	XPEventMaterialDownloaded     = "material_downloaded"

	XPCourseEnrolled     = 50
	XPLessonCompleted    = 25
	XPCourseCompleted    = 250
	XPEventAttendance    = 100
	XPDiscussion         = 10
	XPMaterialDownloaded = 15
	levelSize            = 500
)

type GamificationSummary struct {
	TotalXP         int                       `json:"total_xp"`
	Level           int                       `json:"level"`
	CurrentLevelXP  int                       `json:"current_level_xp"`
	NextLevelXP     int                       `json:"next_level_xp"`
	ProgressPercent int                       `json:"progress_percent"`
	RecentXP        []GamificationLedgerItem  `json:"recent_xp"`
	Achievements    []GamificationAchievement `json:"achievements"`
}

type GamificationLedgerItem struct {
	EventType     string    `json:"event_type"`
	Points        int       `json:"points"`
	DescriptionEn string    `json:"description_en"`
	DescriptionID string    `json:"description_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type GamificationAchievement struct {
	Code          string    `json:"code"`
	NameEn        string    `json:"name_en"`
	NameID        string    `json:"name_id"`
	DescriptionEn string    `json:"description_en"`
	DescriptionID string    `json:"description_id"`
	Icon          string    `json:"icon"`
	AwardedAt     time.Time `json:"awarded_at"`
}

func AwardXP(ctx context.Context, db *gorm.DB, userID uint, eventType, referenceType string, referenceID uint, points int, descriptionEn, descriptionID string) error {
	if db == nil || userID == 0 || points <= 0 || eventType == "" {
		return nil
	}
	entry := models.XPLedger{
		UserID:        userID,
		EventType:     eventType,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Points:        points,
		DescriptionEn: descriptionEn,
		DescriptionID: descriptionID,
	}
	result := db.WithContext(ctx).
		Where(models.XPLedger{UserID: userID, EventType: eventType, ReferenceType: referenceType, ReferenceID: referenceID}).
		Attrs(entry).
		FirstOrCreate(&entry)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return SyncUserAchievements(ctx, db, userID)
}

func GamificationForUser(ctx context.Context, db *gorm.DB, userID uint) (GamificationSummary, error) {
	summary := levelSummary(0)
	if db == nil || userID == 0 {
		return summary, nil
	}

	if err := db.WithContext(ctx).Model(&models.XPLedger{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(points), 0)").
		Scan(&summary.TotalXP).Error; err != nil {
		return summary, fmt.Errorf("gamification total xp: %w", err)
	}
	summary = levelSummary(summary.TotalXP)

	var ledger []models.XPLedger
	if err := db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(5).
		Find(&ledger).Error; err != nil {
		return summary, fmt.Errorf("gamification recent xp: %w", err)
	}
	summary.RecentXP = make([]GamificationLedgerItem, 0, len(ledger))
	for _, item := range ledger {
		summary.RecentXP = append(summary.RecentXP, GamificationLedgerItem{
			EventType:     item.EventType,
			Points:        item.Points,
			DescriptionEn: item.DescriptionEn,
			DescriptionID: item.DescriptionID,
			CreatedAt:     item.CreatedAt,
		})
	}

	var achievements []models.UserAchievement
	if err := db.WithContext(ctx).
		Preload("Achievement").
		Where("user_id = ?", userID).
		Order("awarded_at desc").
		Limit(8).
		Find(&achievements).Error; err != nil {
		return summary, fmt.Errorf("gamification achievements: %w", err)
	}
	summary.Achievements = make([]GamificationAchievement, 0, len(achievements))
	for _, item := range achievements {
		summary.Achievements = append(summary.Achievements, GamificationAchievement{
			Code:          item.Achievement.Code,
			NameEn:        item.Achievement.NameEn,
			NameID:        item.Achievement.NameID,
			DescriptionEn: item.Achievement.DescriptionEn,
			DescriptionID: item.Achievement.DescriptionID,
			Icon:          item.Achievement.Icon,
			AwardedAt:     item.AwardedAt,
		})
	}
	return summary, nil
}

func SyncUserAchievements(ctx context.Context, db *gorm.DB, userID uint) error {
	if db == nil || userID == 0 {
		return nil
	}
	var achievements []models.Achievement
	if err := db.WithContext(ctx).Where("is_active = ?", true).Find(&achievements).Error; err != nil {
		return err
	}
	var totalXP int
	if err := db.WithContext(ctx).Model(&models.XPLedger{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(points), 0)").
		Scan(&totalXP).Error; err != nil {
		return err
	}
	for _, achievement := range achievements {
		unlocked := achievement.RequiredXP > 0 && totalXP >= achievement.RequiredXP
		if !unlocked && achievement.EventType != "" && achievement.Threshold > 0 {
			var count int64
			if err := db.WithContext(ctx).Model(&models.XPLedger{}).
				Where("user_id = ? AND event_type = ?", userID, achievement.EventType).
				Count(&count).Error; err != nil {
				return err
			}
			unlocked = count >= int64(achievement.Threshold)
		}
		if !unlocked {
			continue
		}
		row := models.UserAchievement{UserID: userID, AchievementID: achievement.ID, AwardedAt: time.Now().UTC()}
		if err := db.WithContext(ctx).
			Where(models.UserAchievement{UserID: userID, AchievementID: achievement.ID}).
			Attrs(row).
			FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func SeedGamificationAchievements(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	for _, item := range defaultGamificationAchievements() {
		achievement := item
		if err := db.WithContext(ctx).
			Where(models.Achievement{Code: achievement.Code}).
			Assign(achievement).
			FirstOrCreate(&achievement).Error; err != nil {
			return fmt.Errorf("seed achievement %s: %w", item.Code, err)
		}
	}
	return nil
}

func levelSummary(totalXP int) GamificationSummary {
	level := totalXP/levelSize + 1
	current := totalXP % levelSize
	progress := int(math.Round(float64(current) / float64(levelSize) * 100))
	return GamificationSummary{
		TotalXP:         totalXP,
		Level:           level,
		CurrentLevelXP:  current,
		NextLevelXP:     levelSize,
		ProgressPercent: progress,
		RecentXP:        []GamificationLedgerItem{},
		Achievements:    []GamificationAchievement{},
	}
}

func defaultGamificationAchievements() []models.Achievement {
	return []models.Achievement{
		{Code: "first_steps", NameEn: "First Steps", NameID: "Langkah Pertama", DescriptionEn: "Earn your first 50 XP.", DescriptionID: "Kumpulkan 50 XP pertama.", Icon: "sparkles", RequiredXP: 50, IsActive: true},
		{Code: "course_finisher", NameEn: "Course Finisher", NameID: "Penuntas Course", DescriptionEn: "Complete one course.", DescriptionID: "Selesaikan satu course.", Icon: "trophy", EventType: XPEventCourseCompleted, Threshold: 1, IsActive: true},
		{Code: "event_goer", NameEn: "Event Goer", NameID: "Peserta Event", DescriptionEn: "Attend one Prometheus event.", DescriptionID: "Hadir di satu event Prometheus.", Icon: "calendar", EventType: XPEventEventAttended, Threshold: 1, IsActive: true},
		{Code: "discussion_starter", NameEn: "Discussion Starter", NameID: "Pembuka Diskusi", DescriptionEn: "Participate in a course discussion.", DescriptionID: "Ikut dalam diskusi course.", Icon: "messages", EventType: XPEventDiscussionParticipated, Threshold: 1, IsActive: true},
		{Code: "resource_collector", NameEn: "Resource Collector", NameID: "Pengumpul Resource", DescriptionEn: "Download three learning materials.", DescriptionID: "Download tiga materi belajar.", Icon: "download", EventType: XPEventMaterialDownloaded, Threshold: 3, IsActive: true},
		{Code: "level_five", NameEn: "Level 5 Learner", NameID: "Learner Level 5", DescriptionEn: "Reach level 5 through consistent learning.", DescriptionID: "Capai level 5 lewat belajar konsisten.", Icon: "award", RequiredXP: 2000, IsActive: true},
	}
}
