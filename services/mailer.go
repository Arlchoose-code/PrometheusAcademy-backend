package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

const brevoSendEndpoint = "https://api.brevo.com/v3/smtp/email"
const brevoSendersEndpoint = "https://api.brevo.com/v3/senders"

type MailerSettings struct {
	Provider  string
	APIKey    string
	FromEmail string
	FromName  string
	ReplyTo   string
}

type MailMessage struct {
	ToEmail string
	ToName  string
	Subject string
	HTML    string
	Text    string
}

type BrevoSender struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Active bool   `json:"active"`
}

func LoadMailerSettings(ctx context.Context, db *gorm.DB) (MailerSettings, error) {
	settings := MailerSettings{
		Provider:  "brevo",
		FromEmail: "hello@academyprometheus.com",
		FromName:  "Prometheus Academy",
	}
	if db == nil {
		return settings, nil
	}

	var rows []models.Setting
	if err := db.WithContext(ctx).Where("`key` IN ?", []string{
		"mailer_provider",
		"brevo_api_key",
		"mailer_from_email",
		"mailer_from_name",
		"mailer_reply_to",
	}).Find(&rows).Error; err != nil {
		return settings, fmt.Errorf("load mailer settings: %w", err)
	}

	for _, row := range rows {
		value := strings.TrimSpace(row.Value)
		switch row.Key {
		case "mailer_provider":
			settings.Provider = fallback(value, settings.Provider)
		case "brevo_api_key":
			settings.APIKey = value
		case "mailer_from_email":
			settings.FromEmail = fallback(value, settings.FromEmail)
		case "mailer_from_name":
			settings.FromName = fallback(value, settings.FromName)
		case "mailer_reply_to":
			settings.ReplyTo = value
		}
	}

	return settings, nil
}

func SendBrevoEmail(ctx context.Context, settings MailerSettings, message MailMessage) (string, error) {
	if strings.ToLower(strings.TrimSpace(settings.Provider)) != "brevo" {
		return "", fmt.Errorf("Brevo mailer is not enabled")
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return "", fmt.Errorf("Brevo API key is not configured")
	}
	if strings.TrimSpace(settings.FromEmail) == "" {
		return "", fmt.Errorf("Mailer sender email is not configured")
	}
	if strings.TrimSpace(message.ToEmail) == "" {
		return "", fmt.Errorf("Recipient email is required")
	}
	if strings.TrimSpace(message.Subject) == "" {
		return "", fmt.Errorf("Subject is required")
	}

	payload := map[string]any{
		"sender": map[string]string{
			"name":  settings.FromName,
			"email": settings.FromEmail,
		},
		"to": []map[string]string{
			{
				"name":  message.ToName,
				"email": message.ToEmail,
			},
		},
		"subject": message.Subject,
	}
	if strings.TrimSpace(message.HTML) != "" {
		payload["htmlContent"] = message.HTML
	}
	if strings.TrimSpace(message.Text) != "" {
		payload["textContent"] = message.Text
	}
	if _, ok := payload["htmlContent"]; !ok {
		payload["htmlContent"] = "<p>" + message.Subject + "</p>"
	}
	if strings.TrimSpace(settings.ReplyTo) != "" {
		payload["replyTo"] = map[string]string{"email": settings.ReplyTo}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoSendEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("api-key", settings.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("Brevo request failed: %s", strings.TrimSpace(string(raw)))
	}

	var result struct {
		MessageID string `json:"messageId"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return result.MessageID, nil
}

func ListBrevoSenders(ctx context.Context, settings MailerSettings) ([]BrevoSender, error) {
	if err := ensureBrevoConfigured(settings); err != nil {
		return nil, err
	}
	raw, err := brevoRequest(ctx, http.MethodGet, brevoSendersEndpoint, settings.APIKey, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Senders []BrevoSender `json:"senders"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Senders, nil
}

func CreateBrevoSender(ctx context.Context, settings MailerSettings, name, email string) (map[string]any, error) {
	if err := ensureBrevoConfigured(settings); err != nil {
		return nil, err
	}
	return brevoSenderMutation(ctx, http.MethodPost, brevoSendersEndpoint, settings.APIKey, name, email)
}

func UpdateBrevoSender(ctx context.Context, settings MailerSettings, senderID string, name, email string) error {
	if err := ensureBrevoConfigured(settings); err != nil {
		return err
	}
	if strings.TrimSpace(senderID) == "" {
		return fmt.Errorf("Sender id is required")
	}
	_, err := brevoSenderMutation(ctx, http.MethodPut, brevoSendersEndpoint+"/"+strings.TrimSpace(senderID), settings.APIKey, name, email)
	return err
}

func DeleteBrevoSender(ctx context.Context, settings MailerSettings, senderID string) error {
	if err := ensureBrevoConfigured(settings); err != nil {
		return err
	}
	if strings.TrimSpace(senderID) == "" {
		return fmt.Errorf("Sender id is required")
	}
	_, err := brevoRequest(ctx, http.MethodDelete, brevoSendersEndpoint+"/"+strings.TrimSpace(senderID), settings.APIKey, nil)
	return err
}

func ensureBrevoConfigured(settings MailerSettings) error {
	if strings.ToLower(strings.TrimSpace(settings.Provider)) != "brevo" {
		return fmt.Errorf("Brevo mailer is not enabled")
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return fmt.Errorf("Brevo API key is not configured")
	}
	return nil
}

func brevoSenderMutation(ctx context.Context, method, endpoint, apiKey, name, email string) (map[string]any, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("Sender name is required")
	}
	if strings.TrimSpace(email) == "" {
		return nil, fmt.Errorf("Sender email is required")
	}
	payload := map[string]string{
		"name":  strings.TrimSpace(name),
		"email": strings.TrimSpace(email),
	}
	raw, err := brevoRequest(ctx, method, endpoint, apiKey, payload)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	if len(raw) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func brevoRequest(ctx context.Context, method, endpoint, apiKey string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("api-key", apiKey)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("Brevo request failed: %s", strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
