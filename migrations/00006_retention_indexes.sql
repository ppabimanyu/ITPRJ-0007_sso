-- +goose Up
CREATE INDEX IF NOT EXISTS audit_event_created_idx ON audit_event (created_at);
CREATE INDEX IF NOT EXISTS session_revoked_idx ON session (revoked_at)
    WHERE revoked_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS session_revoked_idx;
DROP INDEX IF EXISTS audit_event_created_idx;
