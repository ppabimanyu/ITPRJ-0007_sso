package domain

import "context"

// MailMessage is an outbound email. The worker owns template rendering.
type MailMessage struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// MailSender is an external SMTP boundary placeholder for M2.
type MailSender interface {
	Send(ctx context.Context, message MailMessage) error
}
