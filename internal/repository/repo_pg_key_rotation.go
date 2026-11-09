package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/infra/database"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

// MasterKeyRotationStats reports how many envelopes moved to the active key.
type MasterKeyRotationStats struct {
	SigningKeys   int
	SMTPPasswords int
	MFASecrets    int
}

// ReencryptMasterKeys moves every envelope to the active master key. Rows
// already under that key are skipped, so a repeated run is safe.
func ReencryptMasterKeys(ctx context.Context, pool *pgxpool.Pool, ring *pkg.Keyring) (MasterKeyRotationStats, error) {
	stats := MasterKeyRotationStats{}
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		var err error
		stats.SigningKeys, err = reencryptSigningKeys(ctx, tx, ring)
		if err != nil {
			return err
		}
		stats.SMTPPasswords, err = reencryptSMTPPasswords(ctx, tx, ring)
		if err != nil {
			return err
		}
		stats.MFASecrets, err = reencryptMFASecrets(ctx, tx, ring)
		return err
	})
	if err != nil {
		return MasterKeyRotationStats{}, err
	}
	return stats, nil
}

type encryptedRow struct {
	id         string
	ciphertext []byte
}

func reencryptSigningKeys(ctx context.Context, tx pgx.Tx, ring *pkg.Keyring) (int, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, private_key_encrypted FROM realm_signing_key FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("read signing keys: %w", err)
	}
	items, err := collectEncryptedRows(rows)
	if err != nil {
		return 0, fmt.Errorf("read signing keys: %w", err)
	}
	count := 0
	for _, item := range items {
		next, changed, err := reencrypt(ring, item.ciphertext)
		if err != nil {
			return count, fmt.Errorf("reencrypt signing key %s: %w", item.id, err)
		}
		if !changed {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE realm_signing_key SET private_key_encrypted = $2
			WHERE id = $1::text::uuid`, item.id, next); err != nil {
			return count, fmt.Errorf("write signing key %s: %w", item.id, err)
		}
		count++
	}
	return count, nil
}

func reencryptSMTPPasswords(ctx context.Context, tx pgx.Tx, ring *pkg.Keyring) (int, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, settings FROM realm FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("read realm settings: %w", err)
	}
	items, err := collectEncryptedRows(rows)
	if err != nil {
		return 0, fmt.Errorf("read realm settings: %w", err)
	}
	count := 0
	for _, item := range items {
		encoded, changed, err := reencryptSMTPPassword(item.ciphertext, ring)
		if err != nil {
			return count, fmt.Errorf("reencrypt realm settings %s: %w", item.id, err)
		}
		if !changed {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE realm
			SET settings = jsonb_set(settings, '{smtp,password_encrypted}', to_jsonb($2::text), true),
				updated_at = now()
			WHERE id = $1::text::uuid`, item.id, encoded); err != nil {
			return count, fmt.Errorf("write realm settings %s: %w", item.id, err)
		}
		count++
	}
	return count, nil
}

func reencryptMFASecrets(ctx context.Context, tx pgx.Tx, ring *pkg.Keyring) (int, error) {
	rows, err := tx.Query(ctx, `SELECT user_id::text, secret_encrypted FROM mfa_totp FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("read MFA secrets: %w", err)
	}
	items, err := collectEncryptedRows(rows)
	if err != nil {
		return 0, fmt.Errorf("read MFA secrets: %w", err)
	}
	count := 0
	for _, item := range items {
		next, changed, err := reencrypt(ring, item.ciphertext)
		if err != nil {
			return count, fmt.Errorf("reencrypt MFA secret %s: %w", item.id, err)
		}
		if !changed {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE mfa_totp SET secret_encrypted = $2, updated_at = now()
			WHERE user_id = $1::text::uuid`, item.id, next); err != nil {
			return count, fmt.Errorf("write MFA secret %s: %w", item.id, err)
		}
		count++
	}
	return count, nil
}

func collectEncryptedRows(rows pgx.Rows) ([]encryptedRow, error) {
	defer rows.Close()
	var items []encryptedRow
	for rows.Next() {
		var item encryptedRow
		if err := rows.Scan(&item.id, &item.ciphertext); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func reencrypt(ring *pkg.Keyring, ciphertext []byte) ([]byte, bool, error) {
	keyID, err := ring.KeyID(ciphertext)
	if err != nil {
		return nil, false, err
	}
	if keyID == ring.ActiveKeyID() {
		return ciphertext, false, nil
	}
	plaintext, err := ring.Decrypt(ciphertext)
	if err != nil {
		return nil, false, err
	}
	next, err := ring.Encrypt(plaintext)
	if err != nil {
		return nil, false, err
	}
	return next, true, nil
}

// reencryptSMTPPassword returns the new base64 ciphertext so the caller can
// update only that field and leave concurrent settings edits intact.
func reencryptSMTPPassword(raw []byte, ring *pkg.Keyring) (string, bool, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", false, err
	}
	smtp, ok := doc["smtp"].(map[string]any)
	if !ok {
		return "", false, nil
	}
	encoded, ok := smtp["password_encrypted"].(string)
	if !ok || encoded == "" {
		return "", false, nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false, err
	}
	next, changed, err := reencrypt(ring, ciphertext)
	if err != nil || !changed {
		return "", false, err
	}
	return base64.StdEncoding.EncodeToString(next), true, nil
}
