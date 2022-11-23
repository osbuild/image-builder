package main

import (
	"bufio"
	"bytes"
	"os/exec"

	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	conf := config.ImageBuilderConfig{
		ListenAddress:     "unused",
		LogLevel:          "INFO",
		TernExecutable:    "/opt/migrate/tern",
		TernMigrationsDir: "/app/migrations",
		PGHost:            "localhost",
		PGPort:            "5432",
		PGDatabase:        "imagebuilder",
		PGUser:            "postgres",
		PGPassword:        "foobar",
		PGSSLMode:         "prefer",
	}

	err := config.LoadConfigFromEnv(&conf)
	if err != nil {
		panic(err)
	}

	err = logger.ConfigLogger(logrus.StandardLogger(), conf.LogLevel, conf.SyslogServer)
	if err != nil {
		panic(err)
	}

	if conf.CwAccessKeyID != "" {
		err = logger.AddCloudWatchHook(logrus.StandardLogger(), conf.CwAccessKeyID, conf.CwSecretAccessKey, conf.CwRegion, conf.LogGroup)
		if err != nil {
			panic(err)
		}
	}

	// #nosec G204 -- the executable in the config can be trusted
	cmd := exec.Command(conf.TernExecutable,
		"migrate",
		"-m", conf.TernMigrationsDir,
		"--database", conf.PGDatabase,
		"--host", conf.PGHost,
		"--port", conf.PGPort,
		"--user", conf.PGUser,
		"--password", conf.PGPassword,
		"--sslmode", conf.PGSSLMode)
	out, err := cmd.CombinedOutput()

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		logrus.Info(scanner.Text())
	}

	if err != nil {
		panic(err)
	}

	logrus.Info("DB migration successful")
}
