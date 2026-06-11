// Package main is the entrypoint for the tpt-epidemiology CLI.
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

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-epidemiology/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tpt-epidemiology",
	Short: "tpt-epidemiology — disease surveillance, outbreak investigation, and public health reporting for NZ",
	Long: `tpt-epidemiology manages notifiable disease case reporting to EpiSurv/ESR,
outbreak investigation workflows, and aggregate public health surveillance
reports for the tpt-healthcare NZ platform.`,
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: tpt-epidemiology.yaml in current directory or /etc/tpt)")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(validateCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("tpt-epidemiology")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/tpt")
		viper.AddConfigPath("$HOME/.tpt")
	}
	viper.SetEnvPrefix("TPT_EPIDEMIOLOGY")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		}
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the tpt-epidemiology HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		port, _ := cmd.Flags().GetInt("port")
		cfg := api.Config{
			Host:          viper.GetString("server.host"),
			Port:          port,
			DatabaseURL:   viper.GetString("database.url"),
			RedisURL:      viper.GetString("redis.url"),
			EncryptionKey: viper.GetString("encryption.key"),
			Auth0Domain:   viper.GetString("auth0.domain"),
			Auth0Audience: viper.GetString("auth0.audience"),
			TenantHeader:  viper.GetString("server.tenant_header"),
			Logger:        logger,
		}
		if cfg.Host == "" {
			cfg.Host = "0.0.0.0"
		}
		if cfg.DatabaseURL == "" {
			return fmt.Errorf("database.url is required (set TPT_EPIDEMIOLOGY_DATABASE_URL)")
		}

		srv, err := api.NewServer(cfg)
		if err != nil {
			return fmt.Errorf("create server: %w", err)
		}

		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		httpSrv := &http.Server{
			Addr:         addr,
			Handler:      srv.Handler(),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		go func() {
			logger.Info("tpt-epidemiology server starting", slog.String("addr", addr))
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
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		dbURL := viper.GetString("database.url")
		if dbURL == "" {
			return fmt.Errorf("database.url is required (set TPT_EPIDEMIOLOGY_DATABASE_URL)")
		}
		logger.Info("running migrations")
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
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		logger.Info("validating tpt-epidemiology configuration")

		checks := []struct{ name, key string }{
			{"database URL", "database.url"},
			{"auth0 domain", "auth0.domain"},
			{"auth0 audience", "auth0.audience"},
			{"encryption key", "encryption.key"},
		}
		allOK := true
		for _, c := range checks {
			if viper.GetString(c.key) == "" {
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

func init() {
	serveCmd.Flags().Int("port", 8104, "port to listen on")
}
