package testutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type MockCmd struct {
	binDir string
	name   string
}

func MockCommand(t *testing.T, name string, script string) *MockCmd {
	mockCmd := &MockCmd{
		binDir: t.TempDir(),
		name:   name,
	}

	fullScript := `#!/bin/bash -e
for arg in "$@"; do
   echo -e -n "$arg\x0" >> "$0".run
done
echo >> "$0".run
` + script

	t.Setenv("PATH", mockCmd.binDir+":"+os.Getenv("PATH"))
	err := os.WriteFile(filepath.Join(mockCmd.binDir, name), []byte(fullScript), 0755)
	require.NoError(t, err)

	return mockCmd
}

func (mc *MockCmd) Path() string {
	return filepath.Join(mc.binDir, mc.name)
}

func (mc *MockCmd) Calls() [][]string {
	b, err := os.ReadFile(mc.Path() + ".run")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		panic(err)
	}
	var calls [][]string
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		call := strings.Split(line, "\000")
		calls = append(calls, call[0:len(call)-1])
	}
	return calls
}

// CaptureStdio runs the given function f() in an environment that
// captures stdout, stderr and returns the the result as string.
//
// Use this very targeted to avoid real stdout/stderr output
// from being displayed.
func CaptureStdio(t *testing.T, f func()) (string, string) {
	saved1 := os.Stdout
	saved2 := os.Stderr

	r1, w1, err := os.Pipe()
	require.NoError(t, err)
	defer r1.Close()
	defer w1.Close()
	r2, w2, err := os.Pipe()
	require.NoError(t, err)
	defer r2.Close()
	defer w2.Close()

	var wg sync.WaitGroup
	var stdout, stderr bytes.Buffer
	wg.Add(1)
	// this needs to be a go-routines or we could deadlock
	// when the pipe is full
	go func() {
		defer wg.Done()
		io.Copy(&stdout, r1)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(&stderr, r2)
	}()

	os.Stdout = w1
	os.Stderr = w2
	defer func() {
		os.Stdout = saved1
		os.Stderr = saved2
	}()

	f()
	w1.Close()
	w2.Close()
	wg.Wait()
	return stdout.String(), stderr.String()
}

func Chdir(t *testing.T, dir string, f func()) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	defer func() {
		os.Chdir(cwd)
	}()

	err = os.Chdir(dir)
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	f()
}
