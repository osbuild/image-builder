package testutil

import (
	"os"
	"path/filepath"
	"strings"
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

func (mc *MockCmd) Restore() error {
	return os.RemoveAll(mc.binDir)
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
