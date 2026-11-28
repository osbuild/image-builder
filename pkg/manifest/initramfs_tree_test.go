package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/customizations/firmware"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/customizations/kernelmodules"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestInitramfsServices(t *testing.T) {
	for name, options := range map[string]osbuild.SystemdStageOptions{
		"without services": {},
		"enabled":          {EnabledServices: []string{"systemd-sysext-sysroot.service"}},
		"disabled":         {DisabledServices: []string{"systemd-homed.service"}},
		"masked":           {MaskedServices: []string{"initrd-cleanup.service"}},
		"combined": {
			EnabledServices:  []string{"systemd-sysext-sysroot.service"},
			DisabledServices: []string{"systemd-homed.service"},
			MaskedServices:   []string{"initrd-cleanup.service"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := New()
			build := NewBuild(&m, &runner.Linux{}, nil, nil)
			tree := NewInitramfsTree(build, &platform.Data{}, nil)
			tree.Customizations.EnabledServices = options.EnabledServices
			tree.Customizations.DisabledServices = options.DisabledServices
			tree.Customizations.MaskedServices = options.MaskedServices
			configured := len(options.EnabledServices)+len(options.DisabledServices)+len(options.MaskedServices) > 0

			pipeline, err := SerializeWith(tree, Inputs{})
			require.NoError(t, err)
			if configured {
				last := pipeline.Stages[len(pipeline.Stages)-1]
				assert.Equal(t, "org.osbuild.systemd", last.Type)
				assert.Equal(t, &options, last.Options)
			} else {
				for _, stage := range pipeline.Stages {
					assert.NotEqual(t, "org.osbuild.systemd", stage.Type)
				}
			}

			chains, err := m.GetPackageSetChains()
			require.NoError(t, err)
			require.NotEmpty(t, chains[build.Name()])
			if configured {
				assert.Contains(t, chains[build.Name()][0].Include, "systemd")
			}
		})
	}
}

func TestInitramfsKernelModules(t *testing.T) {
	source := NewTestOS()
	source.OSCustomizations.KernelName = "kernel-core"
	require.NoError(t, source.serializeStart(Inputs{Depsolved: depsolvednf.DepsolveResult{
		Transactions: depsolvednf.TransactionList{{{
			Name: "kernel-core", Version: "7.3.4", Release: "300.fc45", Arch: "x86_64",
		}}},
	}}))
	defer source.serializeEnd()
	tree := NewInitramfsTree(source.BuildPipeline(), &platform.Data{}, nil)
	tree.Customizations.KernelModules = &kernelmodules.KernelModules{From: source.Name()}
	tree.Customizations.Firmware = &firmware.Firmware{From: source.Name()}
	file, err := fsnode.NewFile("/etc/test", nil, nil, nil, []byte("test"))
	require.NoError(t, err)
	tree.Customizations.Files = []*fsnode.File{file}
	tree.Customizations.EnabledServices = []string{"example.service"}
	require.NoError(t, tree.serializeStart(Inputs{}))
	defer tree.serializeEnd()

	pipeline, err := tree.serialize()
	require.NoError(t, err)
	require.Greater(t, len(pipeline.Stages), 2)
	stage := pipeline.Stages[0]
	assert.Equal(t, "org.osbuild.kernel-modules.copy", stage.Type)
	assert.Equal(t, &osbuild.KernelModulesCopyStageOptions{
		KernelVersion: "7.3.4-300.fc45.x86_64",
	}, stage.Options)
	assert.Equal(t, osbuild.NewPipelineTreeInputs("tree", "os"), stage.Inputs)
	assert.Equal(t, "org.osbuild.mkdir", pipeline.Stages[1].Type)
	assert.Equal(t, "org.osbuild.copy", pipeline.Stages[2].Type)
	assert.Equal(t, osbuild.NewPipelineTreeInputs("firmware", "os"), pipeline.Stages[2].Inputs)
	assert.Equal(t, "org.osbuild.systemd", pipeline.Stages[len(pipeline.Stages)-1].Type)
	// The module source is not installed as part of the initramfs package set.
	chain, err := tree.getPackageSetChain(DISTRO_FEDORA)
	require.NoError(t, err)
	assert.Nil(t, chain)
}

func TestInitramfsKernelModulesErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		from       string
		lateSource bool
		message    string
	}{
		{"empty", "", false, "kernel_modules.from must name a source pipeline"},
		{"missing", "missing", false, "must precede pipeline"},
		{"self", "initramfs-tree", false, "must precede pipeline"},
		{"not a kernel tree", "build", false, "does not provide a directory tree"},
		{"unresolved kernel", "os", false, "has no resolved kernel version"},
		{"later source", "os", true, "must precede pipeline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			build := NewBuild(&m, &runner.Linux{}, nil, nil)
			if !tc.lateSource {
				NewOS(build, &platform.Data{}, nil)
			}
			tree := NewInitramfsTree(build, &platform.Data{}, nil)
			if tc.lateSource {
				NewOS(build, &platform.Data{}, nil)
			}
			tree.Customizations.KernelModules = &kernelmodules.KernelModules{From: tc.from}
			require.NoError(t, tree.serializeStart(Inputs{}))
			defer tree.serializeEnd()
			_, err := tree.serialize()
			require.ErrorContains(t, err, tc.message)
		})
	}
}

func TestInitramfsFirmware(t *testing.T) {
	// Firmware sources do not need a resolved kernel version.
	source := NewTestOS()
	tree := NewInitramfsTree(source.BuildPipeline(), &platform.Data{}, nil)
	tree.Customizations.Firmware = &firmware.Firmware{From: source.Name()}
	file, err := fsnode.NewFile("/usr/lib/firmware/override.bin", nil, nil, nil, []byte("override"))
	require.NoError(t, err)
	tree.Customizations.Files = []*fsnode.File{file}
	require.NoError(t, tree.serializeStart(Inputs{}))
	defer tree.serializeEnd()

	pipeline, err := tree.serialize()
	require.NoError(t, err)
	require.Greater(t, len(pipeline.Stages), 2)
	assert.Equal(t, "org.osbuild.mkdir", pipeline.Stages[0].Type)
	assert.Equal(t, &osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{{Path: "tree:///usr/lib/firmware", Parents: true, ExistOk: true}},
	}, pipeline.Stages[0].Options)
	assert.Equal(t, "org.osbuild.copy", pipeline.Stages[1].Type)
	assert.Equal(t, &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{{From: "input://firmware/usr/lib/firmware/", To: "tree:///usr/lib/firmware/"}},
	}, pipeline.Stages[1].Options)
	assert.Equal(t, osbuild.NewPipelineTreeInputs("firmware", "os"), pipeline.Stages[1].Inputs)
	for _, stage := range pipeline.Stages {
		assert.NotEqual(t, "org.osbuild.kernel-modules.copy", stage.Type)
	}
	packages, err := tree.getBuildPackages(DISTRO_FEDORA)
	require.NoError(t, err)
	assert.Contains(t, packages, "coreutils")
	chain, err := tree.getPackageSetChain(DISTRO_FEDORA)
	require.NoError(t, err)
	assert.Nil(t, chain)
}

func TestInitramfsFirmwareErrors(t *testing.T) {
	for _, tc := range []struct{ from, message string }{
		{"", "firmware.from must name a source pipeline"},
		{"missing", "firmware source \"missing\" must precede pipeline"},
		{"initramfs-tree", "firmware source \"initramfs-tree\" must precede pipeline"},
		{"build", "firmware source \"build\" does not provide a directory tree"},
	} {
		t.Run(tc.from, func(t *testing.T) {
			source := NewTestOS()
			tree := NewInitramfsTree(source.BuildPipeline(), &platform.Data{}, nil)
			tree.Customizations.Firmware = &firmware.Firmware{From: tc.from}
			require.NoError(t, tree.serializeStart(Inputs{}))
			defer tree.serializeEnd()
			_, err := tree.serialize()
			require.ErrorContains(t, err, tc.message)
		})
	}
}
