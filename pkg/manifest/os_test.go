package manifest_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/bootc"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

// CheckSystemdStageOptions checks the Command strings
func CheckSystemdStageOptions(t *testing.T, stages []*osbuild.Stage, commands []string) {
	t.Helper()

	// Find the systemd.unit.create stage
	s := findStage("org.osbuild.systemd.unit.create", stages)
	require.NotNil(t, s)

	require.NotNil(t, s.Options)
	options, ok := s.Options.(*osbuild.SystemdUnitCreateStageOptions)
	require.True(t, ok)

	// unit must be conditioned on the keyfile
	require.Len(t, options.Config.Unit.ConditionPathExists, 1)
	keyfile := options.Config.Unit.ConditionPathExists[0]
	// keyfile is also the EnvironmentFile
	require.Len(t, options.Config.Service.EnvironmentFile, 1)
	assert.Equal(t, keyfile, options.Config.Service.EnvironmentFile[0])

	execStart := options.Config.Service.ExecStart
	// the rm command gets prepended in every case
	commands = append(commands, fmt.Sprintf("/usr/bin/rm '%s'", keyfile))
	require.Equal(t, len(execStart), len(commands))

	// Make sure the commands are the same
	for idx, cmd := range commands {
		assert.Equal(t, cmd, options.Config.Service.ExecStart[idx])
	}
}

// CheckPkgSetInclude makes sure the packages named in pkgs are all included
func CheckPkgSetInclude(t *testing.T, pkgSetChain []rpmmd.PackageSet, pkgs []string) {

	// Gather up all the includes
	var includes []string
	for _, ps := range pkgSetChain {
		includes = append(includes, ps.Include...)
	}

	for _, p := range pkgs {
		assert.Contains(t, includes, p)
	}
}

func TestSubscriptionManagerCommands(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
	}
	pipeline, err := os.Serialize()
	assert.NoError(t, err)
	CheckSystemdStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager config --server.hostname 'subscription.rhsm.redhat.com'",
		`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'http://cdn.redhat.com/'`,
	})
}

func TestSubscriptionManagerInsightsCommands(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      true,
	}
	pipeline, err := os.Serialize()
	assert.NoError(t, err)
	CheckSystemdStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager config --server.hostname 'subscription.rhsm.redhat.com'",
		`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'http://cdn.redhat.com/'`,
		"/usr/bin/insights-client --register",
	})
}

func TestRhcInsightsCommands(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      false,
		Rhc:           true,
	}
	os.OSCustomizations.PermissiveRHC = common.ToPtr(true)
	pipeline, err := os.Serialize()
	assert.NoError(t, err)
	CheckSystemdStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager config --server.hostname 'subscription.rhsm.redhat.com'",
		`/usr/bin/rhc connect --organization="${ORG_ID}" --activation-key="${ACTIVATION_KEY}"`,
		"/usr/sbin/semanage permissive --add rhcd_t",
	})
}

func TestSubscriptionManagerPackages(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
	}

	pkgSetChain, err := os.GetPackageSetChain(manifest.DISTRO_NULL)
	assert.NoError(t, err)
	CheckPkgSetInclude(t, pkgSetChain, []string{"subscription-manager"})
}

func TestSubscriptionManagerInsightsPackages(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      true,
	}
	pkgSetChain, err := os.GetPackageSetChain(manifest.DISTRO_NULL)
	assert.NoError(t, err)
	CheckPkgSetInclude(t, pkgSetChain, []string{"subscription-manager", "insights-client"})
}

func TestRhcInsightsPackages(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      false,
		Rhc:           true,
	}
	pkgSetChain, err := os.GetPackageSetChain(manifest.DISTRO_NULL)
	assert.NoError(t, err)
	CheckPkgSetInclude(t, pkgSetChain, []string{"rhc", "subscription-manager", "insights-client"})
}

func TestBootupdStage(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSTreeRef = "some/ref"
	os.Bootupd = true
	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.bootupd.gen-metadata", pipeline.Stages)
	require.NotNil(t, st)
}

func TestInsightsClientConfigStage(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.InsightsClientConfig = &osbuild.InsightsClientConfigStageOptions{
		Config: osbuild.InsightsClientConfig{
			Proxy: "some-proxy",
			Path:  "some/path",
		},
	}
	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.insights-client.config", pipeline.Stages)
	require.NotNil(t, st)
}

func TestTomlLibUsedNoneByDefault(t *testing.T) {
	os := manifest.NewTestOS()
	buildPkgs, err := os.GetBuildPackages(manifest.DISTRO_FEDORA)
	assert.NoError(t, err)
	for _, pkg := range []string{"python3-pytoml", "python3-toml", "python3-tomli-w"} {
		assert.NotContains(t, buildPkgs, pkg)
	}
}

func TestTomlLibUsedForContainer(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.Containers = []container.SourceSpec{
		{Source: "some-source"},
	}
	os.OSCustomizations.ContainersStorage = common.ToPtr("foo")

	testTomlPkgsFor(t, os)
}

func TestTomlLibUsedForBootcConfig(t *testing.T) {
	os := manifest.NewTestOS()
	os.BootcConfig = &bootc.Config{Filename: "something"}

	testTomlPkgsFor(t, os)
}

func testTomlPkgsFor(t *testing.T, os *manifest.OS) {
	for _, tc := range []struct {
		distro          manifest.Distro
		expectedTomlPkg string
	}{
		{manifest.DISTRO_EL8, "python3-pytoml"},
		{manifest.DISTRO_EL9, "python3-toml"},
		{manifest.DISTRO_EL10, "python3-tomli-w"},
		{manifest.DISTRO_FEDORA, "python3-tomli-w"},
	} {
		buildPkgs, err := os.GetBuildPackages(tc.distro)
		assert.NoError(t, err)
		assert.Contains(t, buildPkgs, tc.expectedTomlPkg)
	}
}

func TestMachineIdUninitializedIncludesMachineIdStage(t *testing.T) {
	os := manifest.NewTestOS()

	os.OSCustomizations.MachineIdUninitialized = true

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.machine-id", pipeline.Stages)
	require.NotNil(t, st)
}

func TestMachineIdUninitializedDoesNotIncludeMachineIdStage(t *testing.T) {
	os := manifest.NewTestOS()

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.machine-id", pipeline.Stages)
	require.Nil(t, st)
}

func TestModularityIncludesConfigStage(t *testing.T) {
	os := manifest.NewTestOS()

	testModuleConfigPath := filepath.Join(t.TempDir(), "module-config")
	testFailsafeConfigPath := filepath.Join(t.TempDir(), "failsafe-config")

	inputs := manifest.Inputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name:     "pkg1",
						Checksum: rpmmd.Checksum{Type: "sha1", Value: "c02524e2bd19490f2a71677958f792262754c5f46"},
						RepoID:   "dummy-repo-id",
						Repo:     &rpmmd.RepoConfig{Id: "dummy-repo-id"},
					},
				},
			},
			Modules: []rpmmd.ModuleSpec{
				{
					ModuleConfigFile: rpmmd.ModuleConfigFile{
						Path: testModuleConfigPath,
					},
					FailsafeFile: rpmmd.ModuleFailsafeFile{
						Path: testFailsafeConfigPath,
					},
				},
			}},
	}
	pipeline, err := manifest.SerializeWith(os, inputs)
	require.NoError(t, err)
	st := findStage("org.osbuild.dnf.module-config", pipeline.Stages)
	require.NotNil(t, st)
}

func TestModularityDoesNotIncludeConfigStage(t *testing.T) {
	os := manifest.NewTestOS()

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.dnf.module-config", pipeline.Stages)
	require.Nil(t, st)
}

func checkStagesForFSTab(t *testing.T, stages []*osbuild.Stage) {
	fstab := findStage("org.osbuild.fstab", stages)
	require.NotNil(t, fstab)

	// The plain OS pipeline doesn't have any systemd.unit.create stages by
	// default. This test will break and will need to be adjusted if this ever
	// changes (if a systemd.unit.create stage is added to the pipeline by
	// default).
	systemdStages := findStages("org.osbuild.systemd.unit.create", stages)
	require.Nil(t, systemdStages)
}

func checkStagesForMountUnits(t *testing.T, stages []*osbuild.Stage, expectedUnits []string) {
	fstab := findStage("org.osbuild.fstab", stages)
	require.Nil(t, fstab)

	// The plain OS pipeline doesn't have any systemd.unit.create stages by
	// default. This test will break and will need to be adjusted if this ever
	// changes (if a systemd.unit.create stage is added to the pipeline by
	// default).
	systemdStages := findStages("org.osbuild.systemd.unit.create", stages)
	require.Len(t, systemdStages, len(expectedUnits))

	var mountUnitFilenames []string
	for _, stage := range systemdStages {
		options := stage.Options.(*osbuild.SystemdUnitCreateStageOptions)
		mountUnitFilenames = append(mountUnitFilenames, options.Filename)
	}
	require.ElementsMatch(t, mountUnitFilenames, expectedUnits)

	// creating mount units also adds a systemd stage to enable them
	enable := findStage("org.osbuild.systemd", stages)
	require.NotNil(t, enable)
	enableOptions := enable.Options.(*osbuild.SystemdStageOptions)
	require.ElementsMatch(t, enableOptions.EnabledServices, expectedUnits)

}

func checkStagesForNoMounts(t *testing.T, stages []*osbuild.Stage) {
	fstab := findStage("org.osbuild.fstab", stages)
	require.Nil(t, fstab)

	systemdStages := findStages("org.osbuild.systemd.unit.create", stages)
	require.Nil(t, systemdStages)
}

func TestOSPipelineFStabStage(t *testing.T) {
	os := manifest.NewTestOS()

	os.PartitionTable = testdisk.MakeFakePartitionTable("/")                     // PT specifics don't matter
	os.DiskCustomizations.MountConfiguration = osbuild.MOUNT_CONFIGURATION_FSTAB // set it explicitly just to be sure

	checkStagesForFSTab(t, common.Must(os.Serialize()).Stages)
}

func TestOSPipelineMountUnitStages(t *testing.T) {
	os := manifest.NewTestOS()

	expectedUnits := []string{"-.mount", "home.mount"}
	os.PartitionTable = testdisk.MakeFakePartitionTable("/", "/home")
	os.DiskCustomizations.MountConfiguration = osbuild.MOUNT_CONFIGURATION_UNITS

	checkStagesForMountUnits(t, common.Must(os.Serialize()).Stages, expectedUnits)
}

func TestOSPipelineMountNoneStages(t *testing.T) {
	os := manifest.NewTestOS()

	os.PartitionTable = testdisk.MakeFakePartitionTable("/", "/home")
	os.DiskCustomizations.MountConfiguration = osbuild.MOUNT_CONFIGURATION_NONE

	checkStagesForNoMounts(t, common.Must(os.Serialize()).Stages)
}

func TestLanguageIncludesLocaleStage(t *testing.T) {
	os := manifest.NewTestOS()

	os.OSCustomizations.Language = "en_US.UTF-8"

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.locale", pipeline.Stages)
	require.NotNil(t, st)
}

func TestLanguageDoesNotIncludeLocaleStage(t *testing.T) {
	os := manifest.NewTestOS()

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.locale", pipeline.Stages)
	require.Nil(t, st)
}

func TestTimezoneIncludesTimezoneStage(t *testing.T) {
	os := manifest.NewTestOS()

	os.OSCustomizations.Timezone = "Etc/UTC"

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.timezone", pipeline.Stages)
	require.NotNil(t, st)
}

func TestTimezoneDoesNotIncludeTimezoneStage(t *testing.T) {
	os := manifest.NewTestOS()

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.timezone", pipeline.Stages)
	require.Nil(t, st)
}

func TestHostnameIncludesHostnameStage(t *testing.T) {
	os := manifest.NewTestOS()

	os.OSCustomizations.Hostname = "funky.name"

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.hostname", pipeline.Stages)
	require.NotNil(t, st)
}

func TestHostnameDoesNotIncludeHostnameStage(t *testing.T) {
	os := manifest.NewTestOS()

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.hostname", pipeline.Stages)
	require.Nil(t, st)
}

func TestRpmlang(t *testing.T) {
	os := manifest.NewTestOS()
	os.OSCustomizations.InstallLangs = []string{"nl"}

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.rpm", pipeline.Stages)
	require.NotNil(t, st)
	assert.Equal(t, &osbuild.RPMStageOptions{
		InstallLangs: []string{"nl"},
	}, st.Options)
}

func TestRpmKeys(t *testing.T) {
	os := manifest.NewTestOS()

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	st := findStage("org.osbuild.rpm", pipeline.Stages)
	require.NotNil(t, st)
	assert.Equal(t, &osbuild.RPMStageOptions{}, st.Options)

	os = manifest.NewTestOS()
	os.OSCustomizations.RPMKeysBinary = "chickens"
	pipeline, err = os.Serialize()
	assert.NoError(t, err)

	st = findStage("org.osbuild.rpm", pipeline.Stages)
	require.NotNil(t, st)
	assert.Equal(t, &osbuild.RPMStageOptions{
		RPMKeys: &osbuild.RPMKeys{
			BinPath: "chickens",
		},
	}, st.Options)
}

func TestAddInlineOS(t *testing.T) {
	os := manifest.NewTestOS()

	require := require.New(t)

	// add some files to the OSCustomizations which are included near the end
	// of the pipeline
	os.OSCustomizations.Files = createTestFilesForPipeline()

	// enabling FIPS adds files after the Files defined above
	os.OSCustomizations.FIPS = true

	// adding subscription options adds a file before the rest
	os.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "000",
		ActivationKey: "111",
	}

	expectedPaths := []string{
		"tree:///etc/osbuild-subscription-register.env", // from the subscription options
		"tree:///etc/test/one",                          // directly from the OS customizations
		"tree:///etc/test/two",
		"tree:///etc/system-fips", // from FIPS = true
	}

	pipeline, err := os.Serialize()
	assert.NoError(t, err)

	destinationPaths := collectCopyDestinationPaths(pipeline.Stages)

	// The order is significant. Do not use ElementsMatch() or similar.
	require.Equal(expectedPaths, destinationPaths)

	expectedContents := []string{
		"ORG_ID=000\nACTIVATION_KEY=111",
		"test 1",
		"test 2",
		"# FIPS module installation complete\n",
	}

	fileContents := manifest.GetInline(os)
	// These are used to define the 'sources' part of the manifest, so the
	// order doesn't matter
	require.ElementsMatch(expectedContents, fileContents)
}

func createTestFilesForPipeline() []*fsnode.File {
	fileOne := common.Must(fsnode.NewFile("/etc/test/one", nil, nil, nil, []byte("test 1")))
	fileTwo := common.Must(fsnode.NewFile("/etc/test/two", nil, nil, nil, []byte("test 2")))
	return []*fsnode.File{
		fileOne,
		fileTwo,
	}
}

func collectCopyDestinationPaths(stages []*osbuild.Stage) []string {
	destinationPaths := make([]string, 0)
	copyStages := findStages("org.osbuild.copy", stages)
	for _, copyStage := range copyStages {
		copyStageOptions := copyStage.Options.(*osbuild.CopyStageOptions)
		for _, path := range copyStageOptions.Paths {
			destinationPaths = append(destinationPaths, path.To)
		}
	}
	return destinationPaths
}

func TestHMACStageInclusion(t *testing.T) {
	repos := []rpmmd.RepoConfig{}
	runner := &runner.CentOS{Version: 9}

	// We need the OS pipeline to run the serialization functions for the UKI,
	// which means we need a Platform with the correct bootloader setting and a
	// partition table with an ESP.
	platform := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		Bootloader: platform.BOOTLOADER_UKI,
	}
	pt := testdisk.TestPartitionTables()["plain"]

	t.Run("add-hmac-stage", func(t *testing.T) {
		repo := rpmmd.RepoConfig{Id: "dummy-repo-id"}
		inputs := manifest.Inputs{
			Depsolved: depsolvednf.DepsolveResult{
				Transactions: depsolvednf.TransactionList{
					{
						{
							Name:     "test-kernel",
							Epoch:    0,
							Version:  "13.3",
							Release:  "7.el9",
							Arch:     "x86_64",
							Checksum: rpmmd.Checksum{Type: "sha256", Value: "7777777777777777777777777777777777777777777777777777777777777777"},
							RepoID:   repo.Id,
							Repo:     &repo,
						},
						{
							Name:     "uki-direct",
							Epoch:    0,
							Version:  "24.11",
							Release:  "1.el9",
							Arch:     "noarch",
							Checksum: rpmmd.Checksum{Type: "sha256", Value: "c6ade8aef0282a228e1011f4f4b7efe41c035f6e635feb27082ac36cb1a1384b"},
							RepoID:   repo.Id,
							Repo:     &repo,
						},
						{
							Name:     "shim-x64",
							Epoch:    0,
							Version:  "15.8",
							Release:  "3",
							Arch:     "x86_64",
							Checksum: rpmmd.Checksum{Type: "sha256", Value: "aae94b3b8451ef28b02594d9abca5979e153c14f4db25283b011403fa92254fd"},
							RepoID:   repo.Id,
							Repo:     &repo,
						},
					},
				},
				Repos: []rpmmd.RepoConfig{repo},
			},
		}

		m := manifest.New()
		build := manifest.NewBuild(&m, runner, repos, nil)
		os := manifest.NewOS(build, platform, repos)
		os.PartitionTable = &pt
		os.OSCustomizations.KernelName = "test-kernel"
		pipeline, err := manifest.SerializeWith(os, inputs)
		assert.NoError(t, err)
		assert.NotNil(t, pipeline)

		hmacStage := findStage("org.osbuild.hmac", pipeline.Stages)
		assert.NotNil(t, hmacStage)
		hmacStageOptions := hmacStage.Options.(*osbuild.HMACStageOptions)
		assert.Equal(t, hmacStageOptions.Paths, []string{"/boot/efi/EFI/Linux/ffffffffffffffffffffffffffffffff-13.3-7.el9.x86_64.efi"})

		dirStages := findStages("org.osbuild.mkdir", pipeline.Stages)
		assert.NotNil(t, dirStages)
		directories := make([]string, 0)
		for _, dirStage := range dirStages {
			for _, stagePath := range dirStage.Options.(*osbuild.MkdirStageOptions).Paths {
				directories = append(directories, stagePath.Path)
			}
		}
		assert.Contains(t, directories, "/boot/efi/EFI/Linux/ffffffffffffffffffffffffffffffff-13.3-7.el9.x86_64.efi.extra.d")
	})

	t.Run("no-hmac-stage", func(t *testing.T) {
		repo := rpmmd.RepoConfig{Id: "dummy-repo-id"}
		inputs := manifest.Inputs{
			Depsolved: depsolvednf.DepsolveResult{
				Transactions: depsolvednf.TransactionList{
					{
						{
							Name:     "test-kernel",
							Epoch:    0,
							Version:  "13.3",
							Release:  "7.el9",
							Arch:     "x86_64",
							Checksum: rpmmd.Checksum{Type: "sha256", Value: "7777777777777777777777777777777777777777777777777777777777777777"},
							RepoID:   repo.Id,
							Repo:     &repo,
						},
						{
							Name:     "uki-direct",
							Epoch:    0,
							Version:  "25.11",
							Release:  "1.el9",
							Arch:     "noarch",
							Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
							RepoID:   repo.Id,
							Repo:     &repo,
						},
						{
							Name:     "shim-x64",
							Epoch:    0,
							Version:  "15.8",
							Release:  "3",
							Arch:     "x86_64",
							Checksum: rpmmd.Checksum{Type: "sha256", Value: "aae94b3b8451ef28b02594d9abca5979e153c14f4db25283b011403fa92254fd"},
							RepoID:   repo.Id,
							Repo:     &repo,
						},
					},
				},
			},
		}

		m := manifest.New()
		build := manifest.NewBuild(&m, runner, repos, nil)
		os := manifest.NewOS(build, platform, repos)
		os.PartitionTable = &pt
		pipeline, err := manifest.SerializeWith(os, inputs)
		assert.NoError(t, err)
		assert.NotNil(t, pipeline)

		hmacStage := findStage("org.osbuild.hmac", pipeline.Stages)
		assert.Nil(t, hmacStage)

		dirStages := findStages("org.osbuild.mkdir", pipeline.Stages)
		directories := make([]string, 0)
		for _, dirStage := range dirStages {
			for _, stagePath := range dirStage.Options.(*osbuild.MkdirStageOptions).Paths {
				directories = append(directories, stagePath.Path)
			}
		}
		assert.NotContains(t, directories, "/boot/efi/EFI/Linux/ffffffffffffffffffffffffffffffff-13.3-7.el9.x86_64.efi.extra.d")
	})
}

func TestShimVersionLock(t *testing.T) {
	repos := []rpmmd.RepoConfig{}
	runner := &runner.CentOS{Version: 9}

	platform := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		Bootloader: platform.BOOTLOADER_UKI,
	}
	pt := testdisk.TestPartitionTables()["plain"]

	m := manifest.New()
	build := manifest.NewBuild(&m, runner, repos, nil)
	os := manifest.NewOS(build, platform, repos)
	os.PartitionTable = &pt

	// mark the shim-x64 package for version locking
	os.OSCustomizations.VersionlockPackages = []string{"shim-x64"}

	repo := rpmmd.RepoConfig{Id: "dummy-repo-id"}
	inputs := manifest.Inputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name:     "test-kernel",
						Epoch:    0,
						Version:  "13.3",
						Release:  "7.el9",
						Arch:     "x86_64",
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "7777777777777777777777777777777777777777777777777777777777777777"},
						RepoID:   repo.Id,
						Repo:     &repo,
					},
					{
						Name:     "uki-direct",
						Epoch:    0,
						Version:  "25.11",
						Release:  "1.el9",
						Arch:     "noarch",
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
						RepoID:   repo.Id,
						Repo:     &repo,
					},
					{
						Name:     "shim-x64",
						Epoch:    0,
						Version:  "15.8",
						Release:  "3",
						Arch:     "x86_64",
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "aae94b3b8451ef28b02594d9abca5979e153c14f4db25283b011403fa92254fd"},
						RepoID:   repo.Id,
						Repo:     &repo,
					},
					{
						Name:     "dnf",
						Epoch:    0,
						Version:  "4.14.0",
						Release:  "29.el9",
						Arch:     "noarch",
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "72874726d1a16651933e382a4f4683046efd4b278830ad564932ce481ab8b9eb"},
						RepoID:   repo.Id,
						Repo:     &repo,
					},
					{
						Name:     "python3-dnf-plugin-versionlock",
						Epoch:    0,
						Version:  "4.3.0",
						Release:  "21.el9",
						Arch:     "noarch",
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "e14c57f7d0011ea378e4319bbc523000d0e7be4d35b6af7177aa6246c5aaa9ef"},
						RepoID:   repo.Id,
						Repo:     &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
	}

	pipeline, err := manifest.SerializeWith(os, inputs)
	assert.NoError(t, err)
	assert.NotNil(t, pipeline)
	versionlockStage := findStage("org.osbuild.dnf4.versionlock", pipeline.Stages)
	assert.NotNil(t, versionlockStage)
	stageOptions := versionlockStage.Options.(*osbuild.DNF4VersionlockOptions)

	assert.Equal(t, []string{"shim-x64-0:15.8-3"}, stageOptions.Add)
}

func TestOSPackageSetChainStructure(t *testing.T) {
	testCases := []struct {
		name                 string
		customizations       manifest.OSCustomizations
		expectedChainInclude [][]string
	}{
		{
			name: "no customization packages - one package set",
			customizations: manifest.OSCustomizations{
				BasePackages: []string{"bash", "coreutils"},
			},
			expectedChainInclude: [][]string{
				{"bash", "coreutils"},
			},
		},
		{
			name: "no customization packages with blueprint packages - two package sets",
			customizations: manifest.OSCustomizations{
				BasePackages:      []string{"bash", "coreutils"},
				BlueprintPackages: []string{"vim", "tmux"},
			},
			expectedChainInclude: [][]string{
				{"bash", "coreutils"},
				{"vim", "tmux"},
			},
		},
		{
			name: "with SELinux customization - two package sets",
			customizations: manifest.OSCustomizations{
				BasePackages: []string{"bash", "coreutils"},
				SELinux:      "targeted",
			},
			expectedChainInclude: [][]string{
				{"bash", "coreutils"},
				{"selinux-policy-targeted"},
			},
		},
		{
			name: "with blueprint packages - three package sets",
			customizations: manifest.OSCustomizations{
				BasePackages:      []string{"bash", "coreutils"},
				SELinux:           "targeted",
				BlueprintPackages: []string{"vim", "tmux"},
			},
			expectedChainInclude: [][]string{
				{"bash", "coreutils"},
				{"selinux-policy-targeted"},
				{"vim", "tmux"},
			},
		},
		{
			name: "subscription customization",
			customizations: manifest.OSCustomizations{
				BasePackages: []string{"bash"},
				Subscription: &subscription.ImageOptions{Organization: "123", ActivationKey: "key"},
			},
			expectedChainInclude: [][]string{
				{"bash"},
				{"subscription-manager"},
			},
		},
		{
			name: "chrony config customization",
			customizations: manifest.OSCustomizations{
				BasePackages: []string{"bash"},
				ChronyConfig: &osbuild.ChronyStageOptions{},
			},
			expectedChainInclude: [][]string{
				{"bash"},
				{"chrony"},
			},
		},
		{
			name: "firewall config customization",
			customizations: manifest.OSCustomizations{
				BasePackages: []string{"bash"},
				Firewall:     &osbuild.FirewallStageOptions{},
			},
			expectedChainInclude: [][]string{
				{"bash"},
				{"firewalld"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os := manifest.NewTestOS()
			os.OSCustomizations = tc.customizations

			pkgSetChain, err := os.GetPackageSetChain(manifest.DISTRO_NULL)
			require.NoError(t, err)
			require.Len(t, pkgSetChain, len(tc.expectedChainInclude))

			for idx, expectedPkgs := range tc.expectedChainInclude {
				assert.ElementsMatch(t, expectedPkgs, pkgSetChain[idx].Include,
					"package set %d Include mismatch", idx)
			}
		})
	}
}
