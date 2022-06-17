package main

import (
	"fmt"

	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/logger"
)

func main() {
	conf := config.ImageBuilderConfig{
		ListenAddress: "unused",
		LogLevel:      "INFO",
		MigrationsDir: "/usr/share/image-builder/migrations",
		PGHost:        "localhost",
		PGPort:        "5432",
		PGDatabase:    "imagebuilder",
		PGUser:        "postgres",
		PGPassword:    "foobar",
		// should be "prefer", see https://github.com/osbuild/image-builder/issues/197
		PGSSLMode: "disable",
	}

	err := config.LoadConfigFromEnv(&conf)
	if err != nil {
		panic(err)
	}

	log, err := logger.NewLogger(conf.LogLevel, conf.CwAccessKeyID, conf.CwSecretAccessKey, conf.CwRegion, conf.LogGroup, "")
	if err != nil {
		panic(err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", conf.PGUser, conf.PGPassword, conf.PGHost, conf.PGPort, conf.PGDatabase, conf.PGSSLMode)
	err = db.Migrate(connStr, conf.MigrationsDir, log)
	if err != nil {
		panic(err)
	}
}
