package workerhandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

func TestOutboxBackoffGrowsExponentially(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 5 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
	}
	for _, tc := range cases {
		if got := outboxBackoff(tc.attempt); got != tc.want {
			t.Errorf("outboxBackoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
	if outboxBackoff(-1) <= 0 {
		t.Error("negative attempt produced a non-positive delay")
	}
}

type fakeOutbox struct {
	sent    []domain.OutboxEventID
	retried int
	failed  int
}

func (f *fakeOutbox) Enqueue(context.Context, *domain.OutboxEvent) error { return nil }
func (f *fakeOutbox) Claim(context.Context, int, time.Duration) ([]domain.OutboxEvent, error) {
	return nil, nil
}
func (f *fakeOutbox) MarkSent(_ context.Context, id domain.OutboxEventID) error {
	f.sent = append(f.sent, id)
	return nil
}
func (f *fakeOutbox) MarkRetry(context.Context, domain.OutboxEventID, string, time.Time) error {
	f.retried++
	return nil
}
func (f *fakeOutbox) MarkFailed(context.Context, domain.OutboxEventID, string) error {
	f.failed++
	return nil
}

type fakeSigner struct {
	token domain.LogoutToken
}

func (f *fakeSigner) SignLogoutToken(_ context.Context, _ domain.RealmID, token domain.LogoutToken) (string, error) {
	f.token = token
	return "signed-logout-token", nil
}

type fakeSender struct {
	uri   string
	token string
}

func (f *fakeSender) Send(_ context.Context, uri, token string) error {
	f.uri = uri
	f.token = token
	return nil
}

type fakeMailSender struct {
	sent int
}

func (f *fakeMailSender) Send(context.Context, domain.MailMessage) error {
	f.sent++
	return nil
}

type fakeRealmSettings struct {
	settings map[domain.RealmID]domain.RealmSettings
}

func (f *fakeRealmSettings) GetSettings(_ context.Context, realmID domain.RealmID) (*domain.RealmSettings, error) {
	settings, ok := f.settings[realmID]
	if !ok {
		return domain.DefaultRealmSettings(), nil
	}
	return &settings, nil
}

func (f *fakeRealmSettings) UpdateSettings(_ context.Context, _ domain.RealmID, settings *domain.RealmSettings, _ *domain.AuditEvent) (*domain.RealmSettings, error) {
	return settings, nil
}

type selectingMailFactory struct {
	fallback *fakeMailSender
	realm    *fakeMailSender
}

func (f *selectingMailFactory) SenderFor(settings domain.RealmSettings) (domain.MailSender, error) {
	if settings.SMTP.Host == "" {
		return f.fallback, nil
	}
	return f.realm, nil
}

func TestOutboxEmailUsesRealmSenderWithEnvFallback(t *testing.T) {
	repo := &fakeOutbox{}
	fallback := &fakeMailSender{}
	realmSender := &fakeMailSender{}
	settings := &fakeRealmSettings{settings: map[domain.RealmID]domain.RealmSettings{
		"realm-with-smtp": func() domain.RealmSettings {
			value := *domain.DefaultRealmSettings()
			value.SMTP.Host = "smtp.example.com"
			value.SMTP.FromAddress = "no-reply@example.com"
			return value
		}(),
	}}
	worker := NewOutboxWorker(repo, fallback, nil, nil, slog.New(slog.DiscardHandler))
	worker.UseRealmMail(&selectingMailFactory{fallback: fallback, realm: realmSender}, settings)
	ctx := logger.WithLogger(context.Background(), slog.New(slog.DiscardHandler))

	payload, err := json.Marshal(domain.MailJob{
		To:       "user@example.com",
		Name:     "User",
		Template: "invite",
		Locale:   "en",
		Link:     "https://idp.example/accept-invite?token=value",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []*domain.OutboxEvent{
		{ID: "outbox-realm", RealmID: "realm-with-smtp", Type: domain.OutboxEmailInvite, Payload: payload},
		{ID: "outbox-fallback", RealmID: "realm-empty", Type: domain.OutboxEmailInvite, Payload: payload},
	} {
		worker.handle(ctx, event)
	}
	if realmSender.sent != 1 {
		t.Fatalf("realm sender sends = %d, want 1", realmSender.sent)
	}
	if fallback.sent != 1 {
		t.Fatalf("fallback sends = %d, want 1", fallback.sent)
	}
	if len(repo.sent) != 2 {
		t.Fatalf("marked sent = %v, want 2", repo.sent)
	}
}

func TestOutboxDispatchesBackchannelLogout(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"client_id": "sample-spa",
		"uri":       "https://rp.example/backchannel",
		"sub":       "user-1",
		"sid":       "session-1",
		"iss":       "https://idp.example/realms/default",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutbox{}
	signer := &fakeSigner{}
	sender := &fakeSender{}
	worker := NewOutboxWorker(repo, nil, signer, sender, slog.New(slog.DiscardHandler))
	worker.handle(logger.WithLogger(context.Background(), slog.New(slog.DiscardHandler)), &domain.OutboxEvent{
		ID:      "outbox-1",
		RealmID: "realm-1",
		Type:    domain.OutboxLogoutBackchannel,
		Payload: payload,
	})
	if len(repo.sent) != 1 || repo.sent[0] != "outbox-1" {
		t.Fatalf("sent = %v", repo.sent)
	}
	if signer.token.Audience != "sample-spa" || signer.token.SessionID != "session-1" {
		t.Errorf("logout claims = %+v", signer.token)
	}
	if _, ok := signer.token.Events[domain.BackchannelLogoutEvent]; !ok {
		t.Errorf("logout token is missing the backchannel event")
	}
	if sender.uri != "https://rp.example/backchannel" || sender.token != "signed-logout-token" {
		t.Errorf("sender got uri=%q token=%q", sender.uri, sender.token)
	}
}
