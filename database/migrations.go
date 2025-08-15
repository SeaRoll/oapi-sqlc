package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func migrate(db *sql.DB) error {
	// setup database connection
	goose.SetBaseFS(embedMigrations)

	err := goose.SetDialect("postgres")
	if err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	err = goose.Up(db, "migrations")
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
