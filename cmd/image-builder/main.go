package main

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/osbuild/image-builder/internal/oauth2"

	"github.com/osbuild/image-builder/internal/clients/compliance"
	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/clients/content_sources"
	"github.com/osbuild/image-builder/internal/clients/provisioning"
	"github.com/osbuild/image-builder/internal/clients/recommendations"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/config"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/distribution"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/unleash"
	v1 "github.com/osbuild/image-builder/internal/v1"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	sentrylogrus "github.com/getsentry/sentry-go/logrus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	slogger "github.com/osbuild/osbuild-composer/pkg/splunk_logger"
	"github.com/sirupsen/logrus"
)

// gitRev returns the gitHash of the current running binary
func gitRev() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("cannot read build info")
	}
	for _, bs := range info.Settings {
		if bs.Key == "vcs.revision" {
			return bs.Value, nil
		}
	}
	return "", errors.New("vcs.revision not found in debug.ReadBuildInfo()")
}

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

	ctx := context.Background()

	err := config.LoadConfigFromEnv(&conf)
	if err != nil {
		panic(err)
	}

	if conf.GlitchTipDSN != "" {
		err = sentry.Init(sentry.ClientOptions{
			Dsn: conf.GlitchTipDSN,
		})
		if err != nil {
			panic(err)
		}
	}

	err = logger.ConfigLogger(logrus.StandardLogger(), conf.LogLevel)
	if err != nil {
		panic(err)
	}
	logrus.AddHook(&ctxHook{})

	gitRev, err := gitRev()
	if err != nil {
		logrus.Warn(err.Error())
		gitRev = "unknown"
	}

	logrus.Infof("Starting image-builder from Git Hash: %s", gitRev)
	logrus.Infof("Changelog: https://github.com/osbuild/image-builder/commits/%s", gitRev)

	if conf.GlitchTipDSN == "" {
		logrus.Warn("Sentry/Glitchtip was not initialized")
	} else {
		sentryhook := sentrylogrus.NewFromClient([]logrus.Level{logrus.PanicLevel,
			logrus.FatalLevel, logrus.ErrorLevel},
			sentry.CurrentHub().Client())
		logrus.AddHook(sentryhook)
	}

	if conf.CwAccessKeyID != "" {
		err = logger.AddCloudWatchHook(logrus.StandardLogger(), conf.CwAccessKeyID, conf.CwSecretAccessKey, conf.CwRegion, conf.LogGroup)
		if err != nil {
			panic(err)
		}
	}

	if conf.DeploymentChannel != "" {
		logrus.AddHook(&slogger.EnvironmentHook{Channel: conf.DeploymentChannel})
	}

	if conf.SplunkHost != "" {
		err = logger.AddSplunkHook(logrus.StandardLogger(), conf.SplunkHost, conf.SplunkPort, conf.SplunkToken)
		if err != nil {
			panic(err)
		}
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", conf.PGUser, conf.PGPassword, conf.PGHost, conf.PGPort, conf.PGDatabase, conf.PGSSLMode)
	dbase, err := db.InitDBConnectionPool(ctx, connStr)
	if err != nil {
		panic(err)
	}

	composerConf := composer.ComposerClientConfig{
		URL: conf.ComposerURL,
		CA:  conf.ComposerCA,
		Tokener: &oauth2.LazyToken{
			Url:          conf.ComposerTokenURL,
			ClientId:     conf.ComposerClientId,
			ClientSecret: conf.ComposerClientSecret,
		},
	}

	recommendationConf := recommendations.RecommendationsClientConfig{
		URL:   conf.RecommendURL,
		CA:    conf.RecommendCA,
		Proxy: conf.RecommendProxy,
		Tokener: &oauth2.LazyToken{
			Url:          conf.RecommendTokenURL,
			ClientId:     conf.RecommendClientId,
			ClientSecret: conf.RecommendSecret,
		},
	}
	compClient, err := composer.NewClient(composerConf)
	if err != nil {
		panic(err)
	}
	provClient, err := provisioning.NewClient(provisioning.ProvisioningClientConfig{
		URL: conf.ProvisioningURL,
	})
	if err != nil {
		panic(err)
	}

	csClient, err := content_sources.NewClient(content_sources.ContentSourcesClientConfig{
		URL: conf.ContentSourcesURL,
	})
	if err != nil {
		panic(err)
	}

	recommendClient, err := recommendations.NewClient(recommendationConf)
	if err != nil {
		panic(err)
	}

	complianceClient := compliance.NewClient(compliance.ComplianceClientConfig{
		URL: conf.ComplianceURL,
	})

	adr, err := distribution.LoadDistroRegistry(conf.DistributionsDir)
	if err != nil {
		panic(err)
	}

	if len(adr.Available(true).List()) == 0 {
		panic("no distributions defined")
	}

	if conf.UnleashURL != "" {
		err = unleash.Initialize(unleash.Config{
			URL:   conf.UnleashURL,
			Token: conf.UnleashToken,
		})
		if err != nil {
			panic(err)
		}
	}

	echoServer := echo.New()
	echoServer.HideBanner = true
	echoServer.Logger = common.Logger()
	echoServer.Use(requestIdExtractMiddleware)
	echoServer.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogMethod:  true,
		LogValuesFunc: func(c echo.Context, values middleware.RequestLoggerValues) error {
			fields := logrus.Fields{
				"uri":         values.URI,
				"method":      values.Method,
				"status":      values.Status,
				"latency_ms":  values.Latency.Milliseconds(),
				"request_id":  common.RequestId(c.Request().Context()),
				"insights_id": common.InsightsRequestId(c.Request().Context()),
			}
			if values.Error != nil {
				fields["error"] = values.Error
			}
			logrus.WithContext(c.Request().Context()).
				WithFields(fields).Infof("Processed request %s %s", values.Method, values.URI)

			return nil
		},
		Skipper: func(c echo.Context) bool {
			return SkipPath(c.Path())
		},
	}))
	if conf.GlitchTipDSN != "" {
		echoServer.Use(sentryecho.New(sentryecho.Options{}))
	}
	// log stack traces into standard logger as error (instead of stdout)
	echoServer.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogLevel: log.ERROR,
	}))
	if conf.IsDebug() {
		echoServer.Debug = true
	}
	serverConfig := &v1.ServerConfig{
		EchoServer:       echoServer,
		CompClient:       compClient,
		ProvClient:       provClient,
		RecommendClient:  recommendClient,
		ComplianceClient: complianceClient,
		CSClient:         csClient,
		CSReposURL:       conf.ContentSourcesRepoURL,
		DBase:            dbase,

		AwsConfig: v1.AWSConfig{
			Region: conf.OsbuildRegion,
		},
		GcpConfig: v1.GCPConfig{
			Region: conf.OsbuildGCPRegion,
			Bucket: conf.OsbuildGCPBucket,
		},
		QuotaFile:        conf.QuotaFile,
		AllowFile:        conf.AllowFile,
		AllDistros:       adr,
		DistributionsDir: conf.DistributionsDir,
		FedoraAuth:       conf.FedoraAuth,
	}

	_, err = v1.Attach(serverConfig)
	if err != nil {
		panic(err)
	}

	logrus.Infof("🚀 Starting image-builder built %s sha %s server on %v ...\n", common.BuildTime, common.BuildCommit, conf.ListenAddress)
	err = echoServer.Start(conf.ListenAddress)
	if err != nil {
		panic(err)
	}
}
