package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

func newTestSysextTree(t *testing.T) (*manifest.SysextTree, manifest.Build) {
	t.Helper()
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}
	base := manifest.NewSysextTree(build, pf, nil, "test")
	return base, build
}

func testInputs() manifest.Inputs {
	repo := rpmmd.RepoConfig{Id: "test-repo"}
	return manifest.Inputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name:     "pkg1",
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
						RepoID:   repo.Id,
						Repo:     &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
	}
}

func TestNewSysextTreeName(t *testing.T) {
	base, _ := newTestSysextTree(t)
	assert.Equal(t, "sysext-test-tree", base.Name())
}

func TestSysextTreeSerialize(t *testing.T) {
	base, _ := newTestSysextTree(t)
	pipeline, err := manifest.SerializeWith(base, testInputs())
	require.NoError(t, err)

	assert.Equal(t, "sysext-test-tree", pipeline.Name)
	require.Len(t, pipeline.Stages, 1)
	assert.Equal(t, "org.osbuild.rpm", pipeline.Stages[0].Type)
}

func TestSysextTreeSerializeBaseRPMOptions(t *testing.T) {
	base, _ := newTestSysextTree(t)
	base.Customizations.BaseRPMOptions = &osbuild.RPMStageOptions{
		Exclude:      &osbuild.Exclude{Docs: true},
		InstallLangs: []string{"en_US"},
	}
	pipeline, err := manifest.SerializeWith(base, testInputs())
	require.NoError(t, err)

	require.Len(t, pipeline.Stages, 1)
	opts := pipeline.Stages[0].Options.(*osbuild.RPMStageOptions)
	require.NotNil(t, opts.Exclude)
	assert.True(t, opts.Exclude.Docs)
	assert.Equal(t, []string{"en_US"}, opts.InstallLangs)
}

func TestSysextTreeSerializeNotStarted(t *testing.T) {
	base, _ := newTestSysextTree(t)
	_, err := manifest.Serialize(base)
	assert.Error(t, err, "serialize without serializeStart should fail")
}

func TestSysextTreeGetPackageSetChainEmpty(t *testing.T) {
	base, _ := newTestSysextTree(t)
	chain, err := base.GetPackageSetChain(0)
	require.NoError(t, err)
	assert.Nil(t, chain)
}

func TestSysextTreeGetPackageSetChainWithPackages(t *testing.T) {
	base, _ := newTestSysextTree(t)
	base.Customizations.PackageSet = rpmmd.PackageSet{
		Include: []string{"vim", "git"},
	}
	chain, err := base.GetPackageSetChain(0)
	require.NoError(t, err)
	require.Len(t, chain, 1)
	assert.Equal(t, []string{"vim", "git"}, chain[0].Include)
	assert.False(t, chain[0].InstallWeakDeps)
}

func TestNewSysextPrepName(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	overlayPipeline := manifest.NewSysextTree(build, pf, nil, "ext")

	final := manifest.NewSysextPrep(build, refPipeline, overlayPipeline, "ext")
	assert.Equal(t, "sysext-ext-prep", final.Name())
}

func TestSysextPrepSerialize(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	overlayPipeline := manifest.NewSysextTree(build, pf, nil, "ext")

	final := manifest.NewSysextPrep(build, refPipeline, overlayPipeline, "ext")
	final.Customizations.Paths = []string{"/usr", "/opt"}
	final.Customizations.ExtensionRelease.Vars.ID = "fedora"
	final.Customizations.ExtensionRelease.Vars.VersionID = "44"
	pipeline, err := manifest.Serialize(final)
	require.NoError(t, err)

	assert.Equal(t, "sysext-ext-prep", pipeline.Name)
	require.Len(t, pipeline.Stages, 2, "expected tree-delta and os-release stages")

	assert.Equal(t, "org.osbuild.tree-delta", pipeline.Stages[0].Type)
	inputs := pipeline.Stages[0].Inputs.(*osbuild.TreeDeltaStageInputs)
	assert.NotNil(t, inputs.Reference)
	assert.NotNil(t, inputs.Overlay)
	tdOpts := pipeline.Stages[0].Options.(*osbuild.TreeDeltaStageOptions)
	assert.Equal(t, []string{"/usr", "/opt"}, tdOpts.Paths)
	assert.Nil(t, tdOpts.ExcludePaths)

	assert.Equal(t, "org.osbuild.os-release", pipeline.Stages[1].Type)
	erOpts := pipeline.Stages[1].Options.(*osbuild.OSReleaseStageOptions)
	assert.Equal(t, "usr/lib/extension-release.d/extension-release.ext", erOpts.Path)
	assert.Equal(t, "fedora", erOpts.Vars.ID)
	assert.Equal(t, "44", erOpts.Vars.VersionID)
}

func TestSysextPrepSerializeWithExcludePaths(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	overlayPipeline := manifest.NewSysextTree(build, pf, nil, "ext")

	final := manifest.NewSysextPrep(build, refPipeline, overlayPipeline, "ext")
	final.Customizations.Paths = []string{"/usr"}
	final.Customizations.ExcludePaths = []string{"/usr/share/doc", "/usr/share/man"}
	pipeline, err := manifest.Serialize(final)
	require.NoError(t, err)

	opts := pipeline.Stages[0].Options.(*osbuild.TreeDeltaStageOptions)
	assert.Equal(t, []string{"/usr"}, opts.Paths)
	assert.Equal(t, []string{"/usr/share/doc", "/usr/share/man"}, opts.ExcludePaths)
}

func TestSysextPrepExtensionRelease(t *testing.T) {
	newPrep := func(name string) *manifest.SysextPrep {
		mani := manifest.New()
		r := &runner.Linux{}
		build := manifest.NewBuild(&mani, r, nil, nil)
		pf := &platform.Data{Arch: arch.ARCH_X86_64}
		ref := manifest.NewOS(build, pf, nil)
		overlay := manifest.NewSysextTree(build, pf, nil, name)
		return manifest.NewSysextPrep(build, ref, overlay, name)
	}

	t.Run("both fields", func(t *testing.T) {
		prep := newPrep("nginx")
		prep.Customizations.ExtensionRelease.Vars.ID = "fedora"
		prep.Customizations.ExtensionRelease.Vars.VersionID = "44"
		pipeline, err := manifest.Serialize(prep)
		require.NoError(t, err)

		require.Len(t, pipeline.Stages, 2)
		assert.Equal(t, "org.osbuild.os-release", pipeline.Stages[1].Type)
		opts := pipeline.Stages[1].Options.(*osbuild.OSReleaseStageOptions)
		assert.Equal(t, "usr/lib/extension-release.d/extension-release.nginx", opts.Path)
		assert.Equal(t, "fedora", opts.Vars.ID)
		assert.Equal(t, "44", opts.Vars.VersionID)
	})

	t.Run("only ID", func(t *testing.T) {
		prep := newPrep("test")
		prep.Customizations.ExtensionRelease.Vars.ID = "rhel"
		pipeline, err := manifest.Serialize(prep)
		require.NoError(t, err)

		opts := pipeline.Stages[1].Options.(*osbuild.OSReleaseStageOptions)
		assert.Equal(t, "usr/lib/extension-release.d/extension-release.test", opts.Path)
		assert.Equal(t, "rhel", opts.Vars.ID)
		assert.Empty(t, opts.Vars.VersionID)
	})

	t.Run("empty", func(t *testing.T) {
		prep := newPrep("empty")
		pipeline, err := manifest.Serialize(prep)
		require.NoError(t, err)

		opts := pipeline.Stages[1].Options.(*osbuild.OSReleaseStageOptions)
		assert.Equal(t, "usr/lib/extension-release.d/extension-release.empty", opts.Path)
		assert.Empty(t, opts.Vars.ID)
		assert.Empty(t, opts.Vars.VersionID)
	})
}

func TestNewSysextPipelines(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	pipelines := manifest.NewSysextPipelines(build, pf, nil, refPipeline, refPipeline, "myext")

	assert.Equal(t, "sysext-myext-tree", pipelines.Tree.Name())
	assert.Equal(t, "sysext-myext-prep", pipelines.Prep.Name())
}

func TestSysextTreeGetPackageSetChainMergesReference(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	refPipeline.OSCustomizations.BasePackages = []string{"coreutils", "bash"}

	sp := manifest.NewSysextPipelines(build, pf, nil, refPipeline, refPipeline, "nginx")
	sp.Tree.Customizations.PackageSet = rpmmd.PackageSet{
		Include: []string{"nginx"},
	}

	chain, err := sp.Tree.GetPackageSetChain(0)
	require.NoError(t, err)
	require.Len(t, chain, 1)
	assert.Contains(t, chain[0].Include, "nginx")
	assert.Contains(t, chain[0].Include, "coreutils")
	assert.Contains(t, chain[0].Include, "bash")
	assert.False(t, chain[0].InstallWeakDeps)
}

func TestSysextTreeGetPackageSetChainNilDepsolveReference(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	refPipeline.OSCustomizations.BasePackages = []string{"coreutils", "bash"}

	sp := manifest.NewSysextPipelines(build, pf, nil, refPipeline, nil, "nginx")
	sp.Tree.Customizations.PackageSet = rpmmd.PackageSet{
		Include: []string{"nginx"},
	}

	chain, err := sp.Tree.GetPackageSetChain(0)
	require.NoError(t, err)
	require.Len(t, chain, 1)
	assert.Equal(t, []string{"nginx"}, chain[0].Include)
}
