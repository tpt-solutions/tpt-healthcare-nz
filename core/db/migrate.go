package db

import (
	"context"

	"github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate runs module-specific SQL migrations from a filesystem directory.
// dirOrLogger accepts either a string directory path or a logger (legacy compat);
// when a non-string value is passed the function returns nil without running migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dirOrLogger any) error {
	dir, ok := dirOrLogger.(string)
	if !ok || dir == "" {
		return nil
	}
	r := migrate.NewFromDir(dir, pool)
	return r.Up(ctx)
}
