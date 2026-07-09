// Package db provides embedded SQL migrations for the tpt-immunisation module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
