package testutil_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder-cli/internal/testutil"
)

func TestMockCommand(t *testing.T) {
	fakeCmd := testutil.MockCommand(t, "false", "exit 0")
	defer fakeCmd.Restore()

	err := exec.Command("false", "run1-arg1", "run1-arg2").Run()
	assert.NoError(t, err)
	err = exec.Command("false", "run2-arg1", "run2-arg2").Run()
	assert.NoError(t, err)

	assert.Equal(t, [][]string{
		{"run1-arg1", "run1-arg2"},
		{"run2-arg1", "run2-arg2"},
	}, fakeCmd.Calls())
}
