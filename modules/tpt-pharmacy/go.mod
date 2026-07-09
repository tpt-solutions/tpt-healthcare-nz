module github.com/PhillipC05/tpt-healthcare/modules/tpt-pharmacy

go 1.22

require (
	github.com/PhillipC05/tpt-healthcare/core v0.0.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.19.0
)

replace github.com/PhillipC05/tpt-healthcare/core => ../../core
