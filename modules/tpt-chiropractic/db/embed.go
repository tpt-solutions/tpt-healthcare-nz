// Package db provides embedded SQL migrations for the tpt-chiropractic module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
