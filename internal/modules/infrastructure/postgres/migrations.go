package postgres

import "embed"

//go:embed db/migrations/*.sql
var migrations embed.FS
