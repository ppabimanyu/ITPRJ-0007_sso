// Package migrations embeds the goose SQL files so the binary can migrate
// without shipping a second artifact.
package migrations

import "embed"

// FS holds every goose migration in this directory.
//
//go:embed *.sql
var FS embed.FS
