// Package db provides embedded SQL migrations for the tpt-health-billing module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
