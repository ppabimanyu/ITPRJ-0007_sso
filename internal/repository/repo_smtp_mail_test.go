package repository

import (
	"context"
	"testing"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

type fakeMailSender struct {
	sent int
}

func (f *fakeMailSender) Send(context.Context, domain.MailMessage) error {
	f.sent++
	return nil
}

func TestSMTPSenderFactoryFallsBackWhenRealmSettingsAreEmpty(t *testing.T) {
	fallback := &fakeMailSender{}
	factory := NewSMTPSenderFactory(fallback, testKeyring(t))
	sender, err := factory.SenderFor(*domain.DefaultRealmSettings())
	if err != nil {
		t.Fatalf("sender: %v", err)
	}
	if sender != fallback {
		t.Fatalf("sender = %T, want the environment fallback", sender)
	}
}

func TestSMTPSenderFactoryBuildsRealmSenderWithDecryptedPassword(t *testing.T) {
	ring := testKeyring(t)
	ciphertext, err := ring.Encrypt([]byte("realm-smtp-password"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	username := "mailer"
	settings := domain.DefaultRealmSettings()
	settings.SMTP = domain.SMTPSettings{
		Host:              "smtp.example.com",
		Port:              2525,
		Username:          &username,
		FromAddress:       "no-reply@example.com",
		UseTLS:            true,
		PasswordSet:       true,
		PasswordEncrypted: ciphertext,
	}
	factory := NewSMTPSenderFactory(&fakeMailSender{}, ring)
	var captured SMTPSenderConfig
	realmSender := &fakeMailSender{}
	factory.newSender = func(cfg SMTPSenderConfig) domain.MailSender {
		captured = cfg
		return realmSender
	}
	sender, err := factory.SenderFor(*settings)
	if err != nil {
		t.Fatalf("sender: %v", err)
	}
	if sender != realmSender {
		t.Fatalf("sender = %T, want the realm sender", sender)
	}
	if captured.Host != "smtp.example.com" || captured.Port != "2525" ||
		captured.From != "no-reply@example.com" || captured.Username != "mailer" ||
		captured.Password != "realm-smtp-password" || !captured.TLS {
		t.Fatalf("captured config = %+v", captured)
	}
}

func TestSMTPSenderFactoryNeedsKeyringForEncryptedPassword(t *testing.T) {
	settings := domain.DefaultRealmSettings()
	settings.SMTP = domain.SMTPSettings{
		Host:              "smtp.example.com",
		Port:              587,
		FromAddress:       "no-reply@example.com",
		PasswordSet:       true,
		PasswordEncrypted: []byte("ciphertext"),
	}
	factory := NewSMTPSenderFactory(&fakeMailSender{}, nil)
	if _, err := factory.SenderFor(*settings); err == nil {
		t.Fatal("expected a missing-keyring error")
	}
}
