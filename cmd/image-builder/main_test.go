package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testrepos "github.com/osbuild/images/test/data/repositories"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
	"github.com/osbuild/image-builder-cli/internal/testutil"
	"github.com/osbuild/images/internal/manifesttest"
)

func init() {
	// silence logrus by default, it is quite verbose
	logrus.SetLevel(logrus.WarnLevel)
}

func TestListImagesNoArguments(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, args := range [][]string{nil, []string{"--format=text"}} {
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

	restore = main.MockOsArgs([]string{"list-images", "--format=json"})
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
	require.Equal(t, 1, len(fakeOsbuildCmd.Calls()))
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

func TestBuildIntegrationArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	cacheDir := t.TempDir()
	for _, tc := range []struct {
		args          []string
		expectedFiles []string
	}{
		{
			nil,
			nil,
		}, {
			[]string{"--with-manifest"},
			[]string{"centos-9-qcow2-x86_64.osbuild-manifest.json"},
		}, {
			[]string{"--with-buildlog"},
			[]string{"centos-9-qcow2-x86_64.buildlog"},
		}, {
			[]string{"--with-sbom"},
			[]string{"centos-9-qcow2-x86_64.buildroot-build.spdx.json",
				"centos-9-qcow2-x86_64.image-os.spdx.json",
			},
		}, {
			[]string{"--with-manifest", "--with-sbom"},
			[]string{"centos-9-qcow2-x86_64.buildroot-build.spdx.json",
				"centos-9-qcow2-x86_64.image-os.spdx.json",
				"centos-9-qcow2-x86_64.osbuild-manifest.json",
			},
		},
	} {
		t.Run(strings.Join(tc.args, ","), func(t *testing.T) {
			outputDir := filepath.Join(t.TempDir(), "output")

			cmd := []string{
				"build",
				"qcow2",
				"--distro", "centos-9",
				"--cache", cacheDir,
				"--output-dir", outputDir,
			}
			cmd = append(cmd, tc.args...)
			restore = main.MockOsArgs(cmd)
			defer restore()

			script := `cat - > "$0".stdin`
			fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", script)
			defer fakeOsbuildCmd.Restore()

			err := main.Run()
			require.NoError(t, err)

			// ensure output dir override works
			osbuildCall := fakeOsbuildCmd.Calls()[0]
			outputDirPos := slices.Index(osbuildCall, "--output-directory")
			assert.True(t, outputDirPos > -1)
			assert.Equal(t, outputDir, osbuildCall[outputDirPos+1])

			// ensure we get exactly the expected files
			files, err := filepath.Glob(outputDir + "/*")
			assert.NoError(t, err)
			assert.Equal(t, len(tc.expectedFiles), len(files), files)
			for _, expected := range tc.expectedFiles {
				_, err = os.Stat(filepath.Join(outputDir, expected))
				assert.NoError(t, err, fmt.Sprintf("file %q missing", expected))
			}
		})
	}
}

var failingOsbuild = `
cat - > "$0".stdin
echo "error on stdout"
>&2 echo "error on stderr"

sleep 0.1
>&3 echo '{"message": "osbuild-stage-output"}'
exit 1
`

func TestBuildIntegrationErrorsProgressVerbose(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{
		"build",
		"qcow2",
		"--distro", "centos-9",
		"--progress=verbose",
	})
	defer restore()

	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", failingOsbuild)
	defer fakeOsbuildCmd.Restore()

	var err error
	stdout, stderr := testutil.CaptureStdio(t, func() {
		err = main.Run()
	})
	assert.EqualError(t, err, "error running osbuild: exit status 1")

	assert.Contains(t, stdout, "error on stdout\n")
	assert.Contains(t, stderr, "error on stderr\n")
}

func TestBuildIntegrationErrorsProgressVerboseWithBuildlog(t *testing.T) {
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
		"--progress=verbose",
		"--with-buildlog",
		"--output-dir", outputDir,
	})
	defer restore()

	failingOsbuild := `#!/bin/sh
cat - > "$0".stdin
echo "error on stdout"
>&2 echo "error on stderr"
exit 1
`
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", failingOsbuild)
	defer fakeOsbuildCmd.Restore()

	var err error
	stdout, _ := testutil.CaptureStdio(t, func() {
		err = main.Run()
	})
	assert.EqualError(t, err, "error running osbuild: exit status 1")

	// when the buildlog is used we do not get the direct output of
	// osbuild on stderr, to avoid races everything goes via stdout
	assert.Contains(t, stdout, "error on stdout\n")
	assert.Contains(t, stdout, "error on stderr\n")

	buildLog, err := os.ReadFile(filepath.Join(outputDir, "centos-9-qcow2-x86_64.buildlog"))
	assert.NoError(t, err)
	assert.Equal(t, string(buildLog), `error on stdout
error on stderr
`)
}

func TestBuildIntegrationErrorsProgressTerm(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, withBuildlog := range []bool{false, true} {
		t.Run(fmt.Sprintf("with buildlog %v", withBuildlog), func(t *testing.T) {
			outputDir := t.TempDir()
			cmd := []string{
				"build",
				"qcow2",
				"--distro", "centos-9",
				"--progress=term",
				"--output-dir", outputDir,
			}
			if withBuildlog {
				cmd = append(cmd, "--with-buildlog")
			}
			restore = main.MockOsArgs(cmd)
			defer restore()

			fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", failingOsbuild)
			defer fakeOsbuildCmd.Restore()

			var err error
			stdout, stderr := testutil.CaptureStdio(t, func() {
				err = main.Run()
			})
			assert.EqualError(t, err, `error running osbuild: exit status 1
BuildLog:
osbuild-stage-output
Output:
error on stdout
error on stderr
`)
			assert.NotContains(t, stdout, "error on stdout")
			assert.NotContains(t, stderr, "error on stderr")

			if withBuildlog {
				buildLog, err := os.ReadFile(filepath.Join(outputDir, "centos-9-qcow2-x86_64.buildlog"))
				assert.NoError(t, err)
				assert.Equal(t, string(buildLog), `error on stdout
error on stderr
osbuild-stage-output
`)
			} else {
				_, err := os.Stat(filepath.Join(outputDir, "centos-9-qcow2-x86_64.buildlog"))
				assert.True(t, os.IsNotExist(err))
			}
		})
	}
}

func TestManifestIntegrationWithSBOMWithOutputDir(t *testing.T) {
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
		"--with-sbom",
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

func TestDescribeImageSmoke(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{
		"describe-image",
		"qcow2",
		"--distro=centos-9",
		"--arch=x86_64",
	})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)

	assert.Contains(t, fakeStdout.String(), `distro: centos-9
type: qcow2
arch: x86_64`)
}

func TestProgressFromCmd(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("progress", "auto", "")
	cmd.Flags().Bool("verbose", false, "")

	for _, tc := range []struct {
		progress string
		verbose  bool
		// XXX: progress should just export the types, then
		// this would be a bit nicer
		expectedProgress string
	}{
		// we cannto test the "auto/false" case because it
		// depends on if there is a terminal attached or not
		//{"auto", false, "*progress.terminalProgressBar"},
		{"auto", true, "*progress.verboseProgressBar"},
		{"term", false, "*progress.terminalProgressBar"},
		{"term", true, "*progress.terminalProgressBar"},
	} {
		cmd.Flags().Set("progress", tc.progress)
		cmd.Flags().Set("verbose", fmt.Sprintf("%v", tc.verbose))
		pbar, err := main.ProgressFromCmd(cmd)
		assert.NoError(t, err)
		assert.Equal(t, tc.expectedProgress, fmt.Sprintf("%T", pbar))
	}
}

func TestManifestExtraRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}
	if _, err := exec.LookPath("createrepo_c"); err != nil {
		t.Skip("need createrepo_c to run this test")
	}

	localRepoDir := filepath.Join(t.TempDir(), "repo")
	err := os.MkdirAll(localRepoDir, 0755)
	assert.NoError(t, err)
	err = exec.Command("cp", "-a", "../../test/data/rpm/dummy-1.0.0-0.noarch.rpm", localRepoDir).Run()
	assert.NoError(t, err)
	err = exec.Command("createrepo_c", localRepoDir).Run()
	assert.NoError(t, err)

	pkgHelloBlueprint := `{
          "packages": [
            {"name":"dummy"}
          ]
        }`
	restore := main.MockOsArgs([]string{
		"manifest",
		"qcow2",
		"--distro=centos-9",
		fmt.Sprintf("--extra-repo=file://%s", localRepoDir),
		"--blueprint", makeTestBlueprint(t, pkgHelloBlueprint),
	})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err = main.Run()
	require.NoError(t, err)

	// our local repo got added
	assert.Contains(t, fakeStdout.String(), `"path":"dummy-1.0.0-0.noarch.rpm"`)
	assert.Contains(t, fakeStdout.String(), fmt.Sprintf(`"url":"file://%s"`, localRepoDir))
}

func TestManifestOverrideRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	var fakeStderr bytes.Buffer
	restore := main.MockOsStderr(&fakeStderr)
	defer restore()

	restore = main.MockOsArgs([]string{
		"manifest",
		"qcow2",
		"--distro=centos-9",
		"--arch=x86_64",
		"--force-repo=http://xxx.abcdefgh-no-such-host.com/repo",
	})
	defer restore()

	// XXX: dnfjson is very chatty and puts a bunch of output on stderr
	// we should probably silence this in images as its the job of the
	// error to catpure this. Use CaptureStdio here to ensure we don't
	// get noisy and confusing errors when this test runs.
	var err error
	testutil.CaptureStdio(t, func() {
		err = main.Run()
	})
	assert.ErrorContains(t, err, "forced repo#0 xxx.abcdefgh-no-such-host.com/repo: http://xxx.abcdefgh-no-such-host.com/repo]: Cannot download repomd.xml")
	// XXX: we should probably look into "images" here, there is a bunch
	// of redundancy in the full error message:
	//
	// `error depsolving: running osbuild-depsolve-dnf failed:
	// DNF error occurred: RepoError: There was a problem reading a repository: Failed to download metadata for repo '9828718901ab404ac1b600157aec1a8b19f4b2139e7756f347fb0ecc06c92929' [forced repo#0 xxx.abcdefgh-no-such-host.com/repo: http://xxx.abcdefgh-no-such-host.com/repo]: Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried`
}
