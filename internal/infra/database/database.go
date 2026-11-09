// Package database owns the PostgreSQL pool, migrations, and the transaction
// helper repositories use. Handlers never see a pgx type.
package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ppa/itprj-0007-sso/migrations"
)

// DB bundles the pgx pool with the goose provider. Both share one connection
// pool through pgx's database/sql adapter.
type DB struct {
	Pool *pgxpool.Pool

	provider *goose.Provider
	latest   int64
}

// Open connects to Postgres and verifies the connection before returning.
func Open(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		pool.Close()
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	latest, err := LatestVersion()
	if err != nil {
		_ = sqlDB.Close()
		pool.Close()
		return nil, err
	}

	return &DB{Pool: pool, provider: provider, latest: latest}, nil
}

// Close releases the pool. The database/sql wrapper shares it, so closing the
// pool is enough.
func (d *DB) Close() {
	d.Pool.Close()
}

// Ping reports database reachability.
func (d *DB) Ping(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}

// Up applies all pending migrations.
func (d *DB) Up(ctx context.Context) error {
	if _, err := d.provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down rolls back the most recent migration.
func (d *DB) Down(ctx context.Context) error {
	if _, err := d.provider.Down(ctx); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Version returns the applied schema version.
func (d *DB) Version(ctx context.Context) (int64, error) {
	return d.provider.GetDBVersion(ctx)
}

// LatestVersion returns the highest embedded migration number.
func LatestVersion() (int64, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return 0, fmt.Errorf("read migrations: %w", err)
	}
	var latest int64
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			continue
		}
		if version > latest {
			latest = version
		}
	}
	if latest == 0 {
		return 0, fmt.Errorf("no migrations embedded")
	}
	return latest, nil
}

// Ready verifies the database answers and the schema version matches the
// embedded migrations.
func (d *DB) Ready(ctx context.Context) error {
	if err := d.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	version, err := d.Version(ctx)
	if err != nil {
		return fmt.Errorf("read version: %w", err)
	}
	if version != d.latest {
		return fmt.Errorf("schema version %d, want %d", version, d.latest)
	}
	return nil
}

// WithTx runs fn in one transaction, committing only when fn succeeds. A panic
// rolls back before propagating.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
