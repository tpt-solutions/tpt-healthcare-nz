// Package main is the entrypoint for the tpt-massage CLI.
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

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-massage/api"
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
	Use:   "tpt-massage",
	Short: "tpt-massage — Massage therapy practice management for NZ",
	Long:  "ACC registered massage therapy, SOAP notes, contraindication screening, and treatment records.",
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: tpt-massage.yaml)")
	rootCmd.AddCommand(serveCmd, migrateCmd, validateCmd)
	serveCmd.Flags().StringVar(&host, "host", "0.0.0.0", "bind address")
	serveCmd.Flags().IntVar(&port, "port", 8093, "listen port")
	_ = viper.BindPFlag("server.host", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("tpt-massage")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/tpt")
		viper.AddConfigPath("$HOME/.tpt")
	}
	viper.SetEnvPrefix("TPT_MASSAGE")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		}
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		srv, err := api.NewServer(api.Config{
			Host: viper.GetString("server.host"), Port: viper.GetInt("server.port"),
			DatabaseURL: viper.GetString("database.url"), RedisURL: viper.GetString("redis.url"),
			EncryptionKey: viper.GetString("encryption.key"), Auth0Domain: viper.GetString("auth0.domain"),
			Auth0Audience: viper.GetString("auth0.audience"), TenantHeader: viper.GetString("tenant.header"),
			Logger: logger,
		})
		if err != nil {
			return fmt.Errorf("create server: %w", err)
		}
		addr := fmt.Sprintf("%s:%d", viper.GetString("server.host"), viper.GetInt("server.port"))
		httpSrv := &http.Server{
			Addr: addr, Handler: srv.Handler(),
			ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
		}
		go func() {
			logger.Info("tpt-massage server starting", slog.String("addr", addr))
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
		return httpSrv.Shutdown(ctx)
	},
}

var migrateCmd = &cobra.Command{
	Use: "migrate", Short: "Run migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		dbURL := viper.GetString("database.url")
		if dbURL == "" {
			return fmt.Errorf("database.url required (set TPT_MASSAGE_DATABASE_URL)")
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
	Use: "validate", Short: "Validate configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		checks := []struct{ n, k string }{
			{"database URL", "database.url"}, {"redis URL", "redis.url"},
			{"encryption key", "encryption.key"}, {"auth0 domain", "auth0.domain"},
			{"auth0 audience", "auth0.audience"},
		}
		allOK := true
		for _, c := range checks {
			if viper.GetString(c.k) == "" {
				logger.Warn("missing configuration", slog.String("key", c.n))
				allOK = false
			}
		}
		if !allOK {
			return fmt.Errorf("configuration validation failed")
		}
		if err := api.ValidateConnectivity(context.Background(), api.Config{
			DatabaseURL: viper.GetString("database.url"), RedisURL: viper.GetString("redis.url"), Logger: logger,
		}); err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
		logger.Info("all checks passed")
		return nil
	},
}
