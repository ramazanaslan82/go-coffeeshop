//go:build migrate

package app

import (
	"errors"
	"fmt"
	"strconv"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"golang.org/x/exp/slog"

	// migrate tools
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const (
	_defaultAttempts   = 5
	_defaultTimeout    = time.Second
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
		// Prefer absolute path in containers; fallback to repo-relative path for local runs
		sourceURL := "file:///db/migrations"
		if inDocker := os.Getenv("IN_DOCKER"); inDocker != "" {
			if dockered, _ := strconv.ParseBool(inDocker); !dockered {
				cur, _ := os.Getwd()
				sourceURL = fmt.Sprintf("file://%s/%s", filepath.Dir(cur+"/../../.."), _migrationFilePath)
			}
		} else {
			cur, _ := os.Getwd()
			sourceURL = fmt.Sprintf("file://%s/%s", filepath.Dir(cur+"/../../.."), _migrationFilePath)
		}

		slog.Info("migration source", "url", sourceURL)

		m, err = migrate.New(sourceURL, databaseURL)
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
