module github.com/PhillipC05/tpt-healthcare/core

go 1.22.0

require (
	github.com/SherClockHolmes/webpush-go v1.3.0
	github.com/gorilla/websocket v1.5.3
	github.com/coreos/go-oidc/v3 v3.11.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/golang-migrate/migrate/v4 v4.18.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/pquerna/otp v1.4.0
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.7.0
	github.com/riverqueue/river v0.14.2
	github.com/riverqueue/river/riverdriver/riverpgxv5 v0.14.2
	github.com/sony/gobreaker v1.0.0
	github.com/spf13/viper v1.19.0
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go v0.35.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.58.0
	go.opentelemetry.io/otel v1.33.0
	golang.org/x/crypto v0.31.0
	golang.org/x/time v0.9.0
)

require golang.org/x/oauth2 v0.21.0 // indirect
