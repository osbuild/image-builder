package main

import (
	"fmt"
	"io"
	"log"
)

// NewLogger creates a new logger with the given writer, prefix, and quiet flag.
// It formats the prefix to have consistent length.
func NewLogger(writer io.Writer, prefix string, quiet bool) *log.Logger {
	for len(prefix) < MaxCheckNameLen {
		prefix += " "
	}
	logger := log.New(writer, fmt.Sprintf("[%s] ", prefix), 0)

	if quiet {
		logger.SetOutput(io.Discard)
	}

	return logger
}
