// Package main is the entry point for the tpt-health-interop CLI.
// It provides subcommands for running the HTTP server, executing database
// migrations, and validating service connectivity.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/PhillipC05/tpt-healthcare/core/terminology"
	"github.com/PhillipC05/tpt-healthcare/interop/api"
	"github.com/PhillipC05/tpt-healthcare/interop/mdns"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	logLevel string
)

// rootCmd is the top-level cobra command. Persistent flags are inherited by
// all subcommands.
var rootCmd = &cobra.Command{
	Use:   "tpt-health-interop",
	Short: "TPT Healthcare NZ interoperability service",
	Long: `tpt-health-interop is the NZ Health Information interoperability gateway.
It exposes FHIR R4/R5, NHI, terminology and subscription APIs for
Te Whatu Ora-connected healthcare applications.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
		} else {
			viper.SetConfigName("tpt-health-interop")
			viper.AddConfigPath("/etc/tpt-healthcare")
			viper.AddConfigPath("$HOME/.tpt-healthcare")
			viper.AddConfigPath(".")
		}
		viper.AutomaticEnv()
		if err := viper.ReadInConfig(); err != nil {
			// Config file is optional; only surface real errors.
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("reading config: %w", err)
			}
		}
		return nil
	},
}

// serveCmd starts the HTTP interop server.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP interoperability server",
	RunE: func(cmd *cobra.Command, args []string) error {
		port := viper.GetInt("port")
		host := viper.GetString("host")

		cfg := api.Config{
			Port:        port,
			Host:        host,
			CORSOrigins: viper.GetStringSlice("cors_origins"),
			RateLimit:   viper.GetFloat64("rate_limit"),
		}
		if cfg.RateLimit == 0 {
			cfg.RateLimit = 100
		}

		srv := api.New(cfg, loadTerminologyStores()...)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Advertise on the local network via mDNS-SD so clinic devices can
		// discover this server automatically (tpt-interop.local:PORT).
		if viper.GetBool("mdns") {
			go func() {
				if err := mdns.Advertise(ctx, slog.Default(), port); err != nil {
					slog.Error("mDNS advertisement error", "err", err)
				}
			}()
		}

		return srv.Start(ctx)
	},
}

// migrateCmd runs database migrations up or down.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		direction := viper.GetString("direction")
		steps := viper.GetInt("steps")

		switch direction {
		case "up", "down":
		default:
			return fmt.Errorf("invalid --direction %q: must be 'up' or 'down'", direction)
		}

		ctx := context.Background()

		dsn := viper.GetString("database_url")
		if dsn == "" {
			dsn = os.Getenv("DATABASE_URL")
		}
		if dsn == "" {
			return fmt.Errorf("database DSN not configured: set DATABASE_URL or database_url in config")
		}

		fmt.Printf("Running migrations %s", direction)
		if direction == "down" && steps > 0 {
			fmt.Printf(" (%d steps)", steps)
		}
		fmt.Println("...")

		runner, err := api.NewMigrationRunner(ctx, dsn)
		if err != nil {
			return fmt.Errorf("initialising migration runner: %w", err)
		}

		if direction == "up" {
			if err := runner.Up(ctx); err != nil {
				return fmt.Errorf("migration up: %w", err)
			}
			fmt.Println("Migrations applied successfully.")
			return nil
		}

		// direction == "down"
		if steps <= 0 {
			steps = 1
		}
		if err := runner.Down(ctx, steps); err != nil {
			return fmt.Errorf("migration down: %w", err)
		}
		fmt.Printf("Rolled back %d migration step(s).\n", steps)
		return nil
	},
}

// validateCmd checks configuration and connectivity for all downstream services.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and service connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		checks := api.RunConnectivityChecks(ctx, api.ConnectivityConfig{
			DatabaseURL: firstNonEmpty(viper.GetString("database_url"), os.Getenv("DATABASE_URL")),
			RedisURL:    firstNonEmpty(viper.GetString("redis_url"), os.Getenv("REDIS_URL")),
			NHIBaseURL:  firstNonEmpty(viper.GetString("nhi_base_url"), os.Getenv("NHI_BASE_URL")),
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SERVICE\tSTATUS\tDETAIL")
		fmt.Fprintln(w, "-------\t------\t------")
		allOK := true
		for _, c := range checks {
			status := "OK"
			if !c.OK {
				status = "FAIL"
				allOK = false
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, status, c.Detail)
		}
		w.Flush()

		if !allOK {
			return fmt.Errorf("one or more connectivity checks failed")
		}
		return nil
	},
}

func init() {
	// Persistent flags on the root command.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default: search /etc/tpt-healthcare, ~/.tpt-healthcare, .)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	_ = viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))

	// serve flags.
	serveCmd.Flags().Int("port", 8080, "TCP port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host address to bind to")
	serveCmd.Flags().Bool("mdns", false, "Advertise service on the local network via mDNS-SD (tpt-interop.local)")
	_ = viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("host", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("mdns", serveCmd.Flags().Lookup("mdns"))

	// migrate flags.
	migrateCmd.Flags().String("direction", "up", "Migration direction: up or down")
	migrateCmd.Flags().Int("steps", 0, "Number of migration steps (only for down; 0 means 1)")
	_ = viper.BindPFlag("direction", migrateCmd.Flags().Lookup("direction"))
	_ = viper.BindPFlag("steps", migrateCmd.Flags().Lookup("steps"))

	rootCmd.AddCommand(serveCmd, migrateCmd, validateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// firstNonEmpty returns the first non-empty string from the provided values.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// loadTerminologyStores loads terminology data files configured via viper
// and returns a WithTermStore option. Returns nil if no data files are configured.
func loadTerminologyStores() []api.ServerOption {
	snomed, _ := terminology.LoadSNOMEDCSV(viper.GetString("snomed_csv"))
	loinc, _ := terminology.LoadLOINC(viper.GetString("loinc_csv"))
	icd10, _ := terminology.LoadICD10AM(viper.GetString("icd10_csv"))
	nzmt, _ := terminology.LoadNZMT(viper.GetString("nzmt_csv"))

	if snomed == nil && loinc == nil && icd10 == nil && nzmt == nil {
		return nil
	}

	slog.Info("loaded terminology stores",
		"snomed", snomed != nil,
		"loinc", loinc != nil,
		"icd10", icd10 != nil,
		"nzmt", nzmt != nil,
	)
	return []api.ServerOption{api.WithTermStore(api.NewCoreTermStore(snomed, loinc, icd10, nzmt))}
}
