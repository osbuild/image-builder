package main

import (
	"fmt"
	"strings"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/v1"

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

	log, err := logger.NewLogger(conf.LogLevel, conf.CwAccessKeyID, conf.CwSecretAccessKey, conf.CwRegion, conf.LogGroup)
	if err != nil {
		panic(err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", conf.PGUser, conf.PGPassword, conf.PGHost, conf.PGPort, conf.PGDatabase, conf.PGSSLMode)
	dbase, err := db.InitDBConnectionPool(connStr)
	if err != nil {
		panic(err)
	}

	client, err := cloudapi.NewOsbuildClient(conf.OsbuildURL, conf.OsbuildCert, conf.OsbuildKey, conf.OsbuildCA)
	if err != nil {
		panic(err)
	}

	// Make a slice of allowed organization ids, '*' in the slice means blanket permission
	orgIds := []string{}
	if conf.OrgIds != "" {
		orgIds = strings.Split(conf.OrgIds, ";")
	}

	// Make a slice of allowed organization ids, '*' in the slice means blanket permission
	accountNumbers := []string{}
	if conf.AccountNumbers != "" {
		accountNumbers = strings.Split(conf.AccountNumbers, ";")
	}

	aws := v1.AWSConfig{
		Region:          conf.OsbuildRegion,
		AccessKeyId:     conf.OsbuildAccessKeyID,
		SecretAccessKey: conf.OsbuildSecretAccessKey,
		S3Bucket:        conf.OsbuildS3Bucket,
	}
	gcp := v1.GCPConfig{
		Region: conf.OsbuildGCPRegion,
		Bucket: conf.OsbuildGCPBucket,
	}

	azure := v1.AzureConfig{
		Location: conf.OsbuildAzureLocation,
	}

	echoServer := echo.New()
	err = v1.Attach(echoServer, log, client, dbase, aws, gcp, azure, orgIds, accountNumbers, conf.DistributionsDir)
	if err != nil {
		panic(err)
	}

	log.Infof("🚀 Starting image-builder server on %v ...\n", conf.ListenAddress)
	err = echoServer.Start(conf.ListenAddress)
	if err != nil {
		panic(err)
	}
}
