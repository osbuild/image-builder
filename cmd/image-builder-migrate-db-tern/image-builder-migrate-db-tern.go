package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/osbuild/image-builder-crc/internal/config"
	"github.com/osbuild/logging/pkg/sinit"
)

func main() {
	ctx := context.Background()
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

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "image-builder-unknown"
	}

	loggingConfig := sinit.LoggingConfig{
		StdoutConfig: sinit.StdoutConfig{
			Enabled: true,
			Level:   "warning",
			Format:  "text",
		},
		SplunkConfig: sinit.SplunkConfig{
			Enabled:  conf.SplunkHost != "" && conf.SplunkPort != "" && conf.SplunkToken != "",
			Level:    conf.LogLevel,
			URL:      fmt.Sprintf("https://%s:%s/services/collector/event", conf.SplunkHost, conf.SplunkPort),
			Token:    conf.SplunkToken,
			Source:   "image-builder",
			Hostname: hostname,
		},
		CloudWatchConfig: sinit.CloudWatchConfig{
			Enabled:      conf.CwAccessKeyID != "" && conf.CwSecretAccessKey != "" && conf.CwRegion != "",
			Level:        conf.LogLevel,
			AWSRegion:    conf.CwRegion,
			AWSSecret:    conf.CwSecretAccessKey,
			AWSKey:       conf.CwAccessKeyID,
			AWSLogGroup:  conf.LogGroup,
			AWSLogStream: hostname,
		},
		SentryConfig: sinit.SentryConfig{
			Enabled: conf.GlitchTipDSN != "",
			DSN:     conf.GlitchTipDSN,
		},
	}

	err = sinit.InitializeLogging(ctx, loggingConfig)
	if err != nil {
		panic(err)
	}

	slog.InfoContext(ctx, "starting image-builder migration",
		"splunk", loggingConfig.SplunkConfig.Enabled,
		"cloudwatch", loggingConfig.CloudWatchConfig.Enabled,
		"sentry", loggingConfig.SentryConfig.Enabled,
	)

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
		slog.InfoContext(ctx, scanner.Text())
	}

	if err != nil {
		panic(err)
	}

	slog.InfoContext(ctx, "DB migration successful")
}
