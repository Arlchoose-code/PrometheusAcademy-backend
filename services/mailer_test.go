package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMailerDeliveryConfigured(t *testing.T) {
	tests := []struct {
		name     string
		settings MailerSettings
		want     bool
	}{
		{name: "brevo configured", settings: MailerSettings{Provider: "brevo", BrevoAPIKey: "key"}, want: true},
		{name: "brevo missing key", settings: MailerSettings{Provider: "brevo"}, want: false},
		{name: "ghl configured", settings: MailerSettings{Provider: "gohighlevel", APIKey: "key", LocationID: "location"}, want: true},
		{name: "ghl missing location", settings: MailerSettings{Provider: "gohighlevel", APIKey: "key"}, want: false},
		{name: "unknown provider", settings: MailerSettings{Provider: "smtp", APIKey: "key"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MailerDeliveryConfigured(test.settings); got != test.want {
				t.Fatalf("MailerDeliveryConfigured() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDeleteBrevoSender(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", request.Method)
		}
		if request.URL.Path != "/senders/42" {
			t.Fatalf("path = %s, want /senders/42", request.URL.Path)
		}
		if request.Header.Get("api-key") != "secret" {
			t.Fatal("missing Brevo api-key header")
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := DeleteBrevoSender(context.Background(), MailerSettings{
		BrevoAPIKey:     "secret",
		BrevoAPIBaseURL: server.URL,
	}, "42")
	if err != nil {
		t.Fatalf("DeleteBrevoSender() error = %v", err)
	}
}
