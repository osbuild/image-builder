package osbuild_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/datasizes"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func makeFakeOSBuild(t *testing.T, content string) string {
	p := filepath.Join(t.TempDir(), "fake-osbuild")
	err := os.WriteFile(p, []byte("#!/bin/sh\n"+content), 0755)
	assert.NoError(t, err)
	return p
}

func TestNewOSBuildCmdNilOptions(t *testing.T) {
	mf := []byte(`{"real": "manifest"}`)
	cmd := osbuild.NewOSBuildCmd(mf, nil)
	assert.NotNil(t, cmd)

	assert.Equal(
		t,
		[]string{
			"osbuild",
			"--store",
			"",
			"--output-directory",
			"",
			fmt.Sprintf("--cache-max-size=%d", int64(20*datasizes.GiB)),
			"-",
		},
		cmd.Args,
	)

	stdin, err := io.ReadAll(cmd.Stdin)
	assert.NoError(t, err)
	assert.Equal(t, mf, stdin)
}

func TestNewOSBuildCmdFullOptions(t *testing.T) {
	mf := []byte(`{"real": "manifest"}`)
	_, wp, err := os.Pipe()
	assert.NoError(t, err)
	cmd := osbuild.NewOSBuildCmd(
		mf,
		&osbuild.OSBuildOptions{
			StoreDir:  "store",
			OutputDir: "output",
			Exports: []string{
				"export-1",
				"export-2",
			},
			Checkpoints:  []string{"checkpoint-1", "checkpoint-2"},
			ExtraEnv:     []string{"EXTRA_ENV_1=1", "EXTRA_ENV_2=2"},
			Monitor:      osbuild.MonitorLog,
			MonitorFile:  wp,
			JSONOutput:   true,
			CacheMaxSize: 10 * datasizes.GiB,
		},
	)
	assert.NotNil(t, cmd)

	assert.Equal(
		t,
		[]string{
			"osbuild",
			"--store",
			"store",
			"--output-directory",
			"output",
			fmt.Sprintf("--cache-max-size=%d", int64(10*datasizes.GiB)),
			"-",
			"--export",
			"export-1",
			"--export",
			"export-2",
			"--checkpoint",
			"checkpoint-1",
			"--checkpoint",
			"checkpoint-2",
			"--monitor=LogMonitor",
			"--monitor-fd=3",
			"--json",
		},
		cmd.Args,
	)

	assert.Contains(t, cmd.Env, "EXTRA_ENV_1=1")
	assert.Contains(t, cmd.Env, "EXTRA_ENV_2=2")

	stdin, err := io.ReadAll(cmd.Stdin)
	assert.NoError(t, err)
	assert.Equal(t, mf, stdin)

	assert.Equal(t, wp, cmd.ExtraFiles[0])
}

func TestRunOSBuildJSONOutput(t *testing.T) {
	fakeOSBuildBinary := makeFakeOSBuild(t, `
if [ "$1" = "--version" ]; then
    echo '90000.0'
else
    echo '{"success": true}'
fi
`)
	restore := osbuild.MockOSBuildCmd(fakeOSBuildBinary)
	defer restore()

	opts := &osbuild.OSBuildOptions{
		JSONOutput: true,
	}
	result, err := osbuild.RunOSBuild([]byte(`{"fake":"manifest"}`), opts)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestRunOSBuildBuildLog(t *testing.T) {
	fakeOSBuildBinary := makeFakeOSBuild(t, `
if [ "$1" = "--version" ]; then
    echo '90000.0'
else
    echo osbuild-stdout-output
fi
>&2 echo osbuild-stderr-output
`)
	restore := osbuild.MockOSBuildCmd(fakeOSBuildBinary)
	defer restore()

	var buildLog, stdout, stderr bytes.Buffer
	opts := &osbuild.OSBuildOptions{
		BuildLog: &buildLog,
		Stdout:   &stdout,
		Stderr:   &stderr,
	}

	result, err := osbuild.RunOSBuild(nil, opts)
	assert.NoError(t, err)
	// without json output set the result should be empty
	assert.Empty(t, result)

	assert.NoError(t, err)
	assert.Equal(t, `osbuild-stdout-output
osbuild-stderr-output
`, buildLog.String())
	assert.Equal(t, `osbuild-stdout-output
osbuild-stderr-output
`, stdout.String())
	assert.Empty(t, stderr.Bytes())
}

func TestRunOSBuildMonitor(t *testing.T) {
	fakeOSBuildBinary := makeFakeOSBuild(t, `
>&3 echo -n '{"some": "monitor"}'

if [ "$1" = "--version" ]; then
    echo '90000.0'
else
    echo -n osbuild-stdout-output
fi
>&2 echo -n osbuild-stderr-output
`)
	restore := osbuild.MockOSBuildCmd(fakeOSBuildBinary)
	defer restore()

	rp, wp, err := os.Pipe()
	assert.NoError(t, err)
	defer wp.Close()

	var stdout, stderr bytes.Buffer
	opts := &osbuild.OSBuildOptions{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Monitor:     osbuild.MonitorJSONSeq,
		MonitorFile: wp,
	}

	// without json output set the result will be empty
	_, err = osbuild.RunOSBuild(nil, opts)
	assert.NoError(t, err)
	assert.NoError(t, wp.Close())

	monitorOutput, err := io.ReadAll(rp)
	assert.NoError(t, err)
	assert.Equal(t, `{"some": "monitor"}`, string(monitorOutput))
	assert.Equal(t, "osbuild-stdout-output", stdout.String())
	assert.Equal(t, "osbuild-stderr-output", stderr.String())
}

func TestSyncWriter(t *testing.T) {
	var mu sync.Mutex
	var buf bytes.Buffer
	var wg sync.WaitGroup

	for id := 0; id < 100; id++ {
		wg.Add(1)
		w := osbuild.NewSyncedWriter(&mu, &buf)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				fmt.Fprintln(w, strings.Repeat(fmt.Sprintf("%v", id%10), 60))
				time.Sleep(10 * time.Nanosecond)
			}
		}(id)
	}
	wg.Wait()

	scanner := bufio.NewScanner(&buf)
	for res := scanner.Scan(); res; res = scanner.Scan() {
		line := scanner.Text()
		assert.True(t, len(line) == 60, fmt.Sprintf("len %v: line: %v", len(line), line))
	}
	assert.NoError(t, scanner.Err())
}
