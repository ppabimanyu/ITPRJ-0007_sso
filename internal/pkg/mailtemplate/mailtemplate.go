// Package mailtemplate renders the embedded invitation, verification, and
// password reset emails in English and Indonesian.
package mailtemplate

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	texttemplate "text/template"
)

//go:embed templates/*.tmpl
var templates embed.FS

// Message is one rendered email.
type Message struct {
	Subject string
	Text    string
	HTML    string
}

type templateData struct {
	Name string
	Link string
}

var subjects = map[string]map[string]string{
	"invite": {
		"en": "You are invited to sign in",
		"id": "Anda diundang untuk masuk",
	},
	"verify": {
		"en": "Verify your email address",
		"id": "Verifikasi alamat email Anda",
	},
	"reset": {
		"en": "Reset your password",
		"id": "Atur ulang kata sandi Anda",
	},
}

// Render builds the text and HTML bodies. Unknown locales fall back to English.
func Render(kind, locale, name, link string) (Message, error) {
	purposes, ok := subjects[kind]
	if !ok {
		return Message{}, fmt.Errorf("unknown mail template %q", kind)
	}
	locale = normalizeLocale(locale)
	subject := purposes[locale]
	data := templateData{Name: defaultName(name, locale), Link: link}

	textBody, err := renderFile("templates/"+kind+"."+locale+".txt.tmpl", data, false)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := renderFile("templates/"+kind+"."+locale+".html.tmpl", data, true)
	if err != nil {
		return Message{}, err
	}
	return Message{Subject: subject, Text: textBody, HTML: htmlBody}, nil
}

func renderFile(path string, data templateData, html bool) (string, error) {
	raw, err := templates.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	var buf bytes.Buffer
	if html {
		tpl, err := template.New(path).Parse(string(raw))
		if err != nil {
			return "", err
		}
		if err := tpl.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	tpl, err := texttemplate.New(path).Parse(string(raw))
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func normalizeLocale(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "id":
		return "id"
	default:
		return "en"
	}
}

func defaultName(name, locale string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	if locale == "id" {
		return "Pengguna"
	}
	return "there"
}
