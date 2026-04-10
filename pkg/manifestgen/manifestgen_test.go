package manifestgen_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/manifestgen"
	"github.com/osbuild/images/pkg/manifestgen/manifestmock"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/osbuild/manifesttest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	testrepos "github.com/osbuild/images/test/data/repositories"
)

func sha256For(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}

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
			osbuildManifest, err := mg.Generate(&bp, res[0].ImgType, nil)
			require.NoError(t, err)

			pipelineNames, err := manifesttest.PipelineNamesFrom(osbuildManifest)
			assert.NoError(t, err)
			assert.Equal(t, []string{"build", "os", "image", "qcow2"}, pipelineNames)

			// Verify depsolving produced source items
			if useLibrepo {
				// Librepo uses Location field for path
				// Format: "sources":{... "org.osbuild.librepo":{... "items":{... "sha256:...":{"path":"packages/...","mirror":"..."}}}
				sourcesPattern := regexp.MustCompile(
					`"sources":\{"org\.osbuild\.librepo":\{"items":\{"sha256:[a-f0-9]{64}":\{"path":"packages/[^"]+\.rpm","mirror":"[^"]+"\}`)
				assert.Regexp(t, sourcesPattern, string(osbuildManifest))
				assert.NotContains(t, string(osbuildManifest), "org.osbuild.curl")
			} else {
				// Curl uses RemoteLocations for URL
				// Format: "sources":{... "org.osbuild.curl":{... "items":{... "sha256:...":{"url":"https://..."}}}
				sourcesPattern := regexp.MustCompile(
					`"sources":\{"org\.osbuild\.curl":\{"items":\{"sha256:[a-f0-9]{64}":\{"url":"https://[^"]+/packages/[^"]+\.rpm"\}`)
				assert.Regexp(t, sourcesPattern, string(osbuildManifest))
				assert.NotContains(t, string(osbuildManifest), "org.osbuild.librepo")
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
	osbuildManifest, err := mg.Generate(&bp, res[0].ImgType, imageOpts)
	assert.NoError(t, err)

	pipelineNames, err := manifesttest.PipelineNamesFrom(osbuildManifest)
	assert.NoError(t, err)
	assert.Equal(t, []string{"build", "ostree-deployment", "image"}, pipelineNames)

	// XXX: add testhelper to manifesttest for this
	assert.Contains(t, string(osbuildManifest), `{"url":"resolved-url-for-centos/9/x86_64/edge"}`)
	// Test that depsolving happened by checking that sources contain curl items with sha256 checksums and URLs
	sourcesPattern := regexp.MustCompile(
		`"sources":\{"org\.osbuild\.curl":\{"items":\{"sha256:[a-f0-9]{64}":\{"url":"https://[^"]+/packages/[^"]+\.rpm"\}`)
	assert.Regexp(t, sourcesPattern, string(osbuildManifest))
}

func fakeDepsolve(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
	if depsolveWarningsOutput != nil {
		_, _ = depsolveWarningsOutput.Write([]byte(`fake depsolve output`))
	}
	depsolvedSets, err := manifestmock.Depsolve(packageSets, arch, nil, true)
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
				Digest:  "sha256:" + sha256For("digest:"+spec.Source),
				ImageID: "sha256:" + sha256For("id:"+spec.Source),
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
	osbuildManifest, err := mg.Generate(&bp, res[0].ImgType, nil)
	assert.NoError(t, err)

	// container is included
	assert.Contains(t, string(osbuildManifest), "resolved-cnt-"+fakeContainerSource)
}

func TestManifestGeneratorDepsolveWithSbomWriter(t *testing.T) {
	repos, err := testrepos.New()
	assert.NoError(t, err)
	fac := distrofactory.NewDefault()

	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:qcow2", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	generatedSboms := map[string]string{}
	opts := &manifestgen.Options{
		Depsolve:          fakeDepsolve,
		CommitResolver:    panicCommitResolver,
		ContainerResolver: panicContainerResolver,

		SBOMWriter: func(filename string, content io.Reader, docType sbom.StandardType) error {
			assert.Equal(t, sbom.StandardTypeSpdx, docType)

			b, err := io.ReadAll(content)
			assert.NoError(t, err)
			generatedSboms[filename] = strings.TrimSpace(string(b))
			return nil
		},
	}
	mg, err := manifestgen.New(repos, opts)
	assert.NoError(t, err)
	assert.NotNil(t, mg)
	var bp blueprint.Blueprint
	_, err = mg.Generate(&bp, res[0].ImgType, nil)
	require.NoError(t, err)

	assert.Contains(t, generatedSboms, "centos-9-qcow2-x86_64.buildroot-build.spdx.json")
	assert.Contains(t, generatedSboms, "centos-9-qcow2-x86_64.image-os.spdx.json")
	expected := map[string]string{
		"centos-9-qcow2-x86_64.buildroot-build.spdx.json": `{"sbom-for":"build"}`,
		"centos-9-qcow2-x86_64.image-os.spdx.json":        `{"sbom-for":"os"}`,
	}
	assert.Equal(t, expected, generatedSboms)
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
		osbuildManifest, err := mg.Generate(&bp, res[0].ImgType, nil)
		assert.NoError(t, err)

		// with the customSeed we always get a predicatable uuid for
		// the xfs boot partition
		needle := `8b9968ba-f910-4259-915b-f7025a291b00`
		if withCustomSeed {
			assert.Contains(t, string(osbuildManifest), needle)
		} else {
			assert.NotContains(t, string(osbuildManifest), needle)
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
			osbuildManifest, err := mg.Generate(&bp, res[0].ImgType, nil)
			assert.NoError(t, err)
			if withOverrideRepos {
				assert.Contains(t, string(osbuildManifest), "http://example.com/overriden-repo/")
			} else {
				assert.NotContains(t, string(osbuildManifest), "http://example.com/overriden-repo/")
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
			osbuildManifest, err := mg.Generate(&bp, res[0].ImgType, nil)
			assert.NoError(t, err)
			pipelines, err := manifesttest.PipelineNamesFrom(osbuildManifest)
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
