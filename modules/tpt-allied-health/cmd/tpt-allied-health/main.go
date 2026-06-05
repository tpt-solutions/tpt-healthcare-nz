// Package main is the entry point for tpt-allied-health.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/auth/jwt"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func main() {
	// Parse command line flags
	var (
		configFile = flag.String("config", "", "Path to config file")
		migrate    = flag.Bool("migrate", false, "Run database migrations")
		version    = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	if *version {
		log.Info().Msg("tpt-allied-health v0.1.0")
		os.Exit(0)
	}

	// Load configuration
	cfg := loadConfig(*configFile)

	// Initialize database
	dbPool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbPool.Close()

	// Run migrations if requested
	if *migrate {
		if err := db.Migrate(dbPool, "modules/tpt-allied-health/db/migrate"); err != nil {
			log.Fatal().Err(err).Msg("Failed to run migrations")
		}
		log.Info().Msg("Migrations completed successfully")
		os.Exit(0)
	}

	// Initialize auth provider
	authProvider, err := initAuthProvider(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize auth provider")
	}

	// Create and start server
	serverCfg := api.Config{
		Addr:         cfg.ServerAddr,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	server := api.NewServer(authProvider, serverCfg)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("Shutdown signal received")
		cancel()
	}()

	// Start server
	if err := server.Start(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}

	<-ctx.Done()
	log.Info().Msg("tpt-allied-health stopped")
}

// Config holds application configuration.
type Config struct {
	DatabaseURL  string
	ServerAddr   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Auth configuration
	AuthMode     string // "auth0", "jwt", "oidc"
	Auth0Domain  string
	Auth0Audience string
	JWTSecret    string
	JWTIssuer    string
	OIDCIssuer   string
}

// loadConfig loads configuration from file and environment.
func loadConfig(configFile string) Config {
	viper.SetConfigName("tpt-allied-health")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/tpt-allied-health")

	if configFile != "" {
		viper.SetConfigFile(configFile)
	}

	// Environment variable bindings
	viper.AutomaticEnv()
	viper.BindEnv("database_url", "DATABASE_URL")
	viper.BindEnv("server_addr", "SERVER_ADDR")
	viper.BindEnv("auth_mode", "AUTH_MODE")
	viper.BindEnv("auth0_domain", "AUTH0_DOMAIN")
	viper.BindEnv("auth0_audience", "AUTH0_AUDIENCE")
	viper.BindEnv("jwt_secret", "JWT_SECRET")
	viper.BindEnv("jwt_issuer", "JWT_ISSUER")
	viper.BindEnv("oidc_issuer", "OIDC_ISSUER")

	// Defaults
	viper.SetDefault("server_addr", ":8080")
	viper.SetDefault("read_timeout", "15s")
	viper.SetDefault("write_timeout", "15s")
	viper.SetDefault("idle_timeout", "60s")
	viper.SetDefault("auth_mode", "jwt")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Warn().Err(err).Msg("Error reading config file, using defaults and env vars")
		}
	}

	readTimeout, _ := time.ParseDuration(viper.GetString("read_timeout"))
	writeTimeout, _ := time.ParseDuration(viper.GetString("write_timeout"))
	idleTimeout, _ := time.ParseDuration(viper.GetString("idle_timeout"))

	return Config{
		DatabaseURL:  viper.GetString("database_url"),
		ServerAddr:   viper.GetString("server_addr"),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		AuthMode:     viper.GetString("auth_mode"),
		Auth0Domain:  viper.GetString("auth0_domain"),
		Auth0Audience: viper.GetString("auth0_audience"),
		JWTSecret:    viper.GetString("jwt_secret"),
		JWTIssuer:    viper.GetString("jwt_issuer"),
		OIDCIssuer:   viper.GetString("oidc_issuer"),
	}
}

// initAuthProvider initializes the authentication provider based on config.
func initAuthProvider(cfg Config) (auth.Provider, error) {
	switch cfg.AuthMode {
	case "auth0":
		if cfg.Auth0Domain == "" || cfg.Auth0Audience == "" {
			return nil, auth.ErrInvalidConfig
		}
		return auth0.NewProvider(cfg.Auth0Domain, cfg.Auth0Audience)

	case "jwt":
		if cfg.JWTSecret == "" {
			return nil, auth.ErrInvalidConfig
		}
		return jwt.NewProvider(cfg.JWTSecret, cfg.JWTIssuer)

	case "oidc":
		if cfg.OIDCIssuer == "" {
			return nil, auth.ErrInvalidConfig
		}
		// OIDC provider would be implemented here
		return nil, auth.ErrNotImplemented

	default:
		return nil, auth.ErrInvalidConfig
	}
}