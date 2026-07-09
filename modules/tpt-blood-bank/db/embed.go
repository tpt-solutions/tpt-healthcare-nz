// Package db provides embedded SQL migrations for the tpt-blood-bank module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
