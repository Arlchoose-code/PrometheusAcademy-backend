package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

var slugUnsafePattern = regexp.MustCompile(`[^a-z0-9]+`)

func GenerateSlug(title string) string {
	normalized := norm.NFD.String(strings.ToLower(strings.TrimSpace(title)))
	var builder strings.Builder

	for _, r := range normalized {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		builder.WriteRune(r)
	}

	slug := slugUnsafePattern.ReplaceAllString(builder.String(), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "untitled"
	}
	return slug
}

func UniqueSlug(ctx context.Context, db *gorm.DB, tableName string, title string, ignoreID uint) (string, error) {
	if db == nil {
		return GenerateSlug(title), nil
	}

	baseSlug := GenerateSlug(title)
	candidate := baseSlug

	for suffix := 2; ; suffix++ {
		var count int64
		query := db.WithContext(ctx).Table(tableName).Where("slug = ?", candidate)
		if ignoreID > 0 {
			query = query.Where("id <> ?", ignoreID)
		}
		if err := query.Count(&count).Error; err != nil {
			return "", fmt.Errorf("unique slug %s: %w", tableName, err)
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", baseSlug, suffix)
	}
}
