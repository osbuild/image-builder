package bootctest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/randutil"
)

func makeOsRelease(t *testing.T, buildDir string) {
	osRelease := `
NAME="bootc-fake-name"
ID="bootc-fake"
VERSION_ID="1"
`

	osReleasePath := filepath.Join(buildDir, "etc/os-release")
	err := os.MkdirAll(filepath.Dir(osReleasePath), 0755)
	require.NoError(t, err)
	//nolint:gosec
	err = os.WriteFile(osReleasePath, []byte(osRelease), 0644)
	require.NoError(t, err)
}

func makeBootcInstallToml(t *testing.T, buildDir string) {
	installToml := `
[install]
filesystem = [
    { mountpoint = "/", type = "xfs", size = "10 GiB" },
    { mountpoint = "/boot", type = "ext4", size = "1 GiB" },
]
`

	installTomlPath := filepath.Join(buildDir, "usr/lib/bootc/install/99-fedora-install.toml")
	err := os.MkdirAll(filepath.Dir(installTomlPath), 0755)
	require.NoError(t, err)
	//nolint:gosec
	err = os.WriteFile(installTomlPath, []byte(installToml), 0644)
	require.NoError(t, err)
}

func makeFakeBinaries(t *testing.T, buildDir string) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	currentDir := filepath.Dir(currentFile)

	destDir := fmt.Sprintf("DESTDIR=%s", filepath.Join(buildDir, "/usr/bin"))
	//nolint:gosec
	output, err := exec.Command("make", "-C", filepath.Join(currentDir, "exe"), destDir).CombinedOutput()
	require.NoError(t, err, string(output))
}

func makeContainerfile(t *testing.T, buildDir string, extraFiles map[string]string) {
	var fakeBootcCnt = `
FROM scratch
COPY etc /etc
COPY usr/bin /usr/bin
COPY usr/lib/bootc/install /usr/lib/bootc/install
`
	for path, content := range extraFiles {
		fakeBootcCnt += fmt.Sprintf("COPY %s %s\n", path[1:], path)

		err := os.MkdirAll(filepath.Join(buildDir, filepath.Dir(path)), 0755)
		require.NoError(t, err)
		//nolint:gosec
		err = os.WriteFile(filepath.Join(buildDir, path), []byte(content), 0644)
		require.NoError(t, err)
	}

	cntFilePath := filepath.Join(buildDir, "Containerfile")
	//nolint:gosec
	err := os.WriteFile(cntFilePath, []byte(fakeBootcCnt), 0644)
	require.NoError(t, err)
}

func makeFakeContainerImage(t *testing.T, buildDir, purpose string) string {
	imgTag := fmt.Sprintf("image-builder-test-%s-%s", purpose, randutil.String(10, randutil.AsciiLower))
	//nolint:gosec
	output, err := exec.Command(
		"podman", "build",
		"-f", filepath.Join(buildDir, "Containerfile"),
		"-t", imgTag,
	).CombinedOutput()
	require.NoError(t, err, string(output))
	// add cleanup
	t.Cleanup(func() {
		output, err := exec.Command("podman", "image", "rm", "--force", imgTag).CombinedOutput()
		assert.NoError(t, err, string(output))
	})

	return fmt.Sprintf("localhost/%s", imgTag)
}

func NewFakeContainer(t *testing.T, purpose string, extraFiles map[string]string) string {
	t.Helper()

	buildDir := t.TempDir()

	makeContainerfile(t, buildDir, extraFiles)
	makeFakeBinaries(t, buildDir)
	// XXX: make os-release content configurable
	makeOsRelease(t, buildDir)
	makeBootcInstallToml(t, buildDir)

	return makeFakeContainerImage(t, buildDir, purpose)
}
