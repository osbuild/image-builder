package db

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
)

func Migrate(connStr string, migrationsDir string, logger *logrus.Logger) error {
	fmt.Println(migrationsDir, fmt.Sprintf("file://%s", migrationsDir))
	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsDir), connStr)
	if err != nil {
		return err
	}

	version, dirty, err := m.Version()
	if err == migrate.ErrNilVersion {
		logger.Infoln("No migration has been aplied, this is the first one")
	} else if err != nil {
		return err
	}
	logger.Infoln("Current migration version", version)

	if dirty {
		logger.Warnln("Current migration state is dirty, force resetting to version")
		err = m.Force(int(version))
		if err != nil {
			return err
		}
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		logger.Infoln("No change to migrate up to")
	} else if err != nil {
		return err
	}

	version, _, err = m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return err
	}
	logger.Infoln("Migrated to version ", version)

	srcErr, dbErr := m.Close()
	if srcErr != nil {
		logger.Errorln("Source error when closing migration", srcErr)
	}
	if dbErr != nil {
		logger.Errorln("DB error when closing migration", dbErr)
	}

	return nil
}
