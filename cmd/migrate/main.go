package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"app/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fail("config: %v", err)
	}

	m, err := newMigrator(cfg)
	if err != nil {
		fail("%v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Fprintf(os.Stderr, "close source: %v\n", srcErr)
		}
		if dbErr != nil {
			fmt.Fprintf(os.Stderr, "close db: %v\n", dbErr)
		}
	}()

	switch os.Args[1] {
	case "up":
		err = m.Up()
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("migrate: already up to date")
			return
		}
	case "down":
		steps := 1
		if len(os.Args) >= 3 {
			steps, err = strconv.Atoi(os.Args[2])
			if err != nil {
				fail("down steps: %v", err)
			}
		}
		err = m.Steps(-steps)
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("migrate: nothing to roll back")
			return
		}
	case "version":
		ver, dirty, vErr := m.Version()
		if errors.Is(vErr, migrate.ErrNilVersion) {
			fmt.Println("migrate: no version (empty)")
			return
		}
		if vErr != nil {
			fail("version: %v", vErr)
		}
		fmt.Printf("version=%d dirty=%v\n", ver, dirty)
		return
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("migrate %s: ok\n", os.Args[1])
}

func newMigrator(cfg *config.Config) (*migrate.Migrate, error) {
	source, err := iofs.New(os.DirFS("migrations"), ".")
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	dbURL := fmt.Sprintf(
		"mysql://%s:%s@tcp(%s:%s)/%s?multiStatements=true&parseTime=true",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	return m, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate <up|down [n]|version>")
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
