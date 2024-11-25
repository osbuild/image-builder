package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
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
	err = DBCleanup(ctx, dbURL, conf.DryRun, conf.ClonesRetentionMonths)
	if err != nil {
		logrus.Fatalf("Error during DBCleanup: %v", err)
	}
	logrus.Info("🦀🦀🦀 dbqueue cleanup done 🦀🦀🦀")
	close(shutdownSignal)
}
