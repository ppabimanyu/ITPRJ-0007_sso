package domain

// MailJob is the outbox payload the email worker renders. The raw token lives
// in Link inside Postgres and never reaches a log.
type MailJob struct {
	To       string `json:"to"`
	Name     string `json:"name"`
	Template string `json:"template"`
	Locale   string `json:"locale"`
	Link     string `json:"link"`
}
