// Package migrations embeds the SQL migration files so the server binary is
// self-contained. Files are applied in lexical order, so name them
// NNNN_description.sql.
package migrations

import "embed"

// FS holds every *.sql file in this directory.
//
//go:embed *.sql
var FS embed.FS
