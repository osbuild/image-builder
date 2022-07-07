package db

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
)

func Migrate(connStr string, migrationsDir string) error {
	m, err := prepare(connStr, migrationsDir)
	defer cleanup(m)
	if err != nil {
		return err
	}
	return finish(m, m.Up())
}

func MigrateSteps(connStr string, migrationsDir string, steps int) error {
	m, err := prepare(connStr, migrationsDir)
	defer cleanup(m)
	if err != nil {
		return err
	}
	return finish(m, m.Steps(steps))
}

func prepare(connStr string, migrationsDir string) (*migrate.Migrate, error) {
	logrus.Infoln("Applying migrations from directory ", fmt.Sprintf("file://%s", migrationsDir))
	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsDir), connStr)
	if err != nil {
		return nil, err
	}

	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		logrus.Infoln("No migration has been aplied, this is the first one")
	} else if err != nil {
		return nil, err
	}
	logrus.Infoln("Current migration version", version)

	if dirty {
		logrus.Errorln("Current migration state is dirty")
		return m, fmt.Errorf("DB is at schema version %v is dirty", version)
	}
	return m, nil
}

func finish(m *migrate.Migrate, err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		logrus.Infoln("No migrations were applied, already at latest version")
	} else if err != nil {
		return err
	}
	version, _, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	logrus.Infoln("Migrated to version ", version)
	return nil
}

func cleanup(m *migrate.Migrate) {
	if m == nil {
		return
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		logrus.Errorln("Source error when closing migration", srcErr)
	}
	if dbErr != nil {
		logrus.Errorln("DB error when closing migration", dbErr)
	}
}
