package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-health-billing/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "tpt-health-billing",
		Short: "TPT Health Billing — cross-module ACC claiming, PHARMAC subsidies, and health insurance",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newMigrateCmd())
	root.AddCommand(newValidateCmd())

	return root
}

func newServeCmd() *cobra.Command {
	var (
		host string
		port int
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the tpt-health-billing HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.SetDefault("host", host)
			viper.SetDefault("port", port)
			viper.AutomaticEnv()

			cfg := api.Config{
				Host:          viper.GetString("host"),
				Port:          viper.GetInt("port"),
				DatabaseURL:   viper.GetString("DATABASE_URL"),
				RedisURL:      viper.GetString("REDIS_URL"),
				EncryptionKey: viper.GetString("ENCRYPTION_KEY"),
				ACCBaseURL:    viper.GetString("ACC_BASE_URL"),
				PHARMACBaseURL: viper.GetString("PHARMAC_BASE_URL"),
			}

			return api.Start(context.Background(), cfg)
		},
	}

	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "Host address to bind")
	cmd.Flags().IntVar(&port, "port", 8090, "Port to listen on")

	return cmd
}

func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.AutomaticEnv()
			databaseURL := viper.GetString("DATABASE_URL")
			if databaseURL == "" {
				return fmt.Errorf("migrate: DATABASE_URL environment variable is required")
			}
			return api.RunMigrations(context.Background(), databaseURL)
		},
	}
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.AutomaticEnv()
			cfg := api.Config{
				Host:          viper.GetString("host"),
				Port:          viper.GetInt("port"),
				DatabaseURL:   viper.GetString("DATABASE_URL"),
				RedisURL:      viper.GetString("REDIS_URL"),
				EncryptionKey: viper.GetString("ENCRYPTION_KEY"),
				ACCBaseURL:    viper.GetString("ACC_BASE_URL"),
				PHARMACBaseURL: viper.GetString("PHARMAC_BASE_URL"),
			}
			return api.Validate(context.Background(), cfg)
		},
	}
}
