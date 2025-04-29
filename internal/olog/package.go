// Package olog provides a simple wrapper around the standard log package
// variable so logging statements are shorter. Instead of pkg.Logger.Printf one
// can simply write olog.Printf.
//
// The name stands for "optional logger" because the output can be disabled by
// setting the logger to nil or providing io.Discard as the output. This is
// useful for user-configurable logging (e.g. --verbose flags for CLI apps).
package olog
