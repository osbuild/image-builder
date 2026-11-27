package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/image-builder/cmd/image-builder"
	"github.com/osbuild/image-builder/internal/buildconfig"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/osbuild/manifesttest"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	testrepos "github.com/osbuild/image-builder/test/data/repositories"
)

func TestInitramfsManifest(t *testing.T) {
	restore := main.MockManifestgenDepsolver(func(solver *depsolvednf.Solver, cacheDir string, warnings io.Writer,
		sets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
		require.Len(t, sets["initramfs-tree"], 1)
		assert.ElementsMatch(t, []string{"bash", "util-linux-core", "systemd", "systemd-udev"}, sets["initramfs-tree"][0].Include)
		require.Len(t, sets["build"], 1)
		assert.Subset(t, sets["build"][0].Include, []string{"rpm", "cpio", "zstd"})
		return fakeDepsolve(solver, cacheDir, warnings, sets, d, arch)
	})
	defer restore()

	restore = main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	config, err := buildconfig.New(filepath.Join("..", "..", "test", "configs", "initramfs-smoke.json"), nil)
	require.NoError(t, err)
	blueprint, err := json.Marshal(config.Blueprint)
	require.NoError(t, err)
	blueprintPath := filepath.Join(t.TempDir(), "blueprint.json")
	require.NoError(t, os.WriteFile(blueprintPath, blueprint, 0600))
	restore = main.MockOsArgs([]string{
		"manifest", "initramfs",
		"--arch=x86_64", "--distro=fedora-45",
		"--blueprint", blueprintPath,
		"--rpmmd-cache", t.TempDir(),
	})
	defer restore()

	var stdout bytes.Buffer
	restore = main.MockOsStdout(&stdout)
	defer restore()

	require.NoError(t, main.Run())
	m, err := manifesttest.NewManifestFromBytes(stdout.Bytes())
	require.NoError(t, err)
	assert.Equal(t, []string{"build", "initramfs-tree", "initramfs-cpio", "initramfs"}, m.PipelineNames())

	tree := m.Pipeline("initramfs-tree")
	require.NotNil(t, tree)
	require.NotNil(t, tree.Stage("org.osbuild.rpm"))
	require.NotNil(t, tree.Stage("org.osbuild.ln"))
	assertJsonContains(t, string(tree.Stage("org.osbuild.rpm").Options), `"disable_dracut":true`)
	require.NotNil(t, tree.Stage("org.osbuild.chmod"))
	assert.JSONEq(t, `{"items":{"/init":{"mode":"0755"}}}`, string(tree.Stage("org.osbuild.chmod").Options))
	assert.Contains(t, m.Sources, "org.osbuild.inline")

	cpio := m.Pipeline("initramfs-cpio").Stage("org.osbuild.cpio.out")
	require.NotNil(t, cpio)
	assert.Contains(t, string(cpio.Inputs), "name:initramfs-tree")
	assert.JSONEq(t, `{"filename":"image.cpio","format":"newc"}`, string(cpio.Options))

	compressed := m.Pipeline("initramfs").Stage("org.osbuild.zstd")
	require.NotNil(t, compressed)
	assert.JSONEq(t, `{"filename":"initrd.img","level":9}`, string(compressed.Options))
	assert.Contains(t, string(compressed.Inputs), "name:initramfs-cpio")
}
