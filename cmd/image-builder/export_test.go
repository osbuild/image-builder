package main

import (
	"fmt"
	"io"
	"os"

	"github.com/osbuild/images/pkg/reporegistry"
)

func MockOsArgs(new []string) (restore func()) {
	saved := os.Args
	os.Args = append([]string{"argv0"}, new...)
	return func() {
		os.Args = saved
	}
}

func MockOsStdout(new io.Writer) (restore func()) {
	saved := osStdout
	osStdout = new
	return func() {
		osStdout = saved
	}
}

func MockOsStderr(new io.Writer) (restore func()) {
	saved := osStderr
	osStderr = new
	return func() {
		osStderr = saved
	}
}

func MockNewRepoRegistry(f func() (*reporegistry.RepoRegistry, error)) (restore func()) {
	saved := newRepoRegistry
	newRepoRegistry = func(dataDir string) (*reporegistry.RepoRegistry, error) {
		if dataDir != "" {
			panic(fmt.Sprintf("cannot use custom dataDir %v in mock", dataDir))
		}
		return f()
	}
	return func() {
		newRepoRegistry = saved
	}
}

var (
	Run = run
)
