// Command tpt-practice is the entrypoint for the tpt-practice module.
// It embeds the frontend assets (served by the tpt-admin app), runs the
// HTTP API server, and manages database migrations.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-practice/api"
)

func main() {
	root := &cobra.Command{
		Use:   "tpt-practice",
		Short: "tpt-practice — Practice Management & Operations module",
	}
	root.AddCommand(serveCmd(), migrateCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			viper.AutomaticEnv()
			ctx := context.Background()
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

			pool, err := db.Connect(ctx, viper.GetString("DATABASE_URL"))
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer pool.Close()

			srv := api.NewServer(api.Config{
				Pool:   pool,
				Logger: logger,
			})

			addr := viper.GetString("LISTEN_ADDR")
			if addr == "" {
				addr = ":8083"
			}
			logger.Info("tpt-practice listening", "addr", addr)
			return http.ListenAndServe(addr, srv)
		},
	}
}

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			viper.AutomaticEnv()
			ctx := context.Background()
			pool, err := db.Connect(ctx, viper.GetString("DATABASE_URL"))
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer pool.Close()
			return db.Migrate(ctx, pool, "")
		},
	}
}
