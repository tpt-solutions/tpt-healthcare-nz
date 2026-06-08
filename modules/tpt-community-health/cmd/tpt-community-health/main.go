// Package main is the entrypoint for the tpt-community-health CLI.
// Subcommands: serve, migrate, validate.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	host    string
	port    int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tpt-community-health",
	Short: "tpt-community-health — District nursing, home visits, and outreach for NZ",
	Long: `tpt-community-health provides community health workflows including
home visit scheduling and documentation, district nursing care plans,
and community outreach program management for the tpt-healthcare NZ platform.`,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: tpt-community-health.yaml in current directory or /etc/tpt)")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(validateCmd)

	serveCmd.Flags().StringVar(&host, "host", "0.0.0.0", "host address to bind")
	serveCmd.Flags().IntVar(&port, "port", 8092, "port to listen on")
	serveCmd.Flags().String("config", "", "path to config file")

	_ = viper.BindPFlag("server.host", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("tpt-community-health")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/tpt")
		viper.AddConfigPath("$HOME/.tpt")
	}

	viper.SetEnvPrefix("TPT_COMMUNITY_HEALTH")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		}
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the tpt-community-health HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		cfg := api.Config{
			Addr:          fmt.Sprintf("%s:%d", viper.GetString("server.host"), viper.GetInt("server.port")),
			ReadTimeout:   15,
			WriteTimeout:  15,
			IdleTimeout:   60,
			DatabaseURL:   viper.GetString("database.url"),
			Auth0Domain:   viper.GetString("auth0.domain"),
			Auth0Audience: viper.GetString("auth0.audience"),
			Logger:        logger,
		}

		if cfg.DatabaseURL == "" {
			return fmt.Errorf("database.url is required (set TPT_COMMUNITY_HEALTH_DATABASE_URL)")
		}

		if cfg.AllowedOrigins == nil {
			cfg.AllowedOrigins = []string{"*"}
		}

		srv, err := api.NewServer(cfg)
		if err != nil {
			return fmt.Errorf("create server: %w", err)
		}

		httpSrv := &http.Server{
			Addr:         cfg.Addr,
			Handler:      srv.Handler(),
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
		}

		go func() {
			logger.Info("tpt-community-health server starting", slog.String("addr", cfg.Addr))
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("server error", slog.Any("error", err))
				os.Exit(1)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		logger.Info("shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		logger.Info("server stopped")
		return nil
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		dbURL := viper.GetString("database.url")
		if dbURL == "" {
			return fmt.Errorf("database.url is required (set TPT_COMMUNITY_HEALTH_DATABASE_URL)")
		}

		logger.Info("running migrations", slog.String("database_url", dbURL))
		if err := api.RunMigrations(context.Background(), dbURL, logger); err != nil {
			return fmt.Errorf("migrations failed: %w", err)
		}
		logger.Info("migrations complete")
		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		logger.Info("validating tpt-community-health configuration")

		checks := []struct {
			name string
			key  string
		}{
			{"database URL", "database.url"},
			{"auth0 domain", "auth0.domain"},
			{"auth0 audience", "auth0.audience"},
		}

		allOK := true
		for _, c := range checks {
			val := viper.GetString(c.key)
			if val == "" {
				logger.Warn("missing configuration", slog.String("key", c.name))
				allOK = false
			} else {
				logger.Info("configuration OK", slog.String("key", c.name))
			}
		}

		if !allOK {
			return fmt.Errorf("configuration validation failed — see warnings above")
		}

		if err := api.ValidateConnectivity(context.Background(), api.Config{
			DatabaseURL: viper.GetString("database.url"),
			Logger:      logger,
		}); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}

		logger.Info("all checks passed")
		return nil
	},
}
