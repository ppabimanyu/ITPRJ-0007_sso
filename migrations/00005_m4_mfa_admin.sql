-- +goose Up
ALTER TABLE client ADD COLUMN name text NOT NULL DEFAULT '';

UPDATE client SET name = client_id WHERE name = '';

ALTER TABLE session ADD COLUMN acr text;

CREATE INDEX mfa_recovery_user_unused_idx ON mfa_recovery_code (user_id)
    WHERE used_at IS NULL;

CREATE INDEX idempotency_key_realm_endpoint_idx ON idempotency_key (realm_id, endpoint);

-- +goose Down
DROP INDEX idempotency_key_realm_endpoint_idx;
DROP INDEX mfa_recovery_user_unused_idx;
ALTER TABLE session DROP COLUMN acr;
ALTER TABLE client DROP COLUMN name;
