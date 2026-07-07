package migrate

import "embed"

// MigrationsFS embeds the golang-migrate-compatible migration files in
// migrations/ (paired *.up.sql / *.down.sql, tracked via schema_migrations).
//
//go:embed migrations
var MigrationsFS embed.FS
