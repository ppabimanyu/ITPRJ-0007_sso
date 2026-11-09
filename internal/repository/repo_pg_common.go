// Package repository implements the domain ports with PostgreSQL. It is flat
// by design: grouping happens through file prefixes, not folders.
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// SQL statements cast UUID parameters through text and select IDs as text so
// the domain keeps plain string identifier types without a pgtype dependency.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

func nullString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func jsonValue(value []byte) []byte {
	if len(value) == 0 {
		return []byte("{}")
	}
	return value
}
