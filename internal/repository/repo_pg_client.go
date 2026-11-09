package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgClientRepository stores OIDC client registrations.
type PgClientRepository struct {
	pool *pgxpool.Pool
}

func NewClientRepository(pool *pgxpool.Pool) *PgClientRepository {
	return &PgClientRepository{pool: pool}
}

func (r *PgClientRepository) Create(ctx context.Context, client *domain.Client, secret *domain.ClientSecret, redirects []domain.ClientRedirectURI, origins []domain.ClientCORSOrigin) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO client (id, realm_id, client_id, type, grant_types, allowed_scopes,
				third_party, access_token_ttl_seconds, id_token_ttl_seconds, backchannel_logout_uri)
			VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at`,
			string(client.ID), string(client.RealmID), client.ClientID, client.Type,
			client.GrantTypes, client.AllowedScopes, client.ThirdParty,
			client.AccessTokenTTLSeconds, client.IDTokenTTLSeconds,
			client.BackchannelLogoutURI,
		).Scan(&client.CreatedAt, &client.UpdatedAt)
		if err != nil {
			return mapError(err)
		}

		if secret != nil {
			if secret.ID == "" {
				secret.ID = uuid.NewString()
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO client_secret (id, client_id, secret_hash)
				VALUES ($1::text::uuid, $2::text::uuid, $3)`,
				secret.ID, string(client.ID), secret.SecretHash); err != nil {
				return mapError(err)
			}
		}

		for _, redirect := range redirects {
			if _, err := tx.Exec(ctx, `
				INSERT INTO client_redirect_uri (id, client_id, uri, kind)
				VALUES ($1::text::uuid, $2::text::uuid, $3, $4)`,
				uuid.NewString(), string(client.ID), redirect.URI, redirect.Kind); err != nil {
				return mapError(err)
			}
		}

		for _, origin := range origins {
			if _, err := tx.Exec(ctx, `
				INSERT INTO client_cors_origin (id, client_id, origin)
				VALUES ($1::text::uuid, $2::text::uuid, $3)`,
				uuid.NewString(), string(client.ID), origin.Origin); err != nil {
				return mapError(err)
			}
		}
		return nil
	})
}

func (r *PgClientRepository) GetByClientID(ctx context.Context, realmID domain.RealmID, clientID string) (*domain.Client, error) {
	var client domain.Client
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, realm_id::text, client_id, type, grant_types, allowed_scopes,
			third_party, access_token_ttl_seconds, id_token_ttl_seconds,
			backchannel_logout_uri, disabled_at, created_at, updated_at
		FROM client WHERE realm_id = $1::text::uuid AND client_id = $2`,
		string(realmID), clientID,
	).Scan(&client.ID, &client.RealmID, &client.ClientID, &client.Type,
		&client.GrantTypes, &client.AllowedScopes, &client.ThirdParty,
		&client.AccessTokenTTLSeconds, &client.IDTokenTTLSeconds,
		&client.BackchannelLogoutURI, &client.DisabledAt,
		&client.CreatedAt, &client.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &client, nil
}

func (r *PgClientRepository) ListByRealm(ctx context.Context, realmID domain.RealmID) ([]domain.Client, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, realm_id::text, client_id, type, grant_types, allowed_scopes,
			third_party, access_token_ttl_seconds, id_token_ttl_seconds,
			backchannel_logout_uri, disabled_at, created_at, updated_at
		FROM client WHERE realm_id = $1::text::uuid ORDER BY client_id`,
		string(realmID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var clients []domain.Client
	for rows.Next() {
		var client domain.Client
		if err := rows.Scan(&client.ID, &client.RealmID, &client.ClientID, &client.Type,
			&client.GrantTypes, &client.AllowedScopes, &client.ThirdParty,
			&client.AccessTokenTTLSeconds, &client.IDTokenTTLSeconds,
			&client.BackchannelLogoutURI, &client.DisabledAt,
			&client.CreatedAt, &client.UpdatedAt); err != nil {
			return nil, mapError(err)
		}
		clients = append(clients, client)
	}
	return clients, mapError(rows.Err())
}

// FindRedirectURI answers with an exact match on scheme, host, port, path, and
// query. No wildcard or prefix comparison is allowed.
func (r *PgClientRepository) FindRedirectURI(ctx context.Context, clientID domain.ClientID, uri, kind string) (bool, error) {
	var found bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM client_redirect_uri
			WHERE client_id = $1::text::uuid AND uri = $2 AND kind = $3
		)`, string(clientID), uri, kind).Scan(&found)
	return found, mapError(err)
}

func (r *PgClientRepository) CORSOrigins(ctx context.Context, clientID domain.ClientID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT origin FROM client_cors_origin
		WHERE client_id = $1::text::uuid ORDER BY origin`, string(clientID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var origins []string
	for rows.Next() {
		var origin string
		if err := rows.Scan(&origin); err != nil {
			return nil, mapError(err)
		}
		origins = append(origins, origin)
	}
	return origins, mapError(rows.Err())
}

func (r *PgClientRepository) AuthorizeSecret(ctx context.Context, clientID domain.ClientID, secretHash string) (bool, error) {
	var found bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM client_secret
			WHERE client_id = $1::text::uuid AND secret_hash = $2
				AND revoked_at IS NULL
		)`, string(clientID), secretHash).Scan(&found)
	return found, mapError(err)
}

func (r *PgClientRepository) GetConsent(ctx context.Context, userID domain.UserID, clientID domain.ClientID) (*domain.Consent, error) {
	var consent domain.Consent
	err := r.pool.QueryRow(ctx, `
		SELECT user_id::text, client_id::text, scope, granted_at
		FROM consent WHERE user_id = $1::text::uuid AND client_id = $2::text::uuid`,
		string(userID), string(clientID),
	).Scan(&consent.UserID, &consent.ClientID, &consent.Scope, &consent.GrantedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &consent, nil
}

func (r *PgClientRepository) UpsertConsent(ctx context.Context, consent *domain.Consent) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO consent (id, user_id, client_id, scope)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4)
		ON CONFLICT (user_id, client_id) DO UPDATE
		SET scope = EXCLUDED.scope, granted_at = now()
		RETURNING granted_at`,
		uuid.NewString(), string(consent.UserID), string(consent.ClientID), consent.Scope,
	).Scan(&consent.GrantedAt)
	return mapError(err)
}
