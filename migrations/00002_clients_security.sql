-- +goose Up
CREATE TABLE client (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    client_id text NOT NULL,
    type text NOT NULL,
    grant_types text[] NOT NULL DEFAULT '{authorization_code}',
    allowed_scopes text[] NOT NULL DEFAULT '{openid}',
    third_party boolean NOT NULL DEFAULT false,
    access_token_ttl_seconds integer,
    id_token_ttl_seconds integer,
    backchannel_logout_uri text,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_id_uq UNIQUE (client_id),
    CONSTRAINT client_type_chk CHECK (type IN ('public', 'confidential'))
);

CREATE INDEX client_realm_idx ON client (realm_id);

CREATE TABLE client_redirect_uri (
    id uuid PRIMARY KEY,
    client_id uuid NOT NULL REFERENCES client (id) ON DELETE CASCADE,
    uri text NOT NULL,
    kind text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_redirect_kind_chk CHECK (kind IN ('authorize', 'post_logout')),
    CONSTRAINT client_redirect_uq UNIQUE (client_id, uri, kind)
);

CREATE TABLE client_cors_origin (
    id uuid PRIMARY KEY,
    client_id uuid NOT NULL REFERENCES client (id) ON DELETE CASCADE,
    origin text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_cors_origin_uq UNIQUE (client_id, origin)
);

CREATE TABLE client_secret (
    id uuid PRIMARY KEY,
    client_id uuid NOT NULL REFERENCES client (id) ON DELETE CASCADE,
    secret_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    CONSTRAINT client_secret_hash_uq UNIQUE (secret_hash)
);

CREATE INDEX client_secret_client_idx ON client_secret (client_id, revoked_at);

CREATE TABLE consent (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    client_id uuid NOT NULL REFERENCES client (id) ON DELETE CASCADE,
    scope text NOT NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT consent_user_client_uq UNIQUE (user_id, client_id)
);

CREATE TABLE mfa_totp (
    user_id uuid PRIMARY KEY REFERENCES "user" (id) ON DELETE CASCADE,
    secret_encrypted bytea NOT NULL,
    last_used_step bigint NOT NULL DEFAULT 0,
    confirmed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE mfa_recovery_code (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    code_hash text NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mfa_recovery_hash_uq UNIQUE (code_hash)
);

CREATE INDEX mfa_recovery_user_idx ON mfa_recovery_code (user_id);

CREATE TABLE email_token (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    purpose text NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_by uuid REFERENCES "user" (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT email_token_purpose_chk CHECK (purpose IN ('verify', 'reset', 'invite')),
    CONSTRAINT email_token_hash_uq UNIQUE (token_hash)
);

CREATE INDEX email_token_user_purpose_idx ON email_token (user_id, purpose);
CREATE INDEX email_token_expires_idx ON email_token (expires_at);

CREATE TABLE realm_signing_key (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    kid text NOT NULL,
    algorithm text NOT NULL DEFAULT 'RS256',
    private_key_encrypted bytea NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    retire_at timestamptz,
    CONSTRAINT realm_signing_key_kid_uq UNIQUE (realm_id, kid)
);

CREATE UNIQUE INDEX realm_signing_key_one_active ON realm_signing_key (realm_id)
    WHERE active;

CREATE TABLE audit_event (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    actor_type text NOT NULL,
    actor_id text,
    type text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT audit_actor_type_chk CHECK (actor_type IN ('user', 'admin', 'client', 'system'))
);

CREATE INDEX audit_event_realm_created_idx ON audit_event (realm_id, created_at DESC);
CREATE INDEX audit_event_realm_type_created_idx ON audit_event (realm_id, type, created_at DESC);

CREATE TABLE outbox (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    CONSTRAINT outbox_status_chk CHECK (status IN ('pending', 'processing', 'sent', 'failed'))
);

CREATE INDEX outbox_status_next_attempt_idx ON outbox (status, next_attempt_at);
CREATE INDEX outbox_realm_created_idx ON outbox (realm_id, created_at);

CREATE TABLE idempotency_key (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    key text NOT NULL,
    endpoint text NOT NULL,
    request_hash text NOT NULL,
    response jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CONSTRAINT idempotency_key_endpoint_uq UNIQUE (realm_id, endpoint, key)
);

CREATE INDEX idempotency_key_expires_idx ON idempotency_key (expires_at);

-- +goose Down
DROP TABLE idempotency_key;
DROP TABLE outbox;
DROP TABLE audit_event;
DROP TABLE realm_signing_key;
DROP TABLE email_token;
DROP TABLE mfa_recovery_code;
DROP TABLE mfa_totp;
DROP TABLE consent;
DROP TABLE client_secret;
DROP TABLE client_cors_origin;
DROP TABLE client_redirect_uri;
DROP TABLE client;
