package services

import (
	"context"
	"fmt"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

type PublicSettings struct {
	SiteName      string `json:"site_name"`
	CopyrightText string `json:"copyright_text"`
	LogoPath      string `json:"logo_path"`
	FaviconPath   string `json:"favicon_path"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	FacebookURL   string `json:"facebook_url"`
	InstagramURL  string `json:"instagram_url"`
	TiktokURL     string `json:"tiktok_url"`
}

type SettingsService interface {
	GetPublicSettings(ctx context.Context) (PublicSettings, error)
}

type settingsService struct {
	db *gorm.DB
}

func NewSettingsService(db *gorm.DB) SettingsService {
	return &settingsService{db: db}
}

func (s *settingsService) GetPublicSettings(ctx context.Context) (PublicSettings, error) {
	settings := defaultPublicSettings()
	if s.db == nil {
		return settings, nil
	}

	var rows []models.Setting
	if err := s.db.WithContext(ctx).Where("`key` IN ?", publicSettingKeys()).Find(&rows).Error; err != nil {
		return settings, fmt.Errorf("get public settings: %w", err)
	}

	for _, row := range rows {
		switch row.Key {
		case "site_name":
			settings.SiteName = fallback(row.Value, settings.SiteName)
		case "copyright_text":
			settings.CopyrightText = fallback(row.Value, settings.CopyrightText)
		case "logo_path":
			settings.LogoPath = row.Value
		case "favicon_path":
			settings.FaviconPath = row.Value
		case "phone":
			settings.Phone = fallback(row.Value, settings.Phone)
		case "email":
			settings.Email = fallback(row.Value, settings.Email)
		case "facebook_url":
			settings.FacebookURL = fallback(row.Value, settings.FacebookURL)
		case "instagram_url":
			settings.InstagramURL = fallback(row.Value, settings.InstagramURL)
		case "tiktok_url":
			settings.TiktokURL = fallback(row.Value, settings.TiktokURL)
		}
	}

	return settings, nil
}

func defaultPublicSettings() PublicSettings {
	return PublicSettings{
		SiteName:      "Prometheus Academy",
		CopyrightText: "© Academy Prometheus 2026 · All rights reserved",
		LogoPath:      "",
		FaviconPath:   "",
		Phone:         "+62 000 0000 0000",
		Email:         "hello@academyprometheus.com",
		FacebookURL:   "#",
		InstagramURL:  "#",
		TiktokURL:     "#",
	}
}

func publicSettingKeys() []string {
	return []string{
		"site_name",
		"copyright_text",
		"logo_path",
		"favicon_path",
		"phone",
		"email",
		"facebook_url",
		"instagram_url",
		"tiktok_url",
	}
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
