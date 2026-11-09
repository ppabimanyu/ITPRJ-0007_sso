package httphandler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/repository"
)

func TestIntegrationRetentionCleansExpiredRows(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "retention-" + uuid.NewString() + "@example.com"
	user := it.createUser(t, realm, email, "correct horse battery staple", true)
	ctx := context.Background()
	retention := repository.NewRetentionRepository(it.db.Pool)
	now := time.Now().UTC()

	oldSession := uuid.NewString()
	recentSession := uuid.NewString()
	mustExec(t, it, ctx, `
		INSERT INTO session (id, realm_id, user_id, auth_time, last_seen_at, idle_expires_at, absolute_expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, $4, $4, $5)`,
		oldSession, string(realm.ID), string(user.ID), now.Add(-40*24*time.Hour), now.Add(-40*24*time.Hour))
	mustExec(t, it, ctx, `
		INSERT INTO session (id, realm_id, user_id, auth_time, last_seen_at, idle_expires_at, absolute_expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, $4, $4, $5)`,
		recentSession, string(realm.ID), string(user.ID), now, now.Add(time.Hour))

	oldRefresh := uuid.NewString()
	recentRefresh := uuid.NewString()
	mustExec(t, it, ctx, `
		INSERT INTO refresh_token (id, session_id, token_hash, family_id, expires_at, absolute_expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4::text::uuid, $5, $5)`,
		oldRefresh, recentSession, uuid.NewString(), uuid.NewString(), now.Add(-40*24*time.Hour))
	mustExec(t, it, ctx, `
		INSERT INTO refresh_token (id, session_id, token_hash, family_id, expires_at, absolute_expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4::text::uuid, $5, $5)`,
		recentRefresh, recentSession, uuid.NewString(), uuid.NewString(), now.Add(time.Hour))

	oldEmailToken := uuid.NewString()
	recentEmailToken := uuid.NewString()
	mustExec(t, it, ctx, `
		INSERT INTO email_token (id, user_id, purpose, token_hash, expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'reset', $3, $4)`,
		oldEmailToken, string(user.ID), uuid.NewString(), now.Add(-3*24*time.Hour))
	mustExec(t, it, ctx, `
		INSERT INTO email_token (id, user_id, purpose, token_hash, expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'reset', $3, $4)`,
		recentEmailToken, string(user.ID), uuid.NewString(), now.Add(time.Hour))

	oldFailed := uuid.NewString()
	recentFailed := uuid.NewString()
	oldPending := uuid.NewString()
	mustExec(t, it, ctx, `
		INSERT INTO outbox (id, realm_id, type, payload, status, processed_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'email.reset', '{}'::jsonb, 'failed', $3)`,
		oldFailed, string(realm.ID), now.Add(-40*24*time.Hour))
	mustExec(t, it, ctx, `
		INSERT INTO outbox (id, realm_id, type, payload, status, processed_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'email.reset', '{}'::jsonb, 'failed', $3)`,
		recentFailed, string(realm.ID), now.Add(-time.Hour))
	mustExec(t, it, ctx, `
		INSERT INTO outbox (id, realm_id, type, payload, status, processed_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'email.reset', '{}'::jsonb, 'pending', $3)`,
		oldPending, string(realm.ID), now.Add(-40*24*time.Hour))

	oldKey := uuid.NewString()
	recentKey := uuid.NewString()
	mustExec(t, it, ctx, `
		INSERT INTO idempotency_key (id, realm_id, key, endpoint, request_hash, expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, 'POST /users', $4, $5)`,
		oldKey, string(realm.ID), uuid.NewString(), uuid.NewString(), now.Add(-3*24*time.Hour))
	mustExec(t, it, ctx, `
		INSERT INTO idempotency_key (id, realm_id, key, endpoint, request_hash, expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, 'POST /users', $4, $5)`,
		recentKey, string(realm.ID), uuid.NewString(), uuid.NewString(), now.Add(time.Hour))

	t.Cleanup(func() {
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM session WHERE id = $1::text::uuid`, recentSession)
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM email_token WHERE id = $1::text::uuid`, recentEmailToken)
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM outbox WHERE id = $1::text::uuid`, recentFailed)
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM outbox WHERE id = $1::text::uuid`, oldPending)
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM idempotency_key WHERE id = $1::text::uuid`, recentKey)
	})

	if deleted, err := retention.DeleteExpiredSessions(ctx, now.Add(-30*24*time.Hour)); err != nil || deleted != 1 {
		t.Fatalf("delete sessions = %d, err = %v", deleted, err)
	}
	if deleted, err := retention.DeleteExpiredRefreshTokens(ctx, now.Add(-30*24*time.Hour)); err != nil || deleted != 1 {
		t.Fatalf("delete refresh tokens = %d, err = %v", deleted, err)
	}
	if deleted, err := retention.DeleteExpiredEmailTokens(ctx, now.Add(-24*time.Hour)); err != nil || deleted != 1 {
		t.Fatalf("delete email tokens = %d, err = %v", deleted, err)
	}
	if deleted, err := retention.DeleteFailedOutbox(ctx, now.Add(-30*24*time.Hour)); err != nil || deleted != 1 {
		t.Fatalf("delete failed outbox = %d, err = %v", deleted, err)
	}
	if deleted, err := retention.DeleteExpiredIdempotencyKeys(ctx, now.Add(-24*time.Hour)); err != nil || deleted != 1 {
		t.Fatalf("delete idempotency keys = %d, err = %v", deleted, err)
	}

	assertRowCount(t, it, ctx, `SELECT count(*) FROM session WHERE id = $1::text::uuid`, recentSession, 1)
	assertRowCount(t, it, ctx, `SELECT count(*) FROM refresh_token WHERE id = $1::text::uuid`, recentRefresh, 1)
	assertRowCount(t, it, ctx, `SELECT count(*) FROM email_token WHERE id = $1::text::uuid`, recentEmailToken, 1)
	assertRowCount(t, it, ctx, `SELECT count(*) FROM outbox WHERE id = $1::text::uuid`, recentFailed, 1)
	assertRowCount(t, it, ctx, `SELECT count(*) FROM outbox WHERE id = $1::text::uuid`, oldPending, 1)
	assertRowCount(t, it, ctx, `SELECT count(*) FROM idempotency_key WHERE id = $1::text::uuid`, recentKey, 1)
}

func mustExec(t *testing.T, it *integration, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := it.db.Pool.Exec(ctx, query, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func assertRowCount(t *testing.T, it *integration, ctx context.Context, query, id string, want int) {
	t.Helper()
	var got int
	if err := it.db.Pool.QueryRow(ctx, query, id).Scan(&got); err != nil {
		t.Fatalf("count row: %v", err)
	}
	if got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}
}
