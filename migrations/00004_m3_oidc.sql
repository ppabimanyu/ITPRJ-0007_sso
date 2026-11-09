-- +goose Up
ALTER TABLE refresh_token
    ADD COLUMN client_id uuid REFERENCES client (id) ON DELETE CASCADE,
    ADD COLUMN scope text NOT NULL DEFAULT '';

CREATE INDEX refresh_token_client_idx ON refresh_token (client_id);

-- +goose Down
DROP INDEX refresh_token_client_idx;
ALTER TABLE refresh_token
    DROP COLUMN scope,
    DROP COLUMN client_id;
