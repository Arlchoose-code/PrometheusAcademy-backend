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

const defaultGHLAPIBaseURL = "https://services.leadconnectorhq.com"

type MailerSettings struct {
	Provider       string
	APIKey         string
	LocationID     string
	APIBaseURL     string
	FromEmail      string
	FromName       string
	ReplyTo        string
	NewsletterTag  string
	ContactLeadTag string
}

type MailMessage struct {
	ToEmail string
	ToName  string
	Subject string
	HTML    string
	Text    string
	Tags    []string
}

type GHLContact struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func LoadMailerSettings(ctx context.Context, db *gorm.DB) (MailerSettings, error) {
	settings := MailerSettings{
		Provider:       "gohighlevel",
		APIBaseURL:     defaultGHLAPIBaseURL,
		FromEmail:      "hello@academyprometheus.com",
		FromName:       "Prometheus Academy",
		NewsletterTag:  "prometheus-newsletter",
		ContactLeadTag: "prometheus-website-lead",
	}
	if db == nil {
		return settings, nil
	}

	var rows []models.Setting
	if err := db.WithContext(ctx).Where("`key` IN ?", []string{
		"mailer_provider",
		"ghl_access_token",
		"ghl_location_id",
		"ghl_api_base_url",
		"ghl_newsletter_tag",
		"ghl_contact_lead_tag",
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
		case "ghl_access_token":
			settings.APIKey = value
		case "ghl_location_id":
			settings.LocationID = value
		case "ghl_api_base_url":
			settings.APIBaseURL = fallback(strings.TrimRight(value, "/"), settings.APIBaseURL)
		case "ghl_newsletter_tag":
			settings.NewsletterTag = fallback(value, settings.NewsletterTag)
		case "ghl_contact_lead_tag":
			settings.ContactLeadTag = fallback(value, settings.ContactLeadTag)
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

// SyncGHLContact upserts a platform user or lead into the configured HighLevel Location.
// Tags are intentionally used as the hand-off contract for HighLevel workflows.
func SyncGHLContact(ctx context.Context, settings MailerSettings, name, email string, tags []string) (GHLContact, error) {
	if err := ensureGHLConfigured(settings); err != nil {
		return GHLContact{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return GHLContact{}, fmt.Errorf("contact email is required")
	}
	firstName, lastName := splitContactName(name)
	payload := map[string]any{
		"locationId": settings.LocationID,
		"email":      email,
		"name":       strings.TrimSpace(name),
		"firstName":  firstName,
		"lastName":   lastName,
		"source":     "Prometheus Academy website",
	}
	if cleanTags := uniqueNonEmpty(tags); len(cleanTags) > 0 {
		payload["tags"] = cleanTags
	}
	raw, err := ghlRequest(ctx, settings, http.MethodPost, "/contacts/upsert", payload)
	if err != nil {
		return GHLContact{}, err
	}
	var result struct {
		Contact GHLContact `json:"contact"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return GHLContact{}, fmt.Errorf("decode GoHighLevel contact: %w", err)
	}
	if strings.TrimSpace(result.Contact.ID) == "" {
		return GHLContact{}, fmt.Errorf("GoHighLevel contact response did not contain an id")
	}
	return result.Contact, nil
}

// SendMailerEmail sends transactional email through HighLevel Conversations.
// The contact is upserted first so all transactional recipients also belong to the GHL contact list.
func SendMailerEmail(ctx context.Context, settings MailerSettings, message MailMessage) (string, error) {
	if strings.TrimSpace(message.Subject) == "" {
		return "", fmt.Errorf("subject is required")
	}
	contact, err := SyncGHLContact(ctx, settings, message.ToName, message.ToEmail, message.Tags)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"type":      "Email",
		"contactId": contact.ID,
		"subject":   strings.TrimSpace(message.Subject),
		"html":      message.HTML,
		"message":   fallback(strings.TrimSpace(message.Text), stripHTMLForEmail(message.HTML)),
	}
	if strings.TrimSpace(settings.FromEmail) != "" {
		from := strings.TrimSpace(settings.FromEmail)
		if strings.TrimSpace(settings.FromName) != "" {
			from = fmt.Sprintf("%s <%s>", strings.TrimSpace(settings.FromName), from)
		}
		payload["emailFrom"] = from
	}
	raw, err := ghlRequest(ctx, settings, http.MethodPost, "/conversations/messages", payload)
	if err != nil {
		return "", err
	}
	var result struct {
		MessageID string `json:"messageId"`
		ID        string `json:"id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode GoHighLevel message: %w", err)
	}
	return fallback(result.MessageID, result.ID), nil
}

func ensureGHLConfigured(settings MailerSettings) error {
	if strings.ToLower(strings.TrimSpace(settings.Provider)) != "gohighlevel" {
		return fmt.Errorf("GoHighLevel mailer is not enabled")
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return fmt.Errorf("GoHighLevel access token is not configured")
	}
	if strings.TrimSpace(settings.LocationID) == "" {
		return fmt.Errorf("GoHighLevel Location ID is not configured")
	}
	return nil
}

func ghlRequest(ctx context.Context, settings MailerSettings, method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	baseURL := fallback(strings.TrimRight(strings.TrimSpace(settings.APIBaseURL), "/"), defaultGHLAPIBaseURL)
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(settings.APIKey))
	request.Header.Set("Version", "2021-07-28")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GoHighLevel request failed: %w", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("GoHighLevel request failed (%d): %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func splitContactName(name string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
