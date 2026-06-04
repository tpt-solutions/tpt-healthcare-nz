// Command tpt-aged-care is the entrypoint for the tpt-aged-care service.
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

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-aged-care/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "tpt-aged-care",
	Short: "TPT Aged Care service — interRAI, NASC, funded hours, and care plans",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	RunE:  runServe,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply database migrations",
	RunE:  runMigrate,
}

func init() {
	viper.AutomaticEnv()

	serveCmd.Flags().String("host", "0.0.0.0", "Listen host")
	serveCmd.Flags().Int("port", 8086, "Listen port")
	_ = viper.BindPFlag("HOST", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("PORT", serveCmd.Flags().Lookup("port"))

	rootCmd.AddCommand(serveCmd, migrateCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := api.Config{
		Host:           viper.GetString("HOST"),
		Port:           viper.GetInt("PORT"),
		DatabaseURL:    mustEnv("DATABASE_URL"),
		RedisURL:       viper.GetString("REDIS_URL"),
		EncryptionKey:  mustEnv("ENCRYPTION_KEY"),
		Auth0Domain:    mustEnv("AUTH0_DOMAIN"),
		Auth0Audience:  mustEnv("AUTH0_AUDIENCE"),
		Auth0ClientID:  mustEnv("AUTH0_CLIENT_ID"),
		HPIBaseURL:     viper.GetString("HPI_BASE_URL"),
		CORSOrigins:    viper.GetStringSlice("CORS_ORIGINS"),
		RateLimitRPS:   viper.GetFloat64("RATE_LIMIT_RPS"),
		RateLimitBurst: viper.GetInt("RATE_LIMIT_BURST"),
		Logger:         logger,
	}

	srv, err := api.NewServer(cfg)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("tpt-aged-care listening", slog.String("addr", addr))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpSrv.Shutdown(ctx)
}

func runMigrate(_ *cobra.Command, _ []string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return api.RunMigrations(context.Background(), mustEnv("DATABASE_URL"), logger)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
