package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

type TalentReviewRecipient struct {
	Name   string
	Email  string
	Status string
}

func GenerateTalentReviewToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, HashTalentReviewToken(token), nil
}

func HashTalentReviewToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func TalentReviewRecipientForApplication(db *gorm.DB, applicationType string, applicationID uint) (TalentReviewRecipient, error) {
	switch strings.TrimSpace(applicationType) {
	case "job":
		var application models.TalentJobApplication
		if err := db.First(&application, applicationID).Error; err != nil {
			return TalentReviewRecipient{}, err
		}
		return TalentReviewRecipient{Name: application.Name, Email: application.Email, Status: application.Status}, nil
	case "plus":
		var application models.TalentPlusApplication
		if err := db.First(&application, applicationID).Error; err != nil {
			return TalentReviewRecipient{}, err
		}
		return TalentReviewRecipient{Name: strings.TrimSpace(application.FirstName + " " + application.LastName), Email: application.Email, Status: application.Status}, nil
	default:
		return TalentReviewRecipient{}, fmt.Errorf("unsupported application type")
	}
}

func TalentReviewStatusEligible(applicationType string, status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	switch strings.TrimSpace(applicationType) {
	case "job":
		return status == "hired" || status == "accepted" || status == "completed"
	case "plus":
		return status == "placed" || status == "completed"
	default:
		return false
	}
}
