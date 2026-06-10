// Package db provides the embedded SQL migrations for the tpt-epidemiology module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
