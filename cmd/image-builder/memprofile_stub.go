//go:build !profiling

// Memory profiling hooks: this file is the no-op implementation when the binary is built
// without -tags profiling. See memprofile.go for the profiling implementation.

package main

import "github.com/spf13/cobra"

// memProfileResetForProcessStart is called at the start of run() before cobra runs; it keeps
// default allocation sampling behavior. With profiling, it forces sampling off until flags apply.
func memProfileResetForProcessStart() {}

// registerMemProfileFlags would attach --memprofile* flags to the root command; no flags are
// registered without the profiling build tag.
func registerMemProfileFlags(_ *cobra.Command) {}

// memProfilePersistentPreRun would set runtime.MemProfileRate from --memprofile* flags; it does
// nothing without the profiling build tag.
func memProfilePersistentPreRun(_ *cobra.Command, _ []string) {}

// memProfileFlush would write heap and/or goroutine profiles on exit; it always returns nil here.
func memProfileFlush() error { return nil }
