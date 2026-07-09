// Package db provides embedded SQL migrations for the tpt-nutrition module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
