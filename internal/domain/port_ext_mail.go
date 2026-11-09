package domain

import "context"

// MailMessage is an outbound email. The worker owns template rendering.
type MailMessage struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// MailSender is the external SMTP boundary.
type MailSender interface {
	Send(ctx context.Context, message MailMessage) error
}

// MailSenderFactory selects the transport for a realm. It returns the
// environment sender when the realm has no SMTP settings.
type MailSenderFactory interface {
	SenderFor(settings RealmSettings) (MailSender, error)
}
