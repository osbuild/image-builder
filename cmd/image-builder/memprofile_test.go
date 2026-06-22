//go:build profiling

package main_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	main "github.com/osbuild/image-builder/v73/cmd/image-builder"
	testrepos "github.com/osbuild/image-builder/v73/test/data/repositories"
)

func TestMemProfileWritesHeapAndGoroutineFiles(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	dir := t.TempDir()
	heapPath := filepath.Join(dir, "heap.pprof")
	gPath := filepath.Join(dir, "goroutine.pprof")

	restore = main.MockOsArgs([]string{"list", "--format=json", "--memprofile", heapPath, "--memprofile-goroutine", gPath})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	st, err := os.Stat(heapPath)
	require.NoError(t, err)
	require.Greater(t, st.Size(), int64(0))

	stg, err := os.Stat(gPath)
	require.NoError(t, err)
	require.Greater(t, stg.Size(), int64(0))
}
