package main

import (
	"fmt"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/server"
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
	}

	err := config.LoadConfigFromEnv(&conf)
	if err != nil {
		panic(err)
	}

	log, err := logger.NewLogger(conf.LogLevel, conf.CwAccessKeyID, conf.CwSecretAccessKey, conf.CwRegion, conf.LogGroup)
	if err != nil {
		panic(err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", conf.PGUser, conf.PGPassword, conf.PGHost, conf.PGPort, conf.PGDatabase)
	d, err := db.InitDBConnectionPool(connStr)
	if err != nil {
		panic(err)
	}

	client, err := cloudapi.NewOsbuildClient(conf.OsbuildURL, conf.OsbuildCert, conf.OsbuildKey, conf.OsbuildCA)
	if err != nil {
		panic(err)
	}

	s := server.NewServer(log, client, conf.OsbuildRegion, conf.OsbuildAccessKeyID, conf.OsbuildSecretAccessKey, conf.OsbuildS3Bucket, d)
	s.Run(conf.ListenAddress)
}
