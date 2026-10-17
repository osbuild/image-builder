package manifest_test

import (
	"encoding/json"
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

func TestSysextTreeReuseOS(t *testing.T) {
	installed := testInputs().Depsolved.Transactions[0][0]
	first := installed
	first.Name = "first"
	first.Checksum.Value = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	second := installed
	second.Name = "second"
	second.Checksum.Value = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	tests := []struct {
		name         string
		transactions depsolvednf.TransactionList
		standalone   bool
		want         depsolvednf.TransactionList
	}{
		{
			name:         "only install extension transaction after multiple reference transactions",
			transactions: depsolvednf.TransactionList{{installed}, {first}, {second}},
			want:         depsolvednf.TransactionList{{second}},
		},
		{
			name:         "all packages already installed",
			transactions: depsolvednf.TransactionList{{installed}, {}},
		},
		{
			name: "no packages requested",
		},
		{
			name:         "standalone installs full transaction without copying",
			standalone:   true,
			transactions: depsolvednf.TransactionList{{installed, first}},
			want:         depsolvednf.TransactionList{{installed, first}},
		},
		{
			name:         "standalone installs all transactions without copying",
			standalone:   true,
			transactions: depsolvednf.TransactionList{{installed}, {first, second}},
			want:         depsolvednf.TransactionList{{installed}, {first, second}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mani := manifest.New()
			build := manifest.NewContentTestBuild(&mani, nil, nil, nil)
			ref := manifest.NewContentTest("os", build, nil, nil, nil)
			_, err := manifest.SerializeWith(ref, manifest.Inputs{Depsolved: depsolvednf.DepsolveResult{
				Transactions: depsolvednf.TransactionList{{installed}},
			}})
			require.NoError(t, err)
			var depsolveRef manifest.Pipeline = ref
			if tc.standalone {
				depsolveRef = nil
			}
			sp := manifest.NewSysextPipelines(build, nil, nil, ref, depsolveRef, "test")
			before, err := json.Marshal(tc.transactions)
			require.NoError(t, err)
			pipeline, err := manifest.SerializeWith(sp.Tree, manifest.Inputs{Depsolved: depsolvednf.DepsolveResult{
				Transactions: tc.transactions,
			}})
			require.NoError(t, err)
			after, err := json.Marshal(tc.transactions)
			require.NoError(t, err)
			assert.Equal(t, before, after, "serialization must not modify the depsolve result")

			stages := pipeline.Stages
			if !tc.standalone {
				require.NotEmpty(t, stages)
				assert.Equal(t, osbuild.NewCopyStageSimple(
					&osbuild.CopyStageOptions{Paths: []osbuild.CopyStagePath{{From: "input://tree/", To: "tree:///"}}},
					osbuild.NewPipelineTreeInputs("tree", "os"),
				), stages[0])
				stages = stages[1:]
			}
			require.Len(t, stages, len(tc.want))
			for i, pkgs := range tc.want {
				assert.Equal(t, "org.osbuild.rpm", stages[i].Type)
				assert.Equal(t, osbuild.NewRpmStageSourceFilesInputs(pkgs), stages[i].Inputs)
				options, err := json.Marshal(stages[i].Options)
				require.NoError(t, err)
				assert.NotContains(t, string(options), "nodeps")
			}
		})
	}
}

func TestSysextTreeReuseOSInheritedGPGKeys(t *testing.T) {
	mani := manifest.New()
	build := manifest.NewContentTestBuild(&mani, nil, nil, nil)
	ref := manifest.NewContentTest("os", build, nil, nil, nil)
	installed := testInputs().Depsolved.Transactions[0][0]
	installed.Files = []string{"/base-key"}
	_, err := manifest.SerializeWith(ref, manifest.Inputs{Depsolved: depsolvednf.DepsolveResult{
		Transactions: depsolvednf.TransactionList{{installed}},
	}})
	require.NoError(t, err)
	added := installed
	added.Name = "extension"
	added.Checksum.Value = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	added.Files = []string{"/extension-key"}
	sp := manifest.NewSysextPipelines(build, nil, nil, ref, ref, "test")
	sp.Tree.Customizations.BaseRPMOptions = &osbuild.RPMStageOptions{
		GPGKeysFromTree: []string{"/base-key", "/extension-key"},
		InstallLangs:    []string{"en_US"},
		DisableDracut:   true,
	}
	pipeline, err := manifest.SerializeWith(sp.Tree, manifest.Inputs{Depsolved: depsolvednf.DepsolveResult{
		Transactions: depsolvednf.TransactionList{{installed}, {added}},
	}})
	require.NoError(t, err)
	require.Len(t, pipeline.Stages, 2)
	opts := pipeline.Stages[1].Options.(*osbuild.RPMStageOptions)
	assert.Equal(t, []string{"/extension-key"}, opts.GPGKeysFromTree)
	assert.Equal(t, []string{"en_US"}, opts.InstallLangs)
	assert.True(t, opts.DisableDracut)
	assert.Equal(t, []string{"/base-key", "/extension-key"}, sp.Tree.Customizations.BaseRPMOptions.GPGKeysFromTree)
}

func TestSysextTreeReuseOSManifest(t *testing.T) {
	mani := manifest.New()
	build := manifest.NewContentTestBuild(&mani, nil, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}
	ref := manifest.NewOS(build, pf, nil)
	sp := manifest.NewSysextPipelines(build, pf, nil, ref, ref, "test")
	baseInputs := testInputs()
	baseInputs.Depsolved.Transactions[0][0].RemoteLocations = []string{"https://example.com/base.rpm"}
	added := baseInputs.Depsolved.Transactions[0][0]
	added.Name = "extension"
	added.RemoteLocations = []string{"https://example.com/extension.rpm"}
	added.Checksum.Value = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	depsolved := map[string]depsolvednf.DepsolveResult{
		ref.Name(): baseInputs.Depsolved,
		sp.Tree.Name(): {
			Transactions: depsolvednf.TransactionList{baseInputs.Depsolved.Transactions[0], {added}},
		},
	}
	// Repeated serialization must not consume or alter either pipeline's inputs.
	var previous manifest.OSBuildManifest
	for range 2 {
		data, err := mani.Serialize(depsolved, nil, nil, nil, nil)
		require.NoError(t, err)
		if previous != nil {
			assert.Equal(t, previous, data)
		}
		previous = data
		type stage struct {
			Type   string          `json:"type"`
			Inputs json.RawMessage `json:"inputs"`
		}
		var result struct {
			Pipelines []struct {
				Name   string  `json:"name"`
				Stages []stage `json:"stages"`
			} `json:"pipelines"`
		}
		require.NoError(t, json.Unmarshal(data, &result))
		var stages []stage
		for _, pipeline := range result.Pipelines {
			if pipeline.Name == sp.Tree.Name() {
				stages = pipeline.Stages
			}
		}
		require.Len(t, stages, 2)
		assert.Equal(t, "org.osbuild.copy", stages[0].Type)
		copyInputs, err := json.Marshal(osbuild.NewPipelineTreeInputs("tree", ref.Name()))
		require.NoError(t, err)
		assert.JSONEq(t, string(copyInputs), string(stages[0].Inputs))
		assert.Equal(t, "org.osbuild.rpm", stages[1].Type)
		rpmInputs, err := json.Marshal(osbuild.NewRpmStageSourceFilesInputs(rpmmd.PackageList{added}))
		require.NoError(t, err)
		assert.JSONEq(t, string(rpmInputs), string(stages[1].Inputs))
	}
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

func TestSysextPrepSerializeSELinux(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	overlayPipeline := manifest.NewSysextTree(build, pf, nil, "ext")

	final := manifest.NewSysextPrep(build, refPipeline, overlayPipeline, "ext")
	final.Customizations.Paths = []string{"/usr"}
	final.Customizations.SELinux = "targeted"
	pipeline, err := manifest.Serialize(final)
	require.NoError(t, err)

	require.Len(t, pipeline.Stages, 3, "expected tree-delta, os-release, and selinux stages")
	assert.Equal(t, "org.osbuild.tree-delta", pipeline.Stages[0].Type)
	assert.Equal(t, "org.osbuild.os-release", pipeline.Stages[1].Type)

	assert.Equal(t, "org.osbuild.selinux", pipeline.Stages[2].Type)
	selinuxOpts := pipeline.Stages[2].Options.(*osbuild.SELinuxStageOptions)
	assert.Equal(t, "input://tree/etc/selinux/targeted/contexts/files/file_contexts", selinuxOpts.FileContexts)
	assert.NotNil(t, pipeline.Stages[2].Inputs)
}

func TestSysextPrepSerializeNoSELinux(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}

	refPipeline := manifest.NewOS(build, pf, nil)
	overlayPipeline := manifest.NewSysextTree(build, pf, nil, "ext")

	final := manifest.NewSysextPrep(build, refPipeline, overlayPipeline, "ext")
	final.Customizations.Paths = []string{"/usr"}
	pipeline, err := manifest.Serialize(final)
	require.NoError(t, err)

	require.Len(t, pipeline.Stages, 2, "expected only tree-delta and os-release stages")
	assert.Equal(t, "org.osbuild.tree-delta", pipeline.Stages[0].Type)
	assert.Equal(t, "org.osbuild.os-release", pipeline.Stages[1].Type)
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
		assert.Nil(t, opts.ExtensionReleaseStrict)
	})

	t.Run("separate internal name and relaxed filename check", func(t *testing.T) {
		prep := newPrep("se-tmux")
		prep.Customizations.ExtensionReleaseName = "tmux"
		strict := false
		prep.Customizations.ExtensionRelease.ExtensionReleaseStrict = &strict
		prep.Customizations.ExtensionRelease.Vars.ID = "teamsbc"
		prep.Customizations.ExtensionRelease.Vars.VersionID = "46"
		pipeline, err := manifest.Serialize(prep)
		require.NoError(t, err)

		assert.Equal(t, "sysext-se-tmux-prep", pipeline.Name)
		opts := pipeline.Stages[1].Options.(*osbuild.OSReleaseStageOptions)
		assert.Equal(t, "usr/lib/extension-release.d/extension-release.tmux", opts.Path)
		require.NotNil(t, opts.ExtensionReleaseStrict)
		assert.False(t, *opts.ExtensionReleaseStrict)
		assert.Equal(t, "teamsbc", opts.Vars.ID)
		assert.Equal(t, "46", opts.Vars.VersionID)
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

func TestSysextTreeGetPackageSetChainPreservesReference(t *testing.T) {
	mani := manifest.New()
	r := &runner.Linux{}
	build := manifest.NewBuild(&mani, r, nil, nil)
	pf := &platform.Data{Arch: arch.ARCH_X86_64}
	baseRepo := rpmmd.RepoConfig{Id: "base"}
	payloadRepo := rpmmd.RepoConfig{Id: "payload"}
	extensionRepo := rpmmd.RepoConfig{Id: "extension"}

	refPipeline := manifest.NewOS(build, pf, []rpmmd.RepoConfig{baseRepo})
	refPipeline.OSCustomizations.BasePackages = []string{"coreutils", "bash"}
	refPipeline.OSCustomizations.ExcludeBasePackages = []string{"excluded"}
	refPipeline.OSCustomizations.InstallWeakDeps = true
	refPipeline.OSCustomizations.LockRootUser = true
	refPipeline.OSCustomizations.BlueprintPackages = []string{"blueprint"}
	refPipeline.OSCustomizations.BlueprintModules = []string{"test:stream"}
	refPipeline.OSCustomizations.PayloadRepos = []rpmmd.RepoConfig{payloadRepo}
	refChain, err := refPipeline.GetPackageSetChain(0)
	require.NoError(t, err)
	require.Len(t, refChain, 3)

	for _, name := range []string{"nginx", "tmux"} {
		sp := manifest.NewSysextPipelines(build, pf, []rpmmd.RepoConfig{extensionRepo}, refPipeline, refPipeline, name)
		sp.Tree.Customizations.PackageSet = rpmmd.PackageSet{
			Include:         []string{name},
			Exclude:         []string{"extension-excluded"},
			InstallWeakDeps: true,
		}

		chain, err := sp.Tree.GetPackageSetChain(0)
		require.NoError(t, err)
		require.Len(t, chain, len(refChain)+1)
		assert.Equal(t, refChain, chain[:len(refChain)])
		assert.Equal(t, rpmmd.PackageSet{
			Include:      []string{name},
			Exclude:      []string{"extension-excluded"},
			Repositories: []rpmmd.RepoConfig{baseRepo, payloadRepo, extensionRepo},
		}, chain[len(refChain)])
	}
	// Constructing extension chains must not change the reference's inputs.
	after, err := refPipeline.GetPackageSetChain(0)
	require.NoError(t, err)
	assert.Equal(t, refChain, after)
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
