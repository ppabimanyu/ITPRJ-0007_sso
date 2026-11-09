-- +goose Up
ALTER TABLE session ADD COLUMN token_hash text;
ALTER TABLE session ADD COLUMN csrf_hash text;

CREATE UNIQUE INDEX session_token_hash_uq ON session (token_hash)
    WHERE token_hash IS NOT NULL;

ALTER TABLE user_credential
    ADD COLUMN failed_attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN locked_until timestamptz;

ALTER TABLE user_credential
    ADD CONSTRAINT user_credential_failed_attempts_chk CHECK (failed_attempts >= 0);

ALTER TABLE outbox ADD COLUMN locked_until timestamptz;

CREATE INDEX outbox_processing_idx ON outbox (status, locked_until);

-- +goose Down
DROP INDEX outbox_processing_idx;
ALTER TABLE outbox DROP COLUMN locked_until;
ALTER TABLE user_credential DROP CONSTRAINT user_credential_failed_attempts_chk;
ALTER TABLE user_credential
    DROP COLUMN failed_attempts,
    DROP COLUMN locked_until;
DROP INDEX session_token_hash_uq;
ALTER TABLE session
    DROP COLUMN token_hash,
    DROP COLUMN csrf_hash;
