package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner executes SQL migrations against a pgxpool.Pool. It supports two
// source layouts:
//
//   - embed.FS sources (via New) use golang-migrate's versioned
//     {N}_{title}.{up,down}.sql convention, tracked in golang-migrate's own
//     schema_migrations table.
//   - Directory sources (via NewFromDir) use this codebase's simpler,
//     idempotent per-module convention documented in CLAUDE.md: flat
//     NNN_description.sql files (no up/down split) applied in filename
//     order, each written with `CREATE TABLE IF NOT EXISTS` etc. so
//     re-running is a no-op. Applied filenames are tracked in a
//     schema_migrations table keyed by filename.
type Runner struct {
	fs     fs.FS
	pool   *pgxpool.Pool
	dirSrc bool // true when constructed via NewFromDir
}

// New returns a new Runner backed by the given embed.FS and pgxpool.Pool.
func New(fsys embed.FS, pool *pgxpool.Pool) *Runner {
	return &Runner{fs: fsys, pool: pool}
}

// NewFromDir returns a Runner that reads migrations from a filesystem
// directory. Use this for module-specific migrations that are not embedded.
func NewFromDir(dir string, pool *pgxpool.Pool) *Runner {
	return &Runner{fs: os.DirFS(dir), pool: pool, dirSrc: true}
}

// newMigrate constructs a migrate.Migrate instance wired to the embedded FS and pool DSN.
func (r *Runner) newMigrate() (*migrate.Migrate, error) {
	src, err := iofs.New(r.fs, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: create iofs source: %w", err)
	}

	// Derive a standard DSN string from the pool config.
	connCfg := r.pool.Config().ConnConfig
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		connCfg.User,
		connCfg.Password,
		connCfg.Host,
		connCfg.Port,
		connCfg.Database,
	)

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return nil, fmt.Errorf("migrate: create migrate instance: %w", err)
	}

	return m, nil
}

// Up runs all pending up migrations.
func (r *Runner) Up(ctx context.Context) error {
	if r.dirSrc {
		return r.upDir(ctx)
	}

	m, err := r.newMigrate()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}

	return nil
}

// Down rolls back n migration steps. Not supported for directory sources,
// which use flat, idempotent up-only migrations (see Runner doc comment).
func (r *Runner) Down(ctx context.Context, steps int) error {
	if r.dirSrc {
		return fmt.Errorf("migrate: down is not supported for directory-sourced migrations")
	}

	m, err := r.newMigrate()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: down %d steps: %w", steps, err)
	}

	return nil
}

// upDir applies every *.sql file in the directory FS, in filename order,
// that has not already been recorded in the schema_migrations table.
// Each file is applied in its own transaction.
func (r *Runner) upDir(ctx context.Context) error {
	entries, err := fs.ReadDir(r.fs, ".")
	if err != nil {
		return fmt.Errorf("migrate: read migrations dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	if len(names) == 0 {
		return nil
	}

	if _, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename    TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("migrate: create schema_migrations table: %w", err)
	}

	for _, name := range names {
		var alreadyApplied bool
		if err := r.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)`, name,
		).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("migrate: check %s: %w", name, err)
		}
		if alreadyApplied {
			continue
		}

		sqlBytes, err := fs.ReadFile(r.fs, name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: record %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", name, err)
		}
	}

	return nil
}

// Version returns the current migration version, whether it is dirty, and any error.
// Not supported for directory sources (see Runner doc comment).
func (r *Runner) Version(ctx context.Context) (uint, bool, error) {
	if r.dirSrc {
		return 0, false, fmt.Errorf("migrate: version is not supported for directory-sourced migrations")
	}

	m, err := r.newMigrate()
	if err != nil {
		return 0, false, err
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("migrate: version: %w", err)
	}

	return version, dirty, nil
}
