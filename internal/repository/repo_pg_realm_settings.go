package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// realmSettingsDocument is the on-disk shape. The encrypted password is a
// base64 JSON string and never leaves this package through the API.
type realmSettingsDocument struct {
	PasswordPolicy domain.PasswordPolicy `json:"password_policy"`
	SessionPolicy  domain.SessionPolicy  `json:"session_policy"`
	SMTP           smtpSettingsDocument  `json:"smtp"`
}

type smtpSettingsDocument struct {
	Host              string  `json:"host"`
	Port              int     `json:"port"`
	Username          *string `json:"username"`
	FromAddress       string  `json:"from_address"`
	UseTLS            bool    `json:"use_tls"`
	PasswordEncrypted []byte  `json:"password_encrypted,omitempty"`
}

// PgRealmSettingsRepository reads and writes realm.settings. PgRealmRepository
// shares the pool, but the settings port stays separate from realm lookup.
type PgRealmSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewRealmSettingsRepository(pool *pgxpool.Pool) *PgRealmSettingsRepository {
	return &PgRealmSettingsRepository{pool: pool}
}

func (r *PgRealmSettingsRepository) GetSettings(ctx context.Context, realmID domain.RealmID) (*domain.RealmSettings, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT settings FROM realm WHERE id = $1::text::uuid`,
		string(realmID)).Scan(&raw)
	if err != nil {
		return nil, mapError(err)
	}
	return decodeRealmSettings(raw)
}

func (r *PgRealmSettingsRepository) UpdateSettings(ctx context.Context, realmID domain.RealmID, settings *domain.RealmSettings, audit *domain.AuditEvent) (*domain.RealmSettings, error) {
	raw, err := encodeRealmSettings(settings)
	if err != nil {
		return nil, err
	}
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE realm SET settings = $2::jsonb, updated_at = now()
			WHERE id = $1::text::uuid`,
			string(realmID), raw)
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return insertAuditTx(ctx, tx, audit)
	})
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func decodeRealmSettings(raw []byte) (*domain.RealmSettings, error) {
	settings := domain.DefaultRealmSettings()
	if len(raw) == 0 {
		return settings, nil
	}
	var doc realmSettingsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.PasswordPolicy.MinLength != 0 {
		settings.PasswordPolicy.MinLength = doc.PasswordPolicy.MinLength
	}
	if doc.SessionPolicy.IdleMinutes != 0 {
		settings.SessionPolicy.IdleMinutes = doc.SessionPolicy.IdleMinutes
	}
	if doc.SessionPolicy.AbsoluteHours != 0 {
		settings.SessionPolicy.AbsoluteHours = doc.SessionPolicy.AbsoluteHours
	}
	if doc.SessionPolicy.AdminIdleMinutes != 0 {
		settings.SessionPolicy.AdminIdleMinutes = doc.SessionPolicy.AdminIdleMinutes
	}
	if doc.SessionPolicy.RememberMeDays != 0 {
		settings.SessionPolicy.RememberMeDays = doc.SessionPolicy.RememberMeDays
	}
	settings.SMTP.Host = doc.SMTP.Host
	settings.SMTP.Port = doc.SMTP.Port
	settings.SMTP.Username = doc.SMTP.Username
	settings.SMTP.FromAddress = doc.SMTP.FromAddress
	settings.SMTP.UseTLS = doc.SMTP.UseTLS
	settings.SMTP.PasswordEncrypted = doc.SMTP.PasswordEncrypted
	settings.SMTP.PasswordSet = len(doc.SMTP.PasswordEncrypted) > 0
	if settings.SMTP.Port == 0 {
		settings.SMTP.Port = 587
	}
	return settings, nil
}

func encodeRealmSettings(settings *domain.RealmSettings) ([]byte, error) {
	doc := realmSettingsDocument{
		PasswordPolicy: settings.PasswordPolicy,
		SessionPolicy:  settings.SessionPolicy,
		SMTP: smtpSettingsDocument{
			Host:              settings.SMTP.Host,
			Port:              settings.SMTP.Port,
			Username:          settings.SMTP.Username,
			FromAddress:       settings.SMTP.FromAddress,
			UseTLS:            settings.SMTP.UseTLS,
			PasswordEncrypted: settings.SMTP.PasswordEncrypted,
		},
	}
	return json.Marshal(doc)
}

var _ domain.RealmSettingsRepository = (*PgRealmSettingsRepository)(nil)
