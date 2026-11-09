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
