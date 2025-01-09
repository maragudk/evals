package sql

import (
	"context"
	"embed"
	"io/fs"

	"maragu.dev/migrate"
)

//go:embed migrations
var migrations embed.FS

func (d *Helper) MigrateUp(ctx context.Context) error {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	return migrate.Up(ctx, d.DB.DB, fsys)
}

func (d *Helper) MigrateDown(ctx context.Context) error {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	return migrate.Down(ctx, d.DB.DB, fsys)
}
