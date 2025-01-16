package manifestgen_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
	testrepos "github.com/osbuild/images/test/data/repositories"

	"github.com/osbuild/image-builder-cli/internal/manifestgen"
	"github.com/osbuild/image-builder-cli/internal/manifesttest"
)

func init() {
	// silence logrus by default, it is quite verbose
	logrus.SetLevel(logrus.WarnLevel)
}

func sha256For(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return fmt.Sprintf("sha256:%x", bs)
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

			var osbuildManifest bytes.Buffer
			opts := &manifestgen.Options{
				Output:            &osbuildManifest,
				Depsolver:         fakeDepsolve,
				CommitResolver:    panicCommitResolver,
				ContainerResolver: panicContainerResolver,

				RpmDownloader: rpmDownloader,
			}
			mg, err := manifestgen.New(repos, opts)
			assert.NoError(t, err)
			assert.NotNil(t, mg)
			var bp blueprint.Blueprint
			err = mg.Generate(&bp, res[0].Distro, res[0].ImgType, res[0].Arch, nil)
			require.NoError(t, err)

			pipelineNames, err := manifesttest.PipelineNamesFrom(osbuildManifest.Bytes())
			assert.NoError(t, err)
			assert.Equal(t, []string{"build", "os", "image", "qcow2"}, pipelineNames)

			// we expect at least a "kernel" package in the manifest,
			// sadly the test distro does not really generate much here so we
			// need to use this as a canary that resolving happend
			// XXX: add testhelper to manifesttest for this
			expectedSha256 := sha256For("kernel")
			assert.Contains(t, osbuildManifest.String(), expectedSha256)

			assert.Equal(t, strings.Contains(osbuildManifest.String(), "org.osbuild.librepo"), useLibrepo)
		})
	}
}

func TestManifestGeneratorWithOstreeCommit(t *testing.T) {
	var osbuildManifest bytes.Buffer

	repos, err := testrepos.New()
	assert.NoError(t, err)

	fac := distrofactory.NewDefault()
	filter, err := imagefilter.New(fac, repos)
	assert.NoError(t, err)
	res, err := filter.Filter("distro:centos-9", "type:edge-ami", "arch:x86_64")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	opts := &manifestgen.Options{
		Output:            &osbuildManifest,
		Depsolver:         fakeDepsolve,
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
	err = mg.Generate(&bp, res[0].Distro, res[0].ImgType, res[0].Arch, imageOpts)
	assert.NoError(t, err)

	pipelineNames, err := manifesttest.PipelineNamesFrom(osbuildManifest.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, []string{"build", "ostree-deployment", "image"}, pipelineNames)

	// XXX: add testhelper to manifesttest for this
	assert.Contains(t, osbuildManifest.String(), `{"url":"resolved-url-for-centos/9/x86_64/edge"}`)
	// we expect at least a "glibc" package in the manifest,
	// sadly the test distro does not really generate much here so we
	// need to use this as a canary that resolving happend
	// XXX: add testhelper to manifesttest for this
	expectedSha256 := sha256For("glibc")
	assert.Contains(t, osbuildManifest.String(), expectedSha256)
}

func fakeDepsolve(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]dnfjson.DepsolveResult, error) {
	depsolvedSets := make(map[string]dnfjson.DepsolveResult)
	for name, pkgSets := range packageSets {
		repoId := fmt.Sprintf("repo_id_%s", name)
		var resolvedSet dnfjson.DepsolveResult
		for _, pkgSet := range pkgSets {
			for _, pkgName := range pkgSet.Include {
				resolvedSet.Packages = append(resolvedSet.Packages, rpmmd.PackageSpec{
					Name:     pkgName,
					Checksum: sha256For(pkgName),
					Path:     fmt.Sprintf("path/%s.rpm", pkgName),
					RepoID:   repoId,
				})
				resolvedSet.Repos = append(resolvedSet.Repos, rpmmd.RepoConfig{
					Id:       repoId,
					Metalink: "https://example.com/metalink",
				})
			}
		}
		depsolvedSets[name] = resolvedSet
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

	var osbuildManifest bytes.Buffer
	opts := &manifestgen.Options{
		Output:            &osbuildManifest,
		Depsolver:         fakeDepsolve,
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
	err = mg.Generate(&bp, res[0].Distro, res[0].ImgType, res[0].Arch, nil)
	assert.NoError(t, err)

	// container is included
	assert.Contains(t, osbuildManifest.String(), "resolved-cnt-"+fakeContainerSource)
}
