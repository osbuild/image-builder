package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/depsolvednf"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
	"github.com/osbuild/image-builder/v73/pkg/platform"
	"github.com/osbuild/image-builder/v73/pkg/rpmmd"
	"github.com/osbuild/image-builder/v73/pkg/runner"
)

func newCoreOSInstaller() *CoreOSInstaller {
	m := &Manifest{}
	runner := &runner.Linux{}
	build := NewBuild(m, runner, nil, nil)

	x86plat := &platform.Data{Arch: arch.ARCH_X86_64}

	product := ""
	osversion := ""

	installer := NewCoreOSInstaller(build, x86plat, nil, "kernel", product, osversion)
	return installer
}

func TestCoreOSInstallerDracutModulesAndDrivers(t *testing.T) {
	transactions := depsolvednf.TransactionList{
		{
			{
				Name:     "kernel",
				Checksum: rpmmd.Checksum{Type: "sha256", Value: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
				RepoID:   "dummy-repo-id",
				Repo:     &rpmmd.RepoConfig{Id: "dummy-repo-id"},
			},
		},
	}

	coiPipeline := newCoreOSInstaller()
	coiPipeline.AdditionalDracutModules = []string{"test-module"}
	coiPipeline.AdditionalDrivers = []string{"test-driver"}
	err := coiPipeline.serializeStart(Inputs{Depsolved: depsolvednf.DepsolveResult{Transactions: transactions}})
	require.NoError(t, err)
	pipeline, err := coiPipeline.serialize()
	require.NoError(t, err)

	require := require.New(t)
	require.NotNil(pipeline)
	require.NotNil(pipeline.Stages)

	var stageOptions *osbuild.DracutStageOptions
	for _, stage := range pipeline.Stages {
		if stage.Type == "org.osbuild.dracut" {
			stageOptions = stage.Options.(*osbuild.DracutStageOptions)
		}
	}

	require.NotNil(stageOptions, "serialized anaconda pipeline does not contain an org.osbuild.anaconda stage")
	require.Contains(stageOptions.Modules, "test-module")
	require.Contains(stageOptions.AddDrivers, "test-driver")
}
