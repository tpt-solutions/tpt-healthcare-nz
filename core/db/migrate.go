package db

import (
	"context"

	"github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate runs module-specific SQL migrations from a filesystem directory using
// the migratedb Runner. It is intended for use in module main.go files that do
// not embed their migration files.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	r := migrate.NewFromDir(dir, pool)
	return r.Up(ctx)
}
