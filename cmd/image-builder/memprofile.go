//go:build profiling

// See @cmd/image-builder/memprofile_stub.go for API documentation shared with the non-profiling build.

package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/spf13/cobra"
)

// Go default allocation sampling rate (see runtime.MemProfileRate).
const defaultMemProfileRate = 512 * 1024

var (
	memProfilePath          string
	memProfileGoroutinePath string
	memProfileRateOpt       int = -1
)

func memProfileResetForProcessStart() {
	runtime.MemProfileRate = 0
}

func registerMemProfileFlags(root *cobra.Command) {
	f := root.PersistentFlags()
	f.StringVar(&memProfilePath, "memprofile", "", "write a heap memory profile in pprof format when the program exits (use with go tool pprof)")
	f.StringVar(&memProfileGoroutinePath, "memprofile-goroutine", "", "write a goroutine profile in pprof format on exit (optional; use with go tool pprof)")
	f.IntVar(&memProfileRateOpt, "memprofile-rate", -1, "when --memprofile is set, sets runtime.MemProfileRate for allocation sampling (-1 uses Go's default 524288; 0 samples only live heap at exit with minimal runtime overhead)")
}

func memProfilePersistentPreRun(_ *cobra.Command, _ []string) {
	runtime.MemProfileRate = 0
	if memProfilePath != "" {
		if memProfileRateOpt >= 0 {
			runtime.MemProfileRate = memProfileRateOpt
		} else {
			runtime.MemProfileRate = defaultMemProfileRate
		}
	}
}

func memProfileFlush() error {
	var errs []error
	if memProfilePath != "" {
		if err := writeHeapProfile(memProfilePath); err != nil {
			errs = append(errs, err)
		}
	}
	if memProfileGoroutinePath != "" {
		if err := writeGoroutineProfile(memProfileGoroutinePath); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func writeHeapProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("memprofile: create heap profile %q: %w", path, err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("memprofile: write heap profile %q: %w", path, err)
	}
	return nil
}

func writeGoroutineProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("memprofile: create goroutine profile %q: %w", path, err)
	}
	defer f.Close()
	prof := pprof.Lookup("goroutine")
	if prof == nil {
		return fmt.Errorf("memprofile: goroutine profile unavailable")
	}
	if err := prof.WriteTo(f, 0); err != nil {
		return fmt.Errorf("memprofile: write goroutine profile %q: %w", path, err)
	}
	return nil
}
