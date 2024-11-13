package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetReportCaller(true)

	conf := Config{
		DryRun:                true,
		EnableDBMaintenance:   false,
		ClonesRetentionMonths: 24,
	}

	err := LoadConfigFromEnv(&conf)
	if err != nil {
		logrus.Fatal(err)
	}

	if conf.DryRun {
		logrus.Info("Dry run, no state will be changed")
	}

	if !conf.EnableDBMaintenance {
		logrus.Info("🦀🦀🦀 DB maintenance not enabled, skipping  🦀🦀🦀")
		return
	}
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		conf.PGUser,
		conf.PGPassword,
		conf.PGHost,
		conf.PGPort,
		conf.PGDatabase,
		conf.PGSSLMode,
	)
	err = DBCleanup(dbURL, conf.DryRun, conf.ClonesRetentionMonths)
	if err != nil {
		logrus.Fatalf("Error during DBCleanup: %v", err)
	}
	logrus.Info("🦀🦀🦀 dbqueue cleanup done 🦀🦀🦀")
}
