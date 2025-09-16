package progress_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder-cli/pkg/progress"
)

func makeFakeOsbuild(t *testing.T, content string) string {
	p := filepath.Join(t.TempDir(), "fake-osbuild")
	err := os.WriteFile(p, []byte("#!/bin/sh\n"+content), 0755)
	assert.NoError(t, err)
	return p
}

func TestRunOSBuildWithProgressErrorReporting(t *testing.T) {
	restore := progress.MockOsStderr(io.Discard)
	defer restore()

	restore = progress.MockOsbuildCmd(makeFakeOsbuild(t, `
>&3 echo '{"message": "osbuild-stage-message"}'

echo osbuild-stdout-output
>&2 echo osbuild-stderr-output
exit 112
`))
	defer restore()

	pbar, err := progress.New("debug")
	assert.NoError(t, err)
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, nil)
	assert.EqualError(t, err, `error running osbuild: exit status 112
BuildLog:
osbuild-stage-message
Output:
osbuild-stdout-output
osbuild-stderr-output
`)
}

func TestRunOSBuildWithProgressIncorrectJSON(t *testing.T) {
	signalDeliveredMarkerPath := filepath.Join(t.TempDir(), "sigint-delivered")

	restore := progress.MockOsbuildCmd(makeFakeOsbuild(t, fmt.Sprintf(`
trap 'touch "%s";exit 2' INT

>&3 echo invalid-json

# we cannot sleep infinity here or the shell script trap is never run
while true; do
    sleep 0.1
done
`, signalDeliveredMarkerPath)))
	defer restore()

	pbar, err := progress.New("debug")
	assert.NoError(t, err)
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, nil)
	assert.EqualError(t, err, `error parsing osbuild status, please report a bug and try with "--progress=verbose": cannot scan line "invalid-json": invalid character 'i' looking for beginning of value`)

	// ensure the SIGINT got delivered
	var pathExists = func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if pathExists(signalDeliveredMarkerPath) {
			break
		}
	}
	assert.True(t, pathExists(signalDeliveredMarkerPath))
}

func TestRunOSBuildWithBuildlogTerm(t *testing.T) {
	restore := progress.MockOsbuildCmd(makeFakeOsbuild(t, `
echo osbuild-stdout-output
>&2 echo osbuild-stderr-output

# without the sleep this is racy as two different go routines poll
# this does not matter (much) in practise because osbuild output and
# stage output are using the syncedMultiWriter so output is not garbled
sleep 0.1
>&3 echo '{"message": "osbuild-stage-message"}'
`))
	defer restore()

	var fakeStdout, fakeStderr bytes.Buffer
	restore = progress.MockOsStdout(&fakeStdout)
	defer restore()
	restore = progress.MockOsStderr(&fakeStderr)
	defer restore()

	pbar, err := progress.New("term")
	assert.NoError(t, err)

	var buildLog bytes.Buffer
	opts := &progress.OSBuildOptions{
		BuildLog: &buildLog,
	}
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, opts)
	assert.NoError(t, err)
	expectedOutput := `osbuild-stdout-output
osbuild-stderr-output
osbuild-stage-message
`
	assert.Equal(t, expectedOutput, buildLog.String())
}

func TestRunOSBuildWithBuildlogVerbose(t *testing.T) {
	restore := progress.MockOsbuildCmd(makeFakeOsbuild(t, `
echo osbuild-stdout-output
>&2 echo osbuild-stderr-output
`))
	defer restore()

	var fakeStdout, fakeStderr bytes.Buffer
	restore = progress.MockOsStdout(&fakeStdout)
	defer restore()
	restore = progress.MockOsStderr(&fakeStderr)
	defer restore()

	pbar, err := progress.New("verbose")
	assert.NoError(t, err)

	var buildLog bytes.Buffer
	opts := &progress.OSBuildOptions{
		BuildLog: &buildLog,
	}
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, opts)
	assert.NoError(t, err)
	expectedOutput := `osbuild-stdout-output
osbuild-stderr-output
`
	assert.Equal(t, expectedOutput, buildLog.String())
}

func TestRunOSBuildCacheMaxSize(t *testing.T) {
	fakeOsbuildBinary := makeFakeOsbuild(t, `echo "$@" > "$0".cmdline`)
	restore := progress.MockOsbuildCmd(fakeOsbuildBinary)
	defer restore()

	pbar, err := progress.New("debug")
	assert.NoError(t, err)

	osbuildOpts := &progress.OSBuildOptions{
		CacheMaxSize: 77,
	}
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, osbuildOpts)
	assert.NoError(t, err)
	cmdline, err := os.ReadFile(fakeOsbuildBinary + ".cmdline")
	assert.NoError(t, err)
	assert.Contains(t, string(cmdline), "--cache-max-size=77")
}
