package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	testrepos "github.com/osbuild/images/test/data/repositories"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
	"github.com/osbuild/image-builder-cli/internal/manifesttest"
	"github.com/osbuild/image-builder-cli/internal/testutil"
)

func init() {
	// silence logrus by default, it is quite verbose
	logrus.SetLevel(logrus.WarnLevel)
}

func TestListImagesNoArguments(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, args := range [][]string{nil, []string{"--output=text"}} {
		restore = main.MockOsArgs(append([]string{"list-images"}, args...))
		defer restore()

		var fakeStdout bytes.Buffer
		restore = main.MockOsStdout(&fakeStdout)
		defer restore()

		err := main.Run()
		assert.NoError(t, err)
		// we expect at least this canary
		assert.Contains(t, fakeStdout.String(), "rhel-10.0 type:qcow2 arch:x86_64\n")
		// output is sorted, i.e. 8.9 comes before 8.10
		assert.Regexp(t, `(?ms)rhel-8.9.*rhel-8.10`, fakeStdout.String())
	}
}

func TestListImagesNoArgsOutputJSON(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{"list-images", "--output=json"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)

	// smoke test only, we expect valid json and at least the
	// distro/arch/image_type keys in the json
	var jo []map[string]interface{}
	err = json.Unmarshal(fakeStdout.Bytes(), &jo)
	assert.NoError(t, err)
	res := jo[0]
	for _, key := range []string{"distro", "arch", "image_type"} {
		assert.NotNil(t, res[key])
	}
}

func TestListImagesFilteringSmoke(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{"list-images", "--filter=centos*"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)
	// we have centos
	assert.Contains(t, fakeStdout.String(), "centos-9 type:qcow2 arch:x86_64\n")
	// but not rhel
	assert.NotContains(t, fakeStdout.String(), "rhel")
}

func TestBadCmdErrorsNoExtraCobraNoise(t *testing.T) {
	var fakeStderr bytes.Buffer
	restore := main.MockOsStderr(&fakeStderr)
	defer restore()

	restore = main.MockOsArgs([]string{"bad-command"})
	defer restore()

	err := main.Run()
	assert.EqualError(t, err, `unknown command "bad-command" for "image-builder"`)
	// no extra output from cobra
	assert.Equal(t, "", fakeStderr.String())
}

func TestListImagesErrorsOnExtraArgs(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs(append([]string{"list-images"}, "extra-arg"))
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.EqualError(t, err, `unknown command "extra-arg" for "image-builder list-images"`)
}

func hasDepsolveDnf() bool {
	// XXX: expose images/pkg/depsolve:findDepsolveDnf()
	_, err := os.Stat("/usr/libexec/osbuild-depsolve-dnf")
	return err == nil
}

var testBlueprint = `{
  "containers": [
    {
      "source": "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal"
    }
  ],
  "customizations": {
    "user": [
      {
	"name": "alice"
      }
    ]
  }
}`

func makeTestBlueprint(t *testing.T, testBlueprint string) string {
	tmpdir := t.TempDir()
	blueprintPath := filepath.Join(tmpdir, "blueprint.json")
	err := os.WriteFile(blueprintPath, []byte(testBlueprint), 0644)
	assert.NoError(t, err)
	return blueprintPath
}

// XXX: move to pytest like bib maybe?
func TestManifestIntegrationSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, useLibrepo := range []bool{false, true} {
		t.Run(fmt.Sprintf("use-librepo: %v", useLibrepo), func(t *testing.T) {
			restore = main.MockOsArgs([]string{
				"manifest",
				"qcow2",
				"--arch=x86_64",
				"--distro=centos-9",
				fmt.Sprintf("--blueprint=%s", makeTestBlueprint(t, testBlueprint)),
				fmt.Sprintf("--use-librepo=%v", useLibrepo),
			})
			defer restore()

			var fakeStdout bytes.Buffer
			restore = main.MockOsStdout(&fakeStdout)
			defer restore()

			err := main.Run()
			assert.NoError(t, err)

			pipelineNames, err := manifesttest.PipelineNamesFrom(fakeStdout.Bytes())
			assert.NoError(t, err)
			assert.Contains(t, pipelineNames, "qcow2")

			// XXX: provide helpers in manifesttest to extract this in a nicer way
			assert.Contains(t, fakeStdout.String(), `{"type":"org.osbuild.users","options":{"users":{"alice":{}}}}`)
			assert.Contains(t, fakeStdout.String(), `"image":{"name":"registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal"`)

			assert.Equal(t, strings.Contains(fakeStdout.String(), "org.osbuild.librepo"), useLibrepo)
		})
	}
}

func TestManifestIntegrationCrossArch(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{
		"manifest",
		"tar",
		"--distro", "centos-9",
		"--arch", "s390x",
	})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)

	pipelineNames, err := manifesttest.PipelineNamesFrom(fakeStdout.Bytes())
	assert.NoError(t, err)
	assert.Contains(t, pipelineNames, "archive")

	// XXX: provide helpers in manifesttest to extract this in a nicer way
	assert.Contains(t, fakeStdout.String(), `.el9.s390x.rpm`)
}

func TestManifestIntegrationOstreeSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	// we cannot hit ostree.f.o directly, we need to go via the mirrorlist
	resp, err := http.Get("https://ostree.fedoraproject.org/iot/mirrorlist")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	restore = main.MockOsArgs([]string{
		"manifest",
		"iot-raw-image",
		"--arch=x86_64",
		"--distro=fedora-40",
		"--ostree-url=" + strings.SplitN(string(body), "\n", 2)[0],
		"--ostree-ref=fedora/stable/x86_64/iot",
	})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err = main.Run()
	assert.NoError(t, err)

	pipelineNames, err := manifesttest.PipelineNamesFrom(fakeStdout.Bytes())
	assert.NoError(t, err)
	assert.Contains(t, pipelineNames, "ostree-deployment")

	// XXX: provide helpers in manifesttest to extract this in a nicer way
	assert.Contains(t, fakeStdout.String(), `{"type":"org.osbuild.ostree.init-fs"`)
}

func TestManifestIntegrationOstreeSmokeErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	baseArgs := []string{
		"manifest",
		"--arch=x86_64",
		"--distro=fedora-40",
	}

	for _, tc := range []struct {
		extraArgs   []string
		expectedErr string
	}{
		{
			[]string{"iot-raw-image"},
			`iot-raw-image: ostree commit URL required`,
		},
		{
			[]string{"qcow2", "--ostree-url=http://example.com/"},
			`OSTree is not supported for "qcow2"`,
		},
	} {
		args := append(baseArgs, tc.extraArgs...)
		restore = main.MockOsArgs(args)
		defer restore()

		var fakeStdout bytes.Buffer
		restore = main.MockOsStdout(&fakeStdout)
		defer restore()

		err := main.Run()
		assert.EqualError(t, err, tc.expectedErr)
	}
}

func TestBuildIntegrationHappy(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	tmpdir := t.TempDir()
	restore = main.MockOsArgs([]string{
		"build",
		"qcow2",
		fmt.Sprintf("--blueprint=%s", makeTestBlueprint(t, testBlueprint)),
		"--distro", "centos-9",
		"--cache", tmpdir,
	})
	defer restore()

	script := `cat - > "$0".stdin`
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", script)
	defer fakeOsbuildCmd.Restore()

	err := main.Run()
	assert.NoError(t, err)

	// ensure osbuild was run exactly one
	assert.Equal(t, 1, len(fakeOsbuildCmd.Calls()))
	osbuildCall := fakeOsbuildCmd.Calls()[0]
	// --cache is passed correctly to osbuild
	storePos := slices.Index(osbuildCall, "--store")
	assert.True(t, storePos > -1)
	assert.Equal(t, tmpdir, osbuildCall[storePos+1])
	// and we passed the output dir
	outputDirPos := slices.Index(osbuildCall, "--output-directory")
	assert.True(t, outputDirPos > -1)
	assert.Equal(t, "centos-9-qcow2-x86_64", osbuildCall[outputDirPos+1])

	// ... and that the manifest passed to osbuild
	manifest, err := os.ReadFile(fakeOsbuildCmd.Path() + ".stdin")
	assert.NoError(t, err)
	// XXX: provide helpers in manifesttest to extract this in a nicer way
	assert.Contains(t, string(manifest), `{"type":"org.osbuild.users","options":{"users":{"alice":{}}}}`)
	assert.Contains(t, string(manifest), `"image":{"name":"registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal"`)
}

func TestBuildIntegrationSwitchOutputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	tmpdir := t.TempDir()
	restore = main.MockOsArgs([]string{
		"build",
		"qcow2",
		"--distro", "centos-9",
		"--cache", tmpdir,
		"--output-dir", "some-output-dir",
	})
	defer restore()

	script := `cat - > "$0".stdin`
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", script)
	defer fakeOsbuildCmd.Restore()

	err := main.Run()
	assert.NoError(t, err)

	// ensure output dir override works
	osbuildCall := fakeOsbuildCmd.Calls()[0]
	outputDirPos := slices.Index(osbuildCall, "--output-directory")
	assert.True(t, outputDirPos > -1)
	assert.Equal(t, "some-output-dir", osbuildCall[outputDirPos+1])
}

func TestBuildIntegrationExtraArtifactsManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	outputDir := t.TempDir()
	restore = main.MockOsArgs([]string{
		"build",
		"qcow2",
		"--distro", "centos-9",
		"--cache", outputDir,
		"--extra-artifacts", "manifest",
		"--output-dir", outputDir,
	})
	defer restore()

	script := `cat - > "$0".stdin`
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", script)
	defer fakeOsbuildCmd.Restore()

	err := main.Run()
	assert.NoError(t, err)

	manifest, err := filepath.Glob(filepath.Join(outputDir, "*.osbuild-manifest.json"))
	assert.Equal(t, len(manifest), 1)
	assert.Equal(t, filepath.Join(outputDir, "centos-9-qcow2-x86_64.osbuild-manifest.json"), manifest[0])

}

func TestBuildIntegrationErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	var fakeStdout, fakeStderr bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()
	restore = main.MockOsStderr(&fakeStderr)
	defer restore()

	restore = main.MockOsArgs([]string{
		"build",
		"qcow2",
		"--distro", "centos-9",
	})
	defer restore()

	script := `
cat - > "$0".stdin
>&2 echo "error on stderr"
exit 1
`
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", script)
	defer fakeOsbuildCmd.Restore()

	err := main.Run()
	assert.EqualError(t, err, "running osbuild failed: exit status 1")
	// ensure errors from osbuild are passed to the user
	// XXX: once the osbuild.Status is used, also check that stdout
	// is available (but that cannot be done with the existing
	// osbuild-exec.go)
	assert.Equal(t, "error on stderr\n", fakeStderr.String())
}

func TestManifestIntegrationExtraArtifactsSBOMWithOutputDir(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}
	outputDir := filepath.Join(t.TempDir(), "output-dir")

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{
		"manifest",
		"qcow2",
		"--arch=x86_64",
		"--distro=centos-9",
		fmt.Sprintf("--blueprint=%s", makeTestBlueprint(t, testBlueprint)),
		"--extra-artifacts=sbom",
		"--output-dir", outputDir,
	})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)

	sboms, err := filepath.Glob(filepath.Join(outputDir, "*.spdx.json"))
	assert.NoError(t, err)
	assert.Equal(t, len(sboms), 2)
	assert.Equal(t, filepath.Join(outputDir, "centos-9-qcow2-x86_64.buildroot-build.spdx.json"), sboms[0])
	assert.Equal(t, filepath.Join(outputDir, "centos-9-qcow2-x86_64.image-os.spdx.json"), sboms[1])
}
