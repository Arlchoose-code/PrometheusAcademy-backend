package services

import "testing"

func TestReplaceMailerLayoutTokensResolvesNestedContent(t *testing.T) {
	rendered := replaceMailerLayoutTokens("<main>{content}</main>", map[string]string{
		"content":       "<p>Hi {name}</p><a href=\"{dashboard_url}\">Open</a>",
		"name":          "Syahril Haryono",
		"dashboard_url": "http://localhost:3000/en/dashboard",
	})
	want := `<main><p>Hi Syahril Haryono</p><a href="http://localhost:3000/en/dashboard">Open</a></main>`
	if rendered != want {
		t.Fatalf("nested tokens were not resolved: got %q want %q", rendered, want)
	}
}
