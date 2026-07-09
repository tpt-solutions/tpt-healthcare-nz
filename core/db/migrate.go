package db

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate runs module-specific SQL migrations from a filesystem directory.
// dir must be a string: either a path to a directory of flat, idempotent
// NNN_description.sql migration files (see CLAUDE.md), or an empty string
// meaning "this module has no migrations to run" (a deliberate no-op, not
// an error). Passing any other type is a caller bug and returns an error
// rather than silently skipping migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir any) error {
	path, ok := dir.(string)
	if !ok {
		return fmt.Errorf("db: Migrate: third argument must be a string migrations directory path (or \"\"), got %T", dir)
	}
	if path == "" {
		return nil
	}
	r := migrate.NewFromDir(path, pool)
	return r.Up(ctx)
}
