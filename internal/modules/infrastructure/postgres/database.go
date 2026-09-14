package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const schemaVersion int64 = 9

func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(connectCtx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("configure PostgreSQL: %w", err)
	}
	if _, err := sqlc.New(pool).DatabaseHealth(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return pool, nil
}

func SchemaCheck(ctx context.Context, pool *pgxpool.Pool) error {
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var version int64
	err := pool.QueryRow(checkCtx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&version)
	if err != nil {
		return errors.New("schema unavailable; run btg-lms migrate")
	}
	if version != schemaVersion {
		return fmt.Errorf("schema version %d is incompatible with application version %d", version, schemaVersion)
	}
	return nil
}

func Migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	goose.SetBaseFS(migrations)
	if err := goose.UpContext(ctx, db, "db/migrations"); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}
