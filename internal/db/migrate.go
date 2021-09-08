package db

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
)

func Migrate(connStr string, migrationsDir string, logger *logrus.Logger) error {
	m, err := prepare(connStr, migrationsDir, logger)
	defer cleanup(m, logger)
	if err != nil {
		return err
	}
	return finish(m, m.Up(), logger)
}

func MigrateSteps(connStr string, migrationsDir string, steps int, logger *logrus.Logger) error {
	m, err := prepare(connStr, migrationsDir, logger)
	defer cleanup(m, logger)
	if err != nil {
		return err
	}
	return finish(m, m.Steps(steps), logger)
}

func prepare(connStr string, migrationsDir string, logger *logrus.Logger) (*migrate.Migrate, error) {
	logger.Infoln("Applying migrations from directory ", fmt.Sprintf("file://%s", migrationsDir))
	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsDir), connStr)
	if err != nil {
		return nil, err
	}

	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		logger.Infoln("No migration has been aplied, this is the first one")
	} else if err != nil {
		return nil, err
	}
	logger.Infoln("Current migration version", version)

	if dirty {
		logger.Warnln("Current migration state is dirty, force resetting to version", version)
		err = m.Force(int(version))
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func finish(m *migrate.Migrate, err error, logger *logrus.Logger) error {
	if errors.Is(err, migrate.ErrNoChange) {
		logger.Infoln("No migrations were applied, already at latest version")
	} else if err != nil {
		return err
	}
	version, _, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	logger.Infoln("Migrated to version ", version)
	return nil
}

func cleanup(m *migrate.Migrate, logger *logrus.Logger) {
	if m == nil {
		return
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		logger.Errorln("Source error when closing migration", srcErr)
	}
	if dbErr != nil {
		logger.Errorln("DB error when closing migration", dbErr)
	}
}
