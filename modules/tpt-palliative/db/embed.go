// Package db provides the embedded SQL migrations for the tpt-palliative module.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
