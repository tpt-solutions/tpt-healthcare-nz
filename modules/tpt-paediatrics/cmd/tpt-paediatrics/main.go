// Package main is the entrypoint for the tpt-paediatrics CLI.
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

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-paediatrics/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tpt-paediatrics",
	Short: "tpt-paediatrics — paediatric management service for NZ",
}

func init() {
	cobra.OnInitialize(func() {
		viper.SetConfigName("tpt-paediatrics")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/tpt")
		viper.SetEnvPrefix("TPT_PAEDIATRICS")
		viper.AutomaticEnv()
		_ = viper.ReadInConfig()
	})
	rootCmd.AddCommand(serveCmd, migrateCmd)
	serveCmd.Flags().String("host", "0.0.0.0", "host address to bind")
	serveCmd.Flags().Int("port", 8101, "port to listen on")
	_ = viper.BindPFlag("server.host", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the tpt-paediatrics HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		srv, err := api.NewServer(api.Config{
			DatabaseURL:   viper.GetString("database.url"),
			RedisURL:      viper.GetString("redis.url"),
			EncryptionKey: viper.GetString("encryption.key"),
			Auth0Domain:   viper.GetString("auth0.domain"),
			Auth0Audience: viper.GetString("auth0.audience"),
			TenantHeader:  viper.GetString("tenant.header"),
			Logger:        logger,
		})
		if err != nil {
			return fmt.Errorf("create server: %w", err)
		}
		addr := fmt.Sprintf("%s:%d", viper.GetString("server.host"), viper.GetInt("server.port"))
		httpSrv := &http.Server{Addr: addr, Handler: srv.Handler(), ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second}
		go func() {
			logger.Info("tpt-paediatrics server starting", slog.String("addr", addr))
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("server error", slog.Any("error", err))
				os.Exit(1)
			}
		}()
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return httpSrv.Shutdown(ctx)
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		return api.RunMigrations(context.Background(), viper.GetString("database.url"), logger)
	},
}
