package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

const adminClientColumns = `c.id::text, c.client_id, c.name, c.type,
	COALESCE((SELECT jsonb_agg(uri ORDER BY uri) FROM client_redirect_uri
		WHERE client_id = c.id AND kind = 'authorize'), '[]'::jsonb),
	COALESCE((SELECT jsonb_agg(uri ORDER BY uri) FROM client_redirect_uri
		WHERE client_id = c.id AND kind = 'post_logout'), '[]'::jsonb),
	COALESCE((SELECT jsonb_agg(origin ORDER BY origin) FROM client_cors_origin
		WHERE client_id = c.id), '[]'::jsonb),
	c.grant_types, c.allowed_scopes, c.third_party,
	c.access_token_ttl_seconds, c.id_token_ttl_seconds,
	c.backchannel_logout_uri, c.disabled_at,
	(SELECT max(created_at) FROM client_secret
		WHERE client_id = c.id AND revoked_at IS NULL),
	c.created_at`

// PgAdminClientRepository reads and writes console client registrations. It
// embeds the OIDC repository so create shares one insert path.
type PgAdminClientRepository struct {
	*PgClientRepository
}

func NewAdminClientRepository(pool *pgxpool.Pool) *PgAdminClientRepository {
	return &PgAdminClientRepository{PgClientRepository: NewClientRepository(pool)}
}

func (r *PgAdminClientRepository) ListAdmin(ctx context.Context, realmID domain.RealmID, filter domain.AdminClientFilter) ([]domain.AdminClientView, string, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{string(realmID)}
	clauses := []string{"c.realm_id = $1::text::uuid"}
	if filter.After != "" {
		createdAt, id, err := decodeAdminCursor(filter.After)
		if err != nil {
			return nil, "", domain.ErrNotFound
		}
		args = append(args, createdAt, id)
		clauses = append(clauses, "(c.created_at, c.id) > ($2::timestamptz, $3::text::uuid)")
	}
	args = append(args, limit+1)
	query := `SELECT ` + adminClientColumns + `
		FROM client c WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY c.created_at, c.id LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", mapError(err)
	}
	defer rows.Close()

	views := make([]domain.AdminClientView, 0, limit)
	for rows.Next() {
		view, err := scanAdminClientView(rows)
		if err != nil {
			return nil, "", err
		}
		views = append(views, *view)
	}
	if err := rows.Err(); err != nil {
		return nil, "", mapError(err)
	}
	next := ""
	if len(views) > limit {
		last := views[limit-1]
		next = encodeAdminCursor(last.CreatedAt, last.ID)
		views = views[:limit]
	}
	return views, next, nil
}

func (r *PgAdminClientRepository) GetAdmin(ctx context.Context, realmID domain.RealmID, clientID domain.ClientID) (*domain.AdminClientView, error) {
	return scanAdminClientView(r.pool.QueryRow(ctx, `SELECT `+adminClientColumns+`
		FROM client c WHERE c.realm_id = $1::text::uuid AND c.id = $2::text::uuid`,
		string(realmID), string(clientID)))
}

func (r *PgAdminClientRepository) CreateWithAudit(ctx context.Context, client *domain.Client, secret *domain.ClientSecret, redirects []domain.ClientRedirectURI, origins []domain.ClientCORSOrigin, audit *domain.AuditEvent) error {
	return r.create(ctx, client, secret, redirects, origins, audit)
}

func (r *PgAdminClientRepository) UpdateAdmin(ctx context.Context, realmID domain.RealmID, clientID domain.ClientID, update domain.AdminClientUpdate, audit *domain.AuditEvent) (*domain.AdminClientView, error) {
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var name, clientType string
		var backchannel *string
		var disabledAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT name, type, backchannel_logout_uri, disabled_at
			FROM client WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
			FOR UPDATE`, string(realmID), string(clientID),
		).Scan(&name, &clientType, &backchannel, &disabledAt)
		if err != nil {
			return mapError(err)
		}
		if update.Name != nil {
			name = *update.Name
		}
		if update.BackchannelLogoutURI.Set {
			if update.BackchannelLogoutURI.Value == nil {
				backchannel = nil
			} else {
				value := *update.BackchannelLogoutURI.Value
				backchannel = &value
			}
		}
		disabledNow := false
		if update.Disabled != nil {
			if *update.Disabled && disabledAt == nil {
				now := time.Now().UTC()
				disabledAt = &now
				disabledNow = true
			} else if !*update.Disabled {
				disabledAt = nil
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE client SET name = $3, backchannel_logout_uri = $4,
				disabled_at = $5, updated_at = now()
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid`,
			string(realmID), string(clientID), name, backchannel, disabledAt); err != nil {
			return mapError(err)
		}
		if err := replaceClientChildrenTx(ctx, tx, clientID, update); err != nil {
			return err
		}
		if disabledNow {
			if _, err := tx.Exec(ctx, `
				UPDATE refresh_token SET revoked_at = now()
				WHERE client_id = $1::text::uuid AND revoked_at IS NULL`,
				string(clientID)); err != nil {
				return mapError(err)
			}
		}
		return insertAuditTx(ctx, tx, audit)
	})
	if err != nil {
		return nil, err
	}
	return r.GetAdmin(ctx, realmID, clientID)
}

func replaceClientChildrenTx(ctx context.Context, tx pgx.Tx, clientID domain.ClientID, update domain.AdminClientUpdate) error {
	if update.RedirectURIs != nil {
		if _, err := tx.Exec(ctx, `
			DELETE FROM client_redirect_uri
			WHERE client_id = $1::text::uuid AND kind = 'authorize'`,
			string(clientID)); err != nil {
			return mapError(err)
		}
		for _, uri := range *update.RedirectURIs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO client_redirect_uri (id, client_id, uri, kind)
				VALUES ($1::text::uuid, $2::text::uuid, $3, 'authorize')`,
				uuid.NewString(), string(clientID), uri); err != nil {
				return mapError(err)
			}
		}
	}
	if update.PostLogoutRedirectURIs != nil {
		if _, err := tx.Exec(ctx, `
			DELETE FROM client_redirect_uri
			WHERE client_id = $1::text::uuid AND kind = 'post_logout'`,
			string(clientID)); err != nil {
			return mapError(err)
		}
		for _, uri := range *update.PostLogoutRedirectURIs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO client_redirect_uri (id, client_id, uri, kind)
				VALUES ($1::text::uuid, $2::text::uuid, $3, 'post_logout')`,
				uuid.NewString(), string(clientID), uri); err != nil {
				return mapError(err)
			}
		}
	}
	if update.CORSOrigins != nil {
		if _, err := tx.Exec(ctx, `
			DELETE FROM client_cors_origin WHERE client_id = $1::text::uuid`,
			string(clientID)); err != nil {
			return mapError(err)
		}
		for _, origin := range *update.CORSOrigins {
			if _, err := tx.Exec(ctx, `
				INSERT INTO client_cors_origin (id, client_id, origin)
				VALUES ($1::text::uuid, $2::text::uuid, $3)`,
				uuid.NewString(), string(clientID), origin); err != nil {
				return mapError(err)
			}
		}
	}
	if update.GrantTypes != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE client SET grant_types = $2 WHERE id = $1::text::uuid`,
			string(clientID), *update.GrantTypes); err != nil {
			return mapError(err)
		}
	}
	if update.AllowedScopes != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE client SET allowed_scopes = $2 WHERE id = $1::text::uuid`,
			string(clientID), *update.AllowedScopes); err != nil {
			return mapError(err)
		}
	}
	if update.ThirdParty != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE client SET third_party = $2 WHERE id = $1::text::uuid`,
			string(clientID), *update.ThirdParty); err != nil {
			return mapError(err)
		}
	}
	if update.AccessTokenTTLSeconds.Set {
		if _, err := tx.Exec(ctx, `
			UPDATE client SET access_token_ttl_seconds = $2 WHERE id = $1::text::uuid`,
			string(clientID), update.AccessTokenTTLSeconds.Value); err != nil {
			return mapError(err)
		}
	}
	if update.IDTokenTTLSeconds.Set {
		if _, err := tx.Exec(ctx, `
			UPDATE client SET id_token_ttl_seconds = $2 WHERE id = $1::text::uuid`,
			string(clientID), update.IDTokenTTLSeconds.Value); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (r *PgAdminClientRepository) RotateSecret(ctx context.Context, realmID domain.RealmID, clientID domain.ClientID, secretHash string, audit *domain.AuditEvent) (time.Time, error) {
	var rotatedAt time.Time
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var clientType string
		err := tx.QueryRow(ctx, `
			SELECT type FROM client
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
			FOR UPDATE`, string(realmID), string(clientID)).Scan(&clientType)
		if err != nil {
			return mapError(err)
		}
		if clientType != domain.ClientTypeConfidential {
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx, `
			UPDATE client_secret SET revoked_at = now()
			WHERE client_id = $1::text::uuid AND revoked_at IS NULL`,
			string(clientID)); err != nil {
			return mapError(err)
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO client_secret (id, client_id, secret_hash)
			VALUES ($1::text::uuid, $2::text::uuid, $3)
			RETURNING created_at`, uuid.NewString(), string(clientID), secretHash).Scan(&rotatedAt)
		if err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
	return rotatedAt, err
}

// Delete refuses while the client still has a consent or a live refresh token.
// Sessions are user-wide, so the refresh-token family is the client-scoped
// proxy for an active session.
func (r *PgAdminClientRepository) Delete(ctx context.Context, realmID domain.RealmID, clientID domain.ClientID, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var found string
		err := tx.QueryRow(ctx, `
			SELECT id::text FROM client
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
			FOR UPDATE`, string(realmID), string(clientID)).Scan(&found)
		if err != nil {
			return mapError(err)
		}
		var inUse bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM consent WHERE client_id = $1::text::uuid)
				OR EXISTS (
					SELECT 1 FROM refresh_token
					WHERE client_id = $1::text::uuid
						AND revoked_at IS NULL
						AND expires_at > now()
						AND absolute_expires_at > now()
				)`, string(clientID)).Scan(&inUse)
		if err != nil {
			return mapError(err)
		}
		if inUse {
			return domain.ErrConflict
		}
		if err := insertAuditTx(ctx, tx, audit); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM client WHERE id = $1::text::uuid`, string(clientID))
		return mapError(err)
	})
}

func scanAdminClientView(row rowScanner) (*domain.AdminClientView, error) {
	var view domain.AdminClientView
	var redirectJSON, postLogoutJSON, originJSON []byte
	var disabledAt *time.Time
	err := row.Scan(
		&view.ID, &view.ClientID, &view.Name, &view.Type,
		&redirectJSON, &postLogoutJSON, &originJSON,
		&view.GrantTypes, &view.AllowedScopes, &view.ThirdParty,
		&view.AccessTokenTTLSeconds, &view.IDTokenTTLSeconds,
		&view.BackchannelLogoutURI, &disabledAt, &view.SecretRotatedAt,
		&view.CreatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	if err := json.Unmarshal(redirectJSON, &view.RedirectURIs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(postLogoutJSON, &view.PostLogoutRedirectURIs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(originJSON, &view.CORSOrigins); err != nil {
		return nil, err
	}
	if view.RedirectURIs == nil {
		view.RedirectURIs = []string{}
	}
	if view.PostLogoutRedirectURIs == nil {
		view.PostLogoutRedirectURIs = []string{}
	}
	if view.CORSOrigins == nil {
		view.CORSOrigins = []string{}
	}
	if view.GrantTypes == nil {
		view.GrantTypes = []string{}
	}
	if view.AllowedScopes == nil {
		view.AllowedScopes = []string{}
	}
	view.Disabled = disabledAt != nil
	return &view, nil
}

var _ domain.AdminClientRepository = (*PgAdminClientRepository)(nil)
