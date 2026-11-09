-- +goose Up
CREATE TABLE realm (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    issuer text NOT NULL,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT realm_name_uq UNIQUE (name)
);

CREATE TABLE "user" (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    email text NOT NULL,
    username text,
    name text NOT NULL DEFAULT '',
    given_name text NOT NULL DEFAULT '',
    family_name text NOT NULL DEFAULT '',
    locale text,
    email_verified_at timestamptz,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX user_realm_email_uq ON "user" (realm_id, lower(email));
CREATE UNIQUE INDEX user_realm_username_uq ON "user" (realm_id, lower(username))
    WHERE username IS NOT NULL;
CREATE INDEX user_realm_created_idx ON "user" (realm_id, created_at);

CREATE TABLE user_credential (
    user_id uuid PRIMARY KEY REFERENCES "user" (id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    algorithm text NOT NULL DEFAULT 'argon2id',
    params jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX role_realm_name_uq ON role (realm_id, lower(name));

CREATE TABLE user_role (
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES role (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_role_realm_role_idx ON user_role (realm_id, role_id);

CREATE TABLE "group" (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX group_realm_name_uq ON "group" (realm_id, lower(name));

CREATE TABLE user_group (
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    group_id uuid NOT NULL REFERENCES "group" (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX user_group_realm_user_idx ON user_group (realm_id, user_id);

CREATE TABLE session (
    id uuid PRIMARY KEY,
    realm_id uuid NOT NULL REFERENCES realm (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    ip inet,
    user_agent text NOT NULL DEFAULT '',
    amr text[] NOT NULL DEFAULT '{}',
    auth_time timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    remember_me boolean NOT NULL DEFAULT false,
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX session_realm_user_idx ON session (realm_id, user_id, revoked_at);
CREATE INDEX session_idle_expires_idx ON session (idle_expires_at);
CREATE INDEX session_absolute_expires_idx ON session (absolute_expires_at);

CREATE TABLE refresh_token (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES session (id) ON DELETE CASCADE,
    token_hash text NOT NULL,
    family_id uuid NOT NULL,
    parent_id uuid REFERENCES refresh_token (id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    used_at timestamptz,
    replaced_by uuid REFERENCES refresh_token (id) ON DELETE SET NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT refresh_token_hash_uq UNIQUE (token_hash)
);

CREATE INDEX refresh_token_session_idx ON refresh_token (session_id);
CREATE INDEX refresh_token_family_idx ON refresh_token (family_id);
CREATE INDEX refresh_token_expires_idx ON refresh_token (expires_at);

-- +goose Down
DROP TABLE refresh_token;
DROP TABLE session;
DROP TABLE user_group;
DROP TABLE "group";
DROP TABLE user_role;
DROP TABLE role;
DROP TABLE user_credential;
DROP TABLE "user";
DROP TABLE realm;
