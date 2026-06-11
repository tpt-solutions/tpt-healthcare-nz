package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds configuration for the pgxpool connection pool.
type Config struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// New creates a new pgxpool.Pool from the provided Config and verifies connectivity via Ping.
func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}

// NewFromEnv creates a new pgxpool.Pool by reading the DATABASE_URL environment variable
// and applying sensible defaults for pool sizing and connection lifetimes.
func NewFromEnv(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("db: DATABASE_URL environment variable is not set")
	}

	cfg := Config{
		DSN:             dsn,
		MaxConns:        25,
		MinConns:        2,
		MaxConnLifetime: 1 * time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}

	return New(ctx, cfg)
}

// Connect is a convenience wrapper around New for code that only has a DSN string.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return New(ctx, Config{
		DSN:             dsn,
		MaxConns:        25,
		MinConns:        2,
		MaxConnLifetime: 1 * time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	})
}

// Pool is a type alias for *pgxpool.Pool, allowing modules to reference the pool
// type as db.Pool without importing pgxpool directly.
type Pool = *pgxpool.Pool

// NamedArgs is a type alias for pgx.NamedArgs, preserved for module compatibility.
type NamedArgs = pgx.NamedArgs

// IsNoRows reports whether err is a pgx "no rows" error.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
