package main_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	main "github.com/osbuild/image-builder/cmd/image-builder"
	"github.com/osbuild/image-builder/internal/testutil"
	"github.com/osbuild/image-builder/pkg/arch"
	testrepos "github.com/osbuild/image-builder/test/data/repositories"
	"github.com/stretchr/testify/require"
)

func TestTaskIntegrationArgs(t *testing.T) {
	restore := main.MockManifestgenDepsolver(fakeDepsolve)
	defer restore()

	restore = main.MockManifestgenContainerResolver(fakeContainerResolver)
	defer restore()

	restore = main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	currentArch := arch.Current().String()
	prefix := func(s string) string {
		return fmt.Sprintf("centos-9-qcow2-%s.%s", currentArch, s)
	}
	for idx, tc := range []struct {
		args []string

		// expected files after task init
		expectedFilesInit []string

		// expected files after task build (not including expectedFilesInit)
		expectedFilesBuild []string
	}{
		{
			args:               nil,
			expectedFilesInit:  []string{prefix("manifest.json")},
			expectedFilesBuild: []string{prefix("qcow2")},
		},
		{
			args:               []string{"--with-buildlog"},
			expectedFilesInit:  []string{prefix("manifest.json")},
			expectedFilesBuild: []string{prefix("qcow2"), prefix("buildlog")},
		},
		{
			args:               []string{"--with-buildlog", "--with-rpmlist"},
			expectedFilesInit:  []string{prefix("manifest.json"), prefix("rpmlist.json")},
			expectedFilesBuild: []string{prefix("qcow2"), prefix("buildlog")},
		},
	} {
		t.Run(fmt.Sprintf("case%02d", idx), func(t *testing.T) {

			var expFiles []string
			taskdir := t.TempDir()

			t.Run("init", func(t *testing.T) {
				require := require.New(t)

				args := []string{"task", "init", "--image-type", "qcow2", "--distro", "centos-9", taskdir}
				args = append(args, tc.args...)
				restore = main.MockOsArgs(args)
				defer restore()

				osbuildCmd := testutil.MockCommand(t, "osbuild", makeFakeOsbuildScript())
				require.NoError(main.Run())

				// first call is --version for writing the version to the sidecar
				require.Equal([]string{"--version"}, osbuildCmd.CallArgsList()[0])

				// check expected files after init
				actFiles := dirRelative(t, taskdir)

				// we always expect task.json
				expFiles = append(tc.expectedFilesInit, "task.json")
				require.ElementsMatch(expFiles, actFiles)
			})

			t.Run("prefetch", func(t *testing.T) {
				require := require.New(t)

				args := []string{"task", "prefetch", taskdir}
				restore = main.MockOsArgs(args)
				defer restore()

				osbuildCmd := testutil.MockCommand(t, "osbuild", makeFakeOsbuildScript())
				require.NoError(main.Run())

				// first call is --version for comparing the version to the sidecar
				require.Equal([]string{"--version"}, osbuildCmd.CallArgsList()[0])

				// second call sets cache dir to download sources
				expArgs := []string{
					"--store", fmt.Sprintf("%s/cache", taskdir),

					// the following are always added
					"--output-directory", "", // not set but the option is always passed
					"--cache-max-size=21474836480",
					"-",
				}
				require.Equal(expArgs, osbuildCmd.CallArgsList()[1])

				// check expected files after prefetch
				actFiles := dirRelative(t, taskdir)
				expFiles = append(expFiles, "cache") // add cache to previous expected files
				require.ElementsMatch(expFiles, actFiles)
			})

			t.Run("build", func(t *testing.T) {
				require := require.New(t)

				args := []string{"task", "build-image", taskdir}
				restore = main.MockOsArgs(args)
				defer restore()

				osbuildCmd := testutil.MockCommand(t, "osbuild", makeFakeOsbuildScript())
				require.NoError(main.Run())

				// first call is --version for comparing the version to the sidecar
				require.Equal([]string{"--version"}, osbuildCmd.CallArgsList()[0])

				// second call to build the image
				expArgs := []string{
					"--store", fmt.Sprintf("%s/cache", taskdir),
					"--output-directory", taskdir,
					"--cache-max-size=21474836480",
					"-",
					"--export", "qcow2",
				}
				require.Equal(expArgs, osbuildCmd.CallArgsList()[1])

				// check expected files after build
				actFiles := dirRelative(t, taskdir)
				expFiles = append(expFiles, tc.expectedFilesBuild...) // append to previous expected files
				require.ElementsMatch(expFiles, actFiles)
			})
		})
	}
}

// returns a list of filenames in a dir with dir stripped
func dirRelative(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(dir + "/*")
	if err != nil {
		t.Fatalf("failed to glob for files in %q: %s", dir, err)
	}

	stripped := make([]string, len(paths))
	for idx, path := range paths {
		stripped[idx] = strings.TrimPrefix(path, dir+"/")
	}

	return stripped
}
