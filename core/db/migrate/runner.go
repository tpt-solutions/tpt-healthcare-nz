package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner executes SQL migrations embedded in an fs.FS against a pgxpool.Pool.
type Runner struct {
	fs   fs.FS
	pool *pgxpool.Pool
}

// New returns a new Runner backed by the given embed.FS and pgxpool.Pool.
func New(fsys embed.FS, pool *pgxpool.Pool) *Runner {
	return &Runner{fs: fsys, pool: pool}
}

// NewFromDir returns a Runner that reads migrations from a filesystem
// directory. Use this for module-specific migrations that are not embedded.
func NewFromDir(dir string, pool *pgxpool.Pool) *Runner {
	return &Runner{fs: os.DirFS(dir), pool: pool}
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

// Down rolls back n migration steps.
func (r *Runner) Down(ctx context.Context, steps int) error {
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

// Version returns the current migration version, whether it is dirty, and any error.
func (r *Runner) Version(ctx context.Context) (uint, bool, error) {
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
