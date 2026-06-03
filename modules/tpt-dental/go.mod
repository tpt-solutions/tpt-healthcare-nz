module github.com/PhillipC05/tpt-healthcare/modules/tpt-dental

go 1.22

require (
	github.com/PhillipC05/tpt-healthcare/core v0.0.0
	github.com/google/uuid v1.6.0
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.19.0
)

replace github.com/PhillipC05/tpt-healthcare/core => ../../core