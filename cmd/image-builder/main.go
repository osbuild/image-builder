package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/distribution"
	"github.com/osbuild/image-builder/internal/logger"
	v1 "github.com/osbuild/image-builder/internal/v1"

	"github.com/labstack/echo/v4"
)

func main() {
	conf := config.ImageBuilderConfig{
		ListenAddress: "localhost:8086",
		LogLevel:      "INFO",
		PGHost:        "localhost",
		PGPort:        "5432",
		PGDatabase:    "imagebuilder",
		PGUser:        "postgres",
		PGPassword:    "foobar",
		PGSSLMode:     "prefer",
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

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", conf.PGUser, conf.PGPassword, conf.PGHost, conf.PGPort, conf.PGDatabase, conf.PGSSLMode)
	dbase, err := db.InitDBConnectionPool(connStr)
	if err != nil {
		panic(err)
	}

	composerConf := composer.ComposerClientConfig{
		ComposerURL:  conf.ComposerURL,
		CA:           conf.ComposerCA,
		TokenURL:     conf.ComposerTokenURL,
		ClientId:     conf.ComposerClientId,
		OfflineToken: conf.ComposerOfflineToken,
		ClientSecret: conf.ComposerClientSecret,
	}
	client, err := composer.NewClient(composerConf)
	if err != nil {
		panic(err)
	}

	aws := v1.AWSConfig{
		Region: conf.OsbuildRegion,
	}
	gcp := v1.GCPConfig{
		Region: conf.OsbuildGCPRegion,
		Bucket: conf.OsbuildGCPBucket,
	}

	adr, err := distribution.LoadDistroRegistry(conf.DistributionsDir)
	if err != nil {
		panic(err)
	}

	if len(adr.Available(true).List()) == 0 {
		panic("no distributions defined")
	}

	echoServer := echo.New()
	err = v1.Attach(echoServer, client, dbase, aws, gcp, conf.QuotaFile, conf.AllowFile, adr)
	if err != nil {
		panic(err)
	}

	logrus.Infof("🚀 Starting image-builder server on %v ...\n", conf.ListenAddress)
	err = echoServer.Start(conf.ListenAddress)
	if err != nil {
		panic(err)
	}
}
