package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		dsn             string
		migrationsPath  string
		migrationsTable string
		command         string
	)

	flag.StringVar(&dsn, "dsn", "", "postgres DSN")
	flag.StringVar(&migrationsPath, "migrations-path", "./migrations", "path to migrations")
	flag.StringVar(&migrationsTable, "migrations-table", "schema_migrations", "migrations table name")
	flag.StringVar(&command, "command", "up", "migration command: up|down|version")
	flag.Parse()

	if dsn == "" {
		log.Println("dsn is required")
		os.Exit(1)
	}

	sourceURL := "file://" + migrationsPath
	databaseURL, err := withMigrationsTable(dsn, migrationsTable)
	if err != nil {
		log.Printf("build database url: %v\n", err)
		os.Exit(1)
	}

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		log.Printf("create migrator: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "up":
		err = m.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Printf("apply up migrations: %v\n", err)
			os.Exit(1)
		}
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no migrations to apply")
			return
		}
		fmt.Println("migrations applied successfully")

	case "down":
		err = m.Down()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Printf("apply down migrations: %v\n", err)
			os.Exit(1)
		}
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no migrations to rollback")
			return
		}
		fmt.Println("migrations rolled back successfully")

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("no migrations applied yet")
				return
			}
			log.Printf("get version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)

	default:
		log.Printf("unknown command: %s\n", command)
		os.Exit(1)
	}
}

func withMigrationsTable(dsn, table string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("x-migrations-table", table)
	u.RawQuery = q.Encode()

	return u.String(), nil
}
