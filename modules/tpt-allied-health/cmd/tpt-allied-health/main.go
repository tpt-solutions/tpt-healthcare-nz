// Package main is the entry point for tpt-allied-health.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/auth/jwt"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/api"
	"github.com/spf13/viper"
)

func main() {
	var (
		configFile = flag.String("config", "", "Path to config file")
		migrate    = flag.Bool("migrate", false, "Run database migrations")
		version    = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if *version {
		logger.Info("tpt-allied-health v0.1.0")
		os.Exit(0)
	}

	cfg := loadConfig(*configFile)
	ctx := context.Background()

	dbPool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer dbPool.Close()

	if *migrate {
		if err := db.Migrate(ctx, dbPool, "modules/tpt-allied-health/db/migrate"); err != nil {
			logger.Error("failed to run migrations", slog.Any("error", err))
			os.Exit(1)
		}
		logger.Info("migrations completed successfully")
		os.Exit(0)
	}

	authProvider, err := initAuthProvider(cfg)
	if err != nil {
		logger.Error("failed to initialize auth provider", slog.Any("error", err))
		os.Exit(1)
	}

	auditTrail := audit.New(dbPool)
	consentStore := consent.NewStore(dbPool)

	// HPI client is optional — nil disables APC validation (development only).
	// Set HPI_BASE_URL to enable.
	var hpiClient *hpi.Client
	if cfg.HPIBaseURL != "" {
		hpiClient = hpi.New(cfg.HPIBaseURL, func(ctx context.Context) (string, error) {
			// TODO: implement SMART on FHIR client-credentials token fetch.
			return "", nil
		}, nil)
	} else {
		logger.Warn("HPI_BASE_URL not set — APC validation is disabled; do not use in production")
	}

	serverCfg := api.Config{
		Addr:           cfg.ServerAddr,
		ReadTimeout:    int(cfg.ReadTimeout.Seconds()),
		WriteTimeout:   int(cfg.WriteTimeout.Seconds()),
		IdleTimeout:    int(cfg.IdleTimeout.Seconds()),
		AllowedOrigins: cfg.AllowedOrigins,
		Logger:         logger,
	}

	server := api.NewServer(dbPool, authProvider, auditTrail, hpiClient, consentStore, serverCfg)

	httpSrv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      server.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", slog.Any("error", err))
		}
	}()

	logger.Info("tpt-allied-health server starting", slog.String("addr", cfg.ServerAddr))
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("tpt-allied-health stopped")
}

// Config holds application configuration.
type Config struct {
	DatabaseURL    string
	ServerAddr     string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	AllowedOrigins []string

	AuthMode          string
	Auth0Domain       string
	Auth0Audience     string
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	OIDCIssuer        string

	HPIBaseURL string
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

	viper.AutomaticEnv()
	viper.BindEnv("database_url", "DATABASE_URL")
	viper.BindEnv("server_addr", "SERVER_ADDR")
	viper.BindEnv("auth_mode", "AUTH_MODE")
	viper.BindEnv("auth0_domain", "AUTH0_DOMAIN")
	viper.BindEnv("auth0_audience", "AUTH0_AUDIENCE")
	viper.BindEnv("jwt_private_key_path", "JWT_PRIVATE_KEY_PATH")
	viper.BindEnv("jwt_public_key_path", "JWT_PUBLIC_KEY_PATH")
	viper.BindEnv("oidc_issuer", "OIDC_ISSUER")
	viper.BindEnv("hpi_base_url", "HPI_BASE_URL")
	viper.BindEnv("allowed_origins", "ALLOWED_ORIGINS")

	viper.SetDefault("server_addr", ":8080")
	viper.SetDefault("read_timeout", "15s")
	viper.SetDefault("write_timeout", "15s")
	viper.SetDefault("idle_timeout", "60s")
	viper.SetDefault("auth_mode", "jwt")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Default().Warn("error reading config file, using defaults and env vars", slog.Any("error", err))
		}
	}

	readTimeout, _ := time.ParseDuration(viper.GetString("read_timeout"))
	writeTimeout, _ := time.ParseDuration(viper.GetString("write_timeout"))
	idleTimeout, _ := time.ParseDuration(viper.GetString("idle_timeout"))

	allowedOrigins := viper.GetStringSlice("allowed_origins")
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	return Config{
		DatabaseURL:    viper.GetString("database_url"),
		ServerAddr:     viper.GetString("server_addr"),
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		IdleTimeout:    idleTimeout,
		AllowedOrigins: allowedOrigins,
		AuthMode:          viper.GetString("auth_mode"),
		Auth0Domain:       viper.GetString("auth0_domain"),
		Auth0Audience:     viper.GetString("auth0_audience"),
		JWTPrivateKeyPath: viper.GetString("jwt_private_key_path"),
		JWTPublicKeyPath:  viper.GetString("jwt_public_key_path"),
		OIDCIssuer:        viper.GetString("oidc_issuer"),
		HPIBaseURL:        viper.GetString("hpi_base_url"),
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
		if cfg.JWTPrivateKeyPath == "" || cfg.JWTPublicKeyPath == "" {
			return nil, auth.ErrInvalidConfig
		}
		return jwt.NewFromFiles(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath)

	case "oidc":
		if cfg.OIDCIssuer == "" {
			return nil, auth.ErrInvalidConfig
		}
		return nil, auth.ErrNotImplemented

	default:
		return nil, auth.ErrInvalidConfig
	}
}
