package repository

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

// SMTPSenderConfig is the SMTP boundary. Realm settings produce one sender per
// realm; the environment config remains the fallback.
type SMTPSenderConfig struct {
	Host     string
	Port     string
	From     string
	Username string
	Password string
	TLS      bool
}

// SMTPSender implements domain.MailSender.
type SMTPSender struct {
	cfg SMTPSenderConfig
}

func NewSMTPSender(cfg SMTPSenderConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

// SMTPSenderFactory selects the realm transport. An empty realm host or from
// address falls back to the environment sender.
type SMTPSenderFactory struct {
	fallback  domain.MailSender
	keyring   *pkg.Keyring
	newSender func(SMTPSenderConfig) domain.MailSender
}

func NewSMTPSenderFactory(fallback domain.MailSender, keyring *pkg.Keyring) *SMTPSenderFactory {
	return &SMTPSenderFactory{
		fallback: fallback,
		keyring:  keyring,
		newSender: func(cfg SMTPSenderConfig) domain.MailSender {
			return NewSMTPSender(cfg)
		},
	}
}

// SenderFor builds a sender from realm settings, decrypting the stored SMTP
// password only when the realm config is complete.
func (f *SMTPSenderFactory) SenderFor(settings domain.RealmSettings) (domain.MailSender, error) {
	smtpSettings := settings.SMTP
	if smtpSettings.Host == "" || smtpSettings.FromAddress == "" {
		return f.fallback, nil
	}
	password := ""
	if len(smtpSettings.PasswordEncrypted) > 0 {
		if f.keyring == nil {
			return nil, errors.New("master keyring is required to read the SMTP password")
		}
		plaintext, err := f.keyring.Decrypt(smtpSettings.PasswordEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt SMTP password: %w", err)
		}
		password = string(plaintext)
	}
	username := ""
	if smtpSettings.Username != nil {
		username = *smtpSettings.Username
	}
	return f.newSender(SMTPSenderConfig{
		Host:     smtpSettings.Host,
		Port:     strconv.Itoa(smtpSettings.Port),
		From:     smtpSettings.FromAddress,
		Username: username,
		Password: password,
		TLS:      smtpSettings.UseTLS,
	}), nil
}

func (s *SMTPSender) Send(ctx context.Context, message domain.MailMessage) error {
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return fmt.Errorf("parse SMTP from: %w", err)
	}
	to, err := mail.ParseAddress(message.To)
	if err != nil {
		return fmt.Errorf("parse recipient: %w", err)
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return errors.New("subject contains a newline")
	}
	body, err := buildMIMEMessage(from.Address, to.Address, message.Subject, message.Text, message.HTML)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("SMTP hello: %w", err)
	}
	if s.cfg.TLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP server does not offer STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{
			ServerName: s.cfg.Host,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return fmt.Errorf("SMTP starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("SMTP mail from: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("SMTP rcpt: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("SMTP write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("SMTP close data: %w", err)
	}
	return client.Quit()
}

func buildMIMEMessage(from, to, subject, textBody, htmlBody string) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	headers := make(textproto.MIMEHeader)
	headers.Set("From", from)
	headers.Set("To", to)
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", subject))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	headers.Set("Content-Type", `multipart/alternative; boundary="`+mw.Boundary()+`"`)
	if err := writeHeaders(&buf, headers); err != nil {
		return nil, err
	}
	if err := writePart(mw, "text/plain; charset=utf-8", textBody); err != nil {
		return nil, err
	}
	if err := writePart(mw, "text/html; charset=utf-8", htmlBody); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeHeaders(buf *bytes.Buffer, headers textproto.MIMEHeader) error {
	for key, values := range headers {
		for _, value := range values {
			if strings.ContainsAny(value, "\r\n") {
				return fmt.Errorf("header %s contains a newline", key)
			}
			if _, err := fmt.Fprintf(buf, "%s: %s\r\n", key, value); err != nil {
				return err
			}
		}
	}
	_, err := buf.WriteString("\r\n")
	return err
}

func writePart(mw *multipart.Writer, contentType, body string) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := mw.CreatePart(header)
	if err != nil {
		return err
	}
	writer := quotedprintable.NewWriter(part)
	if _, err := writer.Write([]byte(body)); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

var _ domain.MailSender = (*SMTPSender)(nil)
