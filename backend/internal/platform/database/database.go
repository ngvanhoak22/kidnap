package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"journey-safety/backend/migrations"
)

func Open(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = 1
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "15000"
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate takes a dedicated advisory lock for the whole migration command.
// Only use this from a command or serial test setup (goose uses global state).
func Migrate(ctx context.Context, dsn, command string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(712453092)`); err != nil {
		return err
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	goose.SetBaseFS(migrations.Files)
	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch command {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "down":
		return goose.DownContext(ctx, db, ".")
	case "status":
		return goose.StatusContext(ctx, db, ".")
	default:
		return fmt.Errorf("unsupported migration command")
	}
}

func Ready(ctx context.Context, db *pgxpool.Pool) bool {
	var ok bool
	err := db.QueryRow(ctx, `SELECT current_setting('server_version_num')::int/10000=17 AND EXISTS(SELECT 1 FROM pg_extension WHERE extname='postgis') AND EXISTS(SELECT 1 FROM pg_extension WHERE extname='timescaledb') AND to_regclass('public.users') IS NOT NULL AND to_regclass('public.user_sessions') IS NOT NULL AND to_regclass('public.trusted_contacts') IS NOT NULL AND to_regclass('public.trusted_contact_invitations') IS NOT NULL AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='trusted_contacts' AND column_name='label')`).Scan(&ok)
	return err == nil && ok
}

// SafeApplicationRole rejects administrative roles for a production API pool.
func SafeApplicationRole(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) bool {
	var safe bool
	err := q.QueryRow(ctx, `SELECT NOT (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls) FROM pg_roles WHERE rolname=current_user`).Scan(&safe)
	return err == nil && safe
}
