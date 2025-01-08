package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/osbuild/logging/pkg/sinit"
	"github.com/osbuild/logging/pkg/strc"
)

func main() {
	ctx, applicationCancel := context.WithCancel(context.Background())
	defer applicationCancel()

	terminationSignal := make(chan os.Signal, 1)
	signal.Notify(terminationSignal, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(terminationSignal)

	// Channel to track graceful application shutdown
	shutdownSignal := make(chan struct{})

	go func() {
		select {
		case <-terminationSignal:
			fmt.Println("Received termination signal, cancelling context...")
			applicationCancel()
		case <-shutdownSignal:
			// Normal application shutdown, no logging
		}
	}()

	ctx, cancelTimeout := context.WithTimeout(ctx, 2*time.Hour)
	defer cancelTimeout()

	conf := Config{
		DryRun:                  true,
		EnableDBMaintenance:     false,
		ComposesRetentionMonths: 24,
	}

	err := LoadConfigFromEnv(&conf)
	if err != nil {
		panic(err)
	}

	loggingConfig := sinit.LoggingConfig{
		StdoutConfig: sinit.StdoutConfig{
			Enabled: true,
			Level:   "debug",
			Format:  "text",
		},
		TracingConfig: sinit.TracingConfig{
			Enabled: true,
		},
	}

	err = sinit.InitializeLogging(ctx, loggingConfig)
	if err != nil {
		panic(err)
	}

	span, ctx := strc.Start(ctx, "maintainance")
	defer span.End()

	slog.InfoContext(ctx, "starting image-builder maintainance")

	if conf.DryRun {
		slog.InfoContext(ctx, "dry run, no state will be changed")
	}

	if !conf.EnableDBMaintenance {
		slog.InfoContext(ctx, "🦀🦀🦀 DB maintenance not enabled, skipping  🦀🦀🦀")
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
	err = DBCleanup(ctx, dbURL, conf.DryRun, conf.ComposesRetentionMonths)
	if err != nil {
		slog.ErrorContext(ctx, "error during DBCleanup", "err", err)
		os.Exit(1)
	}
	slog.InfoContext(ctx, "🦀🦀🦀 dbqueue cleanup done 🦀🦀🦀")
	close(shutdownSignal)
}
