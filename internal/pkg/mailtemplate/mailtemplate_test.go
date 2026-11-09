package mailtemplate

import (
	"strings"
	"testing"
)

func TestRenderAllTemplates(t *testing.T) {
	for _, kind := range []string{"invite", "verify", "reset"} {
		for _, locale := range []string{"en", "id"} {
			message, err := Render(kind, locale, "Sample User", "http://localhost:5173/action?token=abc")
			if err != nil {
				t.Fatalf("Render(%s, %s) error = %v", kind, locale, err)
			}
			if message.Subject == "" || message.Text == "" || message.HTML == "" {
				t.Errorf("Render(%s, %s) returned an empty part", kind, locale)
			}
			if !strings.Contains(message.Text, "token=abc") {
				t.Errorf("Render(%s, %s) text lost the link", kind, locale)
			}
			if !strings.Contains(message.HTML, "token=abc") {
				t.Errorf("Render(%s, %s) HTML lost the link", kind, locale)
			}
		}
	}
}

func TestRenderFallsBackToEnglish(t *testing.T) {
	message, err := Render("reset", "fr", "", "http://example.test/x")
	if err != nil {
		t.Fatal(err)
	}
	if message.Subject != subjects["reset"]["en"] {
		t.Errorf("subject = %q", message.Subject)
	}
	if !strings.Contains(message.Text, "there") {
		t.Errorf("default name was not applied: %q", message.Text)
	}
}

func TestRenderRejectsUnknownTemplate(t *testing.T) {
	if _, err := Render("marketing", "en", "", "http://example.test"); err == nil {
		t.Fatal("Render accepted an unknown template")
	}
}
