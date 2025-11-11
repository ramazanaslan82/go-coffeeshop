//go:build migrate

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"golang.org/x/exp/slog"

	// migrate tools
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const (
	_defaultAttempts = 5
	_defaultTimeout  = time.Second
)

var (
	_migrationFilePath = "db/migrations"
)

func init() {
	databaseURL, ok := os.LookupEnv("PG_URL")
	if !ok || len(databaseURL) == 0 {
		slog.Error("migrate: environment variable not declared: PG_URL", fmt.Errorf("PG_URL not set"))
		os.Exit(2)
	}

	databaseURL += "?sslmode=disable"

	var (
		attempts = _defaultAttempts
		err      error
		m        *migrate.Migrate
	)

	for attempts > 0 {
		inDocker, _ := os.LookupEnv("IN_DOCKER")

		dir := "file:///db/migrations"
		if dockered, _ := strconv.ParseBool(inDocker); inDocker != "" && !dockered {
			cur, _ := os.Getwd()
			dir = fmt.Sprintf("file://%s/%s", filepath.Dir(cur+"/../../.."), _migrationFilePath)
		}

		slog.Info("migration source", "url", dir)
		m, err = migrate.New(dir, databaseURL)
		if err == nil {
			break
		}

		slog.Info("Migrate: postgres is trying to connect", "attempts_left", attempts)
		time.Sleep(_defaultTimeout)
		attempts--
	}

	if err != nil {
		slog.Error("Migrate: postgres connect error", err)
		os.Exit(2)
	}

	err = m.Up()
	defer m.Close()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("Migrate: up error", err)
		os.Exit(2)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("Migrate: no change")
		return
	}

	slog.Info("Migrate: up success")
}
