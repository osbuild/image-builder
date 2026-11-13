package manifestgen_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/testutil"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/distrofactory"
	"github.com/osbuild/image-builder/pkg/imagefilter"
	"github.com/osbuild/image-builder/pkg/manifestgen"
	"github.com/osbuild/image-builder/pkg/manifestgen/manifestmock"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/osbuild/manifesttest"
	"github.com/osbuild/image-builder/pkg/ostree"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/sbom"
	testrepos "github.com/osbuild/image-builder/test/data/repositories"
)

func TestManifestGeneratorDepsolve(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	for _, useLibrepo := range []bool{false, true} {
		t.Run(fmt.Sprintf("useLibrepo: %v", useLibrepo), func(t *testing.T) {
			var rpmDownloader osbuild.RpmDownloader
			if useLibrepo {
				rpmDownloader = osbuild.RpmDownloaderLibrepo
			}

			opts := &manifestgen.Options{
				Depsolve:          fakeDepsolve,
				CommitResolver:    panicCommitResolver,
				ContainerResolver: panicContainerResolver,

				RpmDownloader: rpmDownloader,
			}
			mg, err := manifestgen.New(repos, opts)
			assert.NoError(t, err)
			assert.NotNil(t, mg)
			var bp blueprint.Blueprint
			result, err := mg.Generate(&bp, res[0].ImgType, nil)
			require.NoError(t, err)

			pipelineNames, err := manifesttest.PipelineNamesFrom(result.Manifest)
			assert.NoError(t, err)
			assert.Equal(t, []string{"build", "os", "image", "qcow2"}, pipelineNames)

			// Verify depsolving produced source items
			if useLibrepo {
				// Librepo uses Location field for path
				// Format: "sources":{... "org.osbuild.librepo":{... "items":{... "sha256:...":{"path":"packages/...","mirror":"..."}}}
				sourcesPattern := regexp.MustCompile(
					`"sources":\{"org\.osbuild\.librepo":\{"items":\{"sha256:[a-f0-9]{64}":\{"path":"packages/[^"]+\.rpm","mirror":"[^"]+"\}`)
				assert.Regexp(t, sourcesPattern, string(result.Manifest))
				assert.NotContains(t, string(result.Manifest), "org.osbuild.curl")
			} else {
				// Curl uses RemoteLocations for URL
				// Format: "sources":{... "org.osbuild.curl":{... "items":{... "sha256:...":{"url":"https://..."}}}
				sourcesPattern := regexp.MustCompile(
					`"sources":\{"org\.osbuild\.curl":\{"items":\{"sha256:[a-f0-9]{64}":\{"url":"https://[^"]+/packages/[^"]+\.rpm"\}`)
				assert.Regexp(t, sourcesPattern, string(result.Manifest))
				assert.NotContains(t, string(result.Manifest), "org.osbuild.librepo")
			}
		})
	}
}

func TestManifestGeneratorWithOstreeCommit(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)

	fac := distrofactory.NewDefault()
	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:edge-ami", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	opts := &manifestgen.Options{
		Depsolve:          fakeDepsolve,
		CommitResolver:    fakeCommitResolver,
		ContainerResolver: panicContainerResolver,
	}
	imageOpts := &distro.ImageOptions{
		OSTree: &ostree.ImageOptions{
			//ImageRef: "latest/1/x86_64/edge",
			URL: "http://example.com/",
		},
	}
	mg, err := manifestgen.New(repos, opts)
	assert.NoError(t, err)
	assert.NotNil(t, mg)
	var bp blueprint.Blueprint
	result, err := mg.Generate(&bp, res[0].ImgType, imageOpts)
	assert.NoError(t, err)

	pipelineNames, err := manifesttest.PipelineNamesFrom(result.Manifest)
	assert.NoError(t, err)
	assert.Equal(t, []string{"build", "ostree-deployment", "image"}, pipelineNames)

	// XXX: add testhelper to manifesttest for this
	assert.Contains(t, string(result.Manifest), `{"url":"resolved-url-for-centos/9/x86_64/edge"}`)
	// Test that depsolving happened by checking that sources contain curl items with sha256 checksums and URLs
	sourcesPattern := regexp.MustCompile(
		`"sources":\{"org\.osbuild\.curl":\{"items":\{"sha256:[a-f0-9]{64}":\{"url":"https://[^"]+/packages/[^"]+\.rpm"\}`)
	assert.Regexp(t, sourcesPattern, string(result.Manifest))
}

func fakeDepsolve(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
	if depsolveWarningsOutput != nil {
		_, _ = depsolveWarningsOutput.Write([]byte(`fake depsolve output`))
	}
	depsolvedSets, err := manifestmock.Depsolve(packageSets, arch, nil)
	if err != nil {
		return nil, err
	}
	return depsolvedSets, nil
}

func fakeCommitResolver(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error) {
	commits := make(map[string][]ostree.CommitSpec, len(commitSources))
	for name, commitSources := range commitSources {
		commitSpecs := make([]ostree.CommitSpec, len(commitSources))
		for idx, commitSource := range commitSources {
			commitSpecs[idx] = ostree.CommitSpec{
				URL: fmt.Sprintf("resolved-url-for-%s", commitSource.Ref),
			}
		}
		commits[name] = commitSpecs
	}
	return commits, nil

}

func panicCommitResolver(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error) {
	if len(commitSources) > 0 {
		panic("panicCommitResolver")
	}
	return nil, nil
}

func fakeContainerResolver(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {
	containerSpecs := make(map[string][]container.Spec, len(containerSources))
	for plName, sourceSpecs := range containerSources {
		var containers []container.Spec
		for _, spec := range sourceSpecs {
			containers = append(containers, container.Spec{
				Source:  fmt.Sprintf("resolved-cnt-%s", spec.Source),
				Digest:  "sha256:" + testutil.SHA256For("digest:"+spec.Source),
				ImageID: "sha256:" + testutil.SHA256For("id:"+spec.Source),
				Arch:    common.Must(arch.FromString(archName)),
			})
		}
		containerSpecs[plName] = containers
	}
	return containerSpecs, nil
}

func panicContainerResolver(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {
	if len(containerSources) > 0 {
		panic("panicContainerResolver")
	}
	return nil, nil
}

func TestManifestGeneratorContainers(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	opts := &manifestgen.Options{
		Depsolve:          fakeDepsolve,
		CommitResolver:    panicCommitResolver,
		ContainerResolver: fakeContainerResolver,
	}
	mg, err := manifestgen.New(repos, opts)
	assert.NoError(t, err)
	assert.NotNil(t, mg)
	fakeContainerSource := "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal"
	bp := blueprint.Blueprint{
		Containers: []blueprint.Container{
			{
				Source: fakeContainerSource,
			},
		},
	}
	result, err := mg.Generate(&bp, res[0].ImgType, nil)
	assert.NoError(t, err)

	// container is included
	assert.Contains(t, string(result.Manifest), "resolved-cnt-"+fakeContainerSource)
}

func TestManifestGeneratorWithRPMListWriter(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	generated := map[string]string{}
	opts := &manifestgen.Options{
		Depsolve:          fakeDepsolve,
		CommitResolver:    panicCommitResolver,
		ContainerResolver: panicContainerResolver,
		RPMListWriter: func(filename string, content io.Reader) error {
			b, err := io.ReadAll(content)
			assert.NoError(t, err)
			generated[filename] = strings.TrimSpace(string(b))
			return nil
		},
	}
	mg, err := manifestgen.New(repos, opts)
	assert.NoError(t, err)
	assert.NotNil(t, mg)
	var bp blueprint.Blueprint
	_, err = mg.Generate(&bp, res[0].ImgType, nil)
	require.NoError(t, err)

	assert.Contains(t, generated, "rpmlist.json")
	require.Len(t, generated, 1)
	var rows []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(generated["rpmlist.json"]), &rows))
	assert.NotEmpty(t, rows)
}

// Payload RPMs include all installation transactions. Buildroot-only RPMs
// are excluded. Packages shared between payload pipelines are kept.
// sbom.Package has no field for repo URLs or secrets; the exact match guards that.
func TestGenerateReturnsPayloadPackages(t *testing.T) {
	repos, err := testrepos.New()
	require.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	require.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	require.NoError(t, err)
	require.Len(t, res, 1)

	imgType := res[0].ImgType
	a := imgType.Arch()
	imgRepos, err := repos.ReposByImageTypeName(a.Distro().Name(), a.Name(), imgType.Name())
	require.NoError(t, err)

	repo := imgRepos[0]
	payloadOnly := rpmmd.Package{
		Name:    "payload-only",
		Version: "1.0",
		Release: "1.fixture",
		Arch:    "x86_64",
		Vendor:  "Fixture Vendor",
		URL:     "https://example.test/payload-only",
		Checksum: rpmmd.Checksum{
			Type:  "sha256",
			Value: strings.Repeat("a", 64),
		},
		RepoID:          repo.Id,
		Repo:            &repo,
		Location:        "packages/payload-only.rpm",
		RemoteLocations: []string{repo.BaseURLs[0] + "/packages/payload-only.rpm"},
		Secrets:         "org.osbuild.rhsm",
	}
	buildrootOnly := rpmmd.Package{
		Name:    "buildroot-only",
		Version: "1.0",
		Release: "1.fixture",
		Arch:    "x86_64",
		Vendor:  "Fixture Vendor",
		URL:     "https://example.test/buildroot-only",
		Checksum: rpmmd.Checksum{
			Type:  "sha256",
			Value: strings.Repeat("b", 64),
		},
		RepoID:          repo.Id,
		Repo:            &repo,
		Location:        "packages/buildroot-only.rpm",
		RemoteLocations: []string{repo.BaseURLs[0] + "/packages/buildroot-only.rpm"},
		Secrets:         "org.osbuild.rhsm",
	}
	both := rpmmd.Package{
		Name:    "both",
		Epoch:   2,
		Version: "1.0",
		Release: "1.fixture",
		Arch:    "x86_64",
		Vendor:  "Fixture Vendor",
		URL:     "https://example.test/both",
		Checksum: rpmmd.Checksum{
			Type:  "sha256",
			Value: strings.Repeat("c", 64),
		},
		RepoID:          repo.Id,
		Repo:            &repo,
		Location:        "packages/both.rpm",
		RemoteLocations: []string{repo.BaseURLs[0] + "/packages/both.rpm"},
		Secrets:         "org.osbuild.rhsm",
	}
	// The OS pipeline looks up a package named kernel while serializing.
	kernel := rpmmd.Package{
		Name:    "kernel",
		Version: "1.0",
		Release: "1.fixture",
		Arch:    "x86_64",
		Vendor:  "Fixture Vendor",
		URL:     "https://example.test/kernel",
		Checksum: rpmmd.Checksum{
			Type:  "sha256",
			Value: strings.Repeat("d", 64),
		},
		RepoID:          repo.Id,
		Repo:            &repo,
		Location:        "packages/kernel.rpm",
		RemoteLocations: []string{repo.BaseURLs[0] + "/packages/kernel.rpm"},
		Secrets:         "org.osbuild.rhsm",
	}
	controlledDepsolve := func(_ *depsolvednf.Solver, _ string, _ io.Writer, _ map[string][]rpmmd.PackageSet, _ distro.Distro, _ string) (map[string]depsolvednf.DepsolveResult, error) {
		return map[string]depsolvednf.DepsolveResult{
			"os":    {Transactions: depsolvednf.TransactionList{{both}, {payloadOnly, kernel}}},
			"image": {Transactions: depsolvednf.TransactionList{{both}}},
			"build": {Transactions: depsolvednf.TransactionList{{buildrootOnly, both}}},
		}, nil
	}

	want := []sbom.Package{
		{
			Name:    "both",
			Epoch:   2,
			Version: "1.0",
			Release: "1.fixture",
			Arch:    "x86_64",
			Vendor:  "Fixture Vendor",
			URL:     "https://example.test/both",
			Checksum: rpmmd.Checksum{
				Type:  "sha256",
				Value: strings.Repeat("c", 64),
			},
		},
		{
			Name:    "kernel",
			Version: "1.0",
			Release: "1.fixture",
			Arch:    "x86_64",
			Vendor:  "Fixture Vendor",
			URL:     "https://example.test/kernel",
			Checksum: rpmmd.Checksum{
				Type:  "sha256",
				Value: strings.Repeat("d", 64),
			},
		},
		{
			Name:    "payload-only",
			Version: "1.0",
			Release: "1.fixture",
			Arch:    "x86_64",
			Vendor:  "Fixture Vendor",
			URL:     "https://example.test/payload-only",
			Checksum: rpmmd.Checksum{
				Type:  "sha256",
				Value: strings.Repeat("a", 64),
			},
		},
		{
			Name:    "both",
			Epoch:   2,
			Version: "1.0",
			Release: "1.fixture",
			Arch:    "x86_64",
			Vendor:  "Fixture Vendor",
			URL:     "https://example.test/both",
			Checksum: rpmmd.Checksum{
				Type:  "sha256",
				Value: strings.Repeat("c", 64),
			},
		},
	}
	for _, tt := range []struct {
		name         string
		sbomPackages bool
		want         []sbom.Package
	}{
		{
			name:         "requested",
			sbomPackages: true,
			want:         want,
		},
		{
			name: "option unset",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var bp blueprint.Blueprint
			mg, err := manifestgen.New(repos, &manifestgen.Options{
				SBOMPackages:      tt.sbomPackages,
				Depsolve:          controlledDepsolve,
				CommitResolver:    panicCommitResolver,
				ContainerResolver: panicContainerResolver,
			})
			require.NoError(t, err)

			result, err := mg.Generate(&bp, imgType, nil)
			require.NoError(t, err)
			require.NotEmpty(t, result.Manifest)
			assert.Equal(t, tt.want, result.Packages)
		})
	}
}

func TestManifestGeneratorSeed(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	for _, withCustomSeed := range []bool{false, true} {
		opts := &manifestgen.Options{
			Depsolve: fakeDepsolve,
		}
		if withCustomSeed {
			customSeed := int64(123)
			opts.CustomSeed = &customSeed
		}

		mg, err := manifestgen.New(repos, opts)
		assert.NoError(t, err)

		var bp blueprint.Blueprint
		result, err := mg.Generate(&bp, res[0].ImgType, nil)
		assert.NoError(t, err)

		// with the customSeed we always get a predicatable uuid for
		// the xfs boot partition
		needle := `8b9968ba-f910-4259-915b-f7025a291b00`
		if withCustomSeed {
			assert.Contains(t, string(result.Manifest), needle)
		} else {
			assert.NotContains(t, string(result.Manifest), needle)
		}
	}
}

func TestManifestGeneratorDepsolveOutput(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	var depsolveWarningsOutput bytes.Buffer
	opts := &manifestgen.Options{
		Depsolve:               fakeDepsolve,
		DepsolveWarningsOutput: &depsolveWarningsOutput,
	}

	mg, err := manifestgen.New(repos, opts)
	assert.NoError(t, err)

	var bp blueprint.Blueprint
	_, err = mg.Generate(&bp, res[0].ImgType, nil)
	assert.NoError(t, err)

	assert.Equal(t, []byte("fake depsolve output"), depsolveWarningsOutput.Bytes())
}

func TestManifestGeneratorOverrideRepos(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	for _, withOverrideRepos := range []bool{false, true} {
		t.Run(fmt.Sprintf("withOverrideRepos: %v", withOverrideRepos), func(t *testing.T) {
			opts := &manifestgen.Options{
				Depsolve: fakeDepsolve,
			}
			if withOverrideRepos {
				opts.OverrideRepos = []rpmmd.RepoConfig{
					{
						Name:     "overriden_repo",
						BaseURLs: []string{"http://example.com/overriden-repo"},
					},
				}
			}

			mg, err := manifestgen.New(repos, opts)
			assert.NoError(t, err)

			var bp blueprint.Blueprint
			result, err := mg.Generate(&bp, res[0].ImgType, nil)
			assert.NoError(t, err)
			if withOverrideRepos {
				assert.Contains(t, string(result.Manifest), "http://example.com/overriden-repo/")
			} else {
				assert.NotContains(t, string(result.Manifest), "http://example.com/overriden-repo/")
			}
		})
	}
}

func TestManifestGeneratorUseBootstrapContainer(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	for _, useBootstrapContainer := range []bool{false, true} {
		t.Run(fmt.Sprintf("useBootstrapContainer: %v", useBootstrapContainer), func(t *testing.T) {
			opts := &manifestgen.Options{
				Depsolve:              fakeDepsolve,
				ContainerResolver:     fakeContainerResolver,
				UseBootstrapContainer: useBootstrapContainer,
			}

			mg, err := manifestgen.New(repos, opts)
			assert.NoError(t, err)

			var bp blueprint.Blueprint
			result, err := mg.Generate(&bp, res[0].ImgType, nil)
			assert.NoError(t, err)
			pipelines, err := manifesttest.PipelineNamesFrom(result.Manifest)
			assert.NoError(t, err)
			needle := "bootstrap-buildroot"
			if useBootstrapContainer {
				// XXX: it would be nice to test more
				// precisely here but we don't have
				// great support for that yet
				assert.Contains(t, pipelines, needle)
			} else {
				assert.NotContains(t, pipelines, needle)
			}
		})
	}
}
