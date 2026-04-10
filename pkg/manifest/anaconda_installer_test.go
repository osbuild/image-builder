package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var dummyDepsolveResult = depsolvednf.DepsolveResult{
	Transactions: depsolvednf.TransactionList{
		{
			{
				Name: "kernel",
				Checksum: rpmmd.Checksum{
					Type:  "sha256",
					Value: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
				},
				RepoID: "dummy-repo-id",
				Repo:   &rpmmd.RepoConfig{Id: "dummy-repo-id"},
			},
		},
	},
	Repos: []rpmmd.RepoConfig{
		{
			Id: "dummy-repo-id",
		},
	},
}

func newAnacondaInstaller() *manifest.AnacondaInstaller {
	m := &manifest.Manifest{}
	runner := &runner.Linux{}
	build := manifest.NewBuild(m, runner, nil, nil)

	x86plat := &platform.Data{Arch: arch.ARCH_X86_64}

	product := ""
	osversion := ""

	preview := false

	instCust := manifest.InstallerCustomizations{
		Product:   product,
		OSVersion: osversion,
		Preview:   preview,
	}
	isoCust := manifest.ISOCustomizations{}
	installer := manifest.NewAnacondaInstaller(manifest.AnacondaInstallerTypePayload, build, x86plat, nil, "kernel", instCust, isoCust)
	return installer
}

func TestAnacondaInstallerModules(t *testing.T) {
	type testCase struct {
		enable   []string
		disable  []string
		expected []string
	}

	testCases := map[string]testCase{
		"empty-args": {
			expected: []string{},
		},
		"enable-users": {
			enable: []string{
				anaconda.ModuleUsers,
			},
			expected: []string{
				anaconda.ModuleUsers,
			},
		},
		"disable-storage": {
			disable: []string{
				anaconda.ModuleStorage,
			},
			expected: []string{},
		},
		"enable-users-disable-storage": {
			enable: []string{
				anaconda.ModuleUsers,
			},
			disable: []string{
				anaconda.ModuleStorage,
			},
			expected: []string{
				anaconda.ModuleUsers,
			},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		// Run each test case twice: once with activatable-modules and once with kickstart-modules.
		// Remove this when we drop support for RHEL 8.
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			for _, legacy := range []bool{true, false} {
				installerPipeline := newAnacondaInstaller()
				installerPipeline.InstallerCustomizations.UseLegacyAnacondaConfig = legacy
				installerPipeline.InstallerCustomizations.EnabledAnacondaModules = tc.enable
				installerPipeline.InstallerCustomizations.DisabledAnacondaModules = tc.disable
				pipeline, err := manifest.SerializeWith(installerPipeline, manifest.Inputs{Depsolved: dummyDepsolveResult})
				require.NoError(err)
				require.NotNil(pipeline)
				require.NotNil(pipeline.Stages)

				var anacondaStageOptions *osbuild.AnacondaStageOptions
				for _, stage := range pipeline.Stages {
					if stage.Type == "org.osbuild.anaconda" {
						anacondaStageOptions = stage.Options.(*osbuild.AnacondaStageOptions)
					}
				}

				require.NotNil(anacondaStageOptions, "serialized anaconda pipeline does not contain an org.osbuild.anaconda stage")
				if legacy {
					require.ElementsMatch(anacondaStageOptions.KickstartModules, tc.expected)
					require.Empty(anacondaStageOptions.ActivatableModules)
				} else {
					require.ElementsMatch(anacondaStageOptions.ActivatableModules, tc.expected)
					require.Empty(anacondaStageOptions.KickstartModules)
				}
			}
		})
	}
}

func TestISOLocale(t *testing.T) {
	locales := map[string]string{
		// input: expected
		"C.UTF-8":     "C.UTF-8",
		"en_US.UTF-8": "en_US.UTF-8",
		"":            "C.UTF-8",  // default
		"whatever":    "whatever", // arbitrary string
	}

	for input, expected := range locales {
		t.Run(input, func(t *testing.T) {
			require := require.New(t)
			installerPipeline := newAnacondaInstaller()
			installerPipeline.Locale = input
			pipeline, err := manifest.SerializeWith(installerPipeline, manifest.Inputs{Depsolved: dummyDepsolveResult})
			require.NoError(err)
			require.NotNil(pipeline)
			require.NotNil(pipeline.Stages)

			var stageOptions *osbuild.LocaleStageOptions
			for _, stage := range pipeline.Stages {
				if stage.Type == "org.osbuild.locale" {
					stageOptions = stage.Options.(*osbuild.LocaleStageOptions)
				}
			}

			require.NotNil(stageOptions, "serialized anaconda pipeline does not contain an org.osbuild.locale stage")
			require.Equal(expected, stageOptions.Language)
		})
	}
}

func TestAnacondaInstallerDracutModulesAndDrivers(t *testing.T) {
	require := require.New(t)

	installerPipeline := newAnacondaInstaller()
	installerPipeline.InstallerCustomizations.AdditionalDracutModules = []string{"test-module"}
	installerPipeline.InstallerCustomizations.AdditionalDrivers = []string{"test-driver"}
	pipeline, err := manifest.SerializeWith(installerPipeline, manifest.Inputs{Depsolved: dummyDepsolveResult})
	require.NoError(err)
	require.NotNil(pipeline)
	require.NotNil(pipeline.Stages)

	var stageOptions *osbuild.DracutStageOptions
	for _, stage := range pipeline.Stages {
		if stage.Type == "org.osbuild.dracut" {
			stageOptions = stage.Options.(*osbuild.DracutStageOptions)
		}
	}

	require.NotNil(stageOptions, "serialized anaconda pipeline does not contain an org.osbuild.anaconda stage")
	require.Contains(stageOptions.AddModules, "test-module")
	require.Contains(stageOptions.AddDrivers, "test-driver")
}

func TestAnacondaInstallerConfigLorax(t *testing.T) {
	require := require.New(t)

	installerPipeline := newAnacondaInstaller()
	installerPipeline.InstallerCustomizations.LoraxTemplatePackage = "lorax-templates-generic"
	installerPipeline.InstallerCustomizations.LoraxLogosPackage = "fedora-logos"
	installerPipeline.InstallerCustomizations.LoraxReleasePackage = "fedora-release"
	installerPipeline.InstallerCustomizations.LoraxTemplates = []manifest.InstallerLoraxTemplate{
		manifest.InstallerLoraxTemplate{Path: "99-generic/runtime-postinstall.tmpl"},
	}
	pipeline, err := manifest.SerializeWith(installerPipeline, manifest.Inputs{Depsolved: dummyDepsolveResult})
	require.NoError(err)
	require.NotNil(pipeline)
	require.NotNil(pipeline.Stages)

	var stageOptions []*osbuild.LoraxScriptStageOptions
	for _, stage := range pipeline.Stages {
		if stage.Type == "org.osbuild.lorax-script" {
			stageOptions = append(stageOptions, stage.Options.(*osbuild.LoraxScriptStageOptions))
		}
	}

	require.Greater(len(stageOptions), 0, "serialized anaconda pipeline does not contain an org.osbuild.lorax stage")
	assert.Equal(t, stageOptions[0].Path, "99-generic/runtime-postinstall.tmpl")
	assert.Equal(t, stageOptions[0].Branding.Logos, "fedora-logos")
	assert.Equal(t, stageOptions[0].Branding.Logos, "fedora-logos")
}

// TestAnacondaInstallerBootcSerialize verifies that AnacondaInstaller correctly
// handles the bootc installer case where containers are provided but packages
// are empty. Bootc installers get the kernel version via introspection, so
// serialization should succeed even when kernelName is set but no packages
// are provided.
func TestAnacondaInstallerBootcSerialize(t *testing.T) {
	installer := newAnacondaInstaller() // This sets kernelName to "kernel"

	// Serialize with containers but NO packages (bootc mode)
	containerSpec := container.Spec{Source: "quay.io/example/bootc-image:latest"}
	_, err := manifest.SerializeWith(installer, manifest.Inputs{
		Containers: []container.Spec{containerSpec},
		// Depsolved is intentionally empty - no packages for bootc installers
	})

	assert.NoError(t, err)
}
