package manifest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/testdisk"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/customizations/subscription"
	"github.com/osbuild/image-builder/pkg/customizations/users"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
)

var containers = []container.SourceSpec{
	{
		Name: "quay.io/centos-bootc/centos-bootc-dev:stream9",
	},
}

func hasPipeline(haystack []manifest.Pipeline, needle manifest.Pipeline) bool {
	for _, p := range haystack {
		if p == needle {
			return true
		}
	}
	return false
}

func TestNewRawBootcImage(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	buildIf := manifest.NewBuildFromContainer(&mani, runner, nil, nil)
	build := buildIf.(*manifest.BuildrootFromContainer)

	rawBootcPipeline := manifest.NewRawBootcImage(build, containers, nil)
	require.NotNil(t, rawBootcPipeline)

	assert.True(t, hasPipeline(build.Dependents(), rawBootcPipeline))

	// disk.img is hardcoded for filename
	assert.Equal(t, "disk.img", rawBootcPipeline.Filename())
}

func TestRawBootcImageSerialize(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuildFromContainer(&mani, runner, nil, nil)
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	rawBootcPipeline := manifest.NewRawBootcImage(build, containers, pf)
	rawBootcPipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
	rawBootcPipeline.OSCustomizations.Users = []users.User{{Name: "root", Key: common.ToPtr("some-ssh-key")}}
	rawBootcPipeline.OSCustomizations.KernelOptionsAppend = []string{"karg1", "karg2"}

	err := rawBootcPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	assert.NoError(t, err)
	imagePipeline, err := rawBootcPipeline.Serialize()
	assert.NoError(t, err)
	assert.Equal(t, "image", imagePipeline.Name)

	bootcInst := findStage("org.osbuild.bootc.install-to-filesystem", imagePipeline.Stages)
	require.NotNil(t, bootcInst)
	opts := bootcInst.Options.(*osbuild.BootcInstallToFilesystemOptions)
	// Note that the root account is customized via the "users" stage
	// (mostly for uniformity)
	assert.Equal(t, len(opts.RootSSHAuthorizedKeys), 0)
	assert.Equal(t, []string{"karg1", "karg2"}, opts.Kargs)
	assert.Equal(t, "quay.io/centos-bootc/centos-bootc-dev:stream9", opts.TargetImgref)
}

func TestRawBootcImageSerializeMountsValidated(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuildFromContainer(&mani, runner, nil, nil)
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	rawBootcPipeline := manifest.NewRawBootcImage(build, nil, pf)
	// note that we create a partition table without /boot here
	rawBootcPipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/missing-boot")
	err := rawBootcPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	assert.NoError(t, err)
	_, err = rawBootcPipeline.Serialize()
	assert.EqualError(t, err, `required mounts for bootupd stage [/boot/efi] missing`)
}

func findMountIdx(mounts []osbuild.Mount, mntType string) int {
	for i, mnt := range mounts {
		if mnt.Type == mntType {
			return i
		}
	}
	return -1
}

func makeFakeRawBootcPipeline() *manifest.RawBootcImage {
	mani := manifest.New()
	runner := &runner.Linux{}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	build := manifest.NewBuildFromContainer(&mani, runner, nil, nil)
	rawBootcPipeline := manifest.NewRawBootcImage(build, nil, pf)
	rawBootcPipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
	err := rawBootcPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	if err != nil {
		panic(err)
	}

	return rawBootcPipeline
}

func TestRawBootcImageSerializeCreateUsersOptions(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	for _, tc := range []struct {
		users              []users.User
		expectedUsersStage bool
	}{
		{nil, false},
		{[]users.User{{Name: "root"}}, true},
		{[]users.User{{Name: "foo"}}, true},
		{[]users.User{{Name: "root"}, {Name: "foo"}}, true},
	} {
		rawBootcPipeline.OSCustomizations.Users = tc.users

		pipeline, err := rawBootcPipeline.Serialize()
		assert.NoError(t, err)

		usersStage := findStage("org.osbuild.users", pipeline.Stages)
		if tc.expectedUsersStage {
			// ensure options got passed
			require.NotNil(t, usersStage)
			userOptions := usersStage.Options.(*osbuild.UsersStageOptions)
			for _, user := range tc.users {
				assert.NotNil(t, userOptions.Users[user.Name])
			}
		} else {
			require.Nil(t, usersStage)
		}
	}
}

func TestRawBootcImageSerializeMkdirOptions(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	for _, tc := range []struct {
		users              []users.User
		expectedMkdirPaths []osbuild.MkdirStagePath
	}{
		{nil, nil},
		{
			[]users.User{{Name: "root"}}, []osbuild.MkdirStagePath{
				{Path: "/var/roothome", Mode: common.ToPtr(os.FileMode(0700)), ExistOk: true},
			},
		},
		{
			[]users.User{{Name: "foo"}}, []osbuild.MkdirStagePath{
				{Path: "/var/home", Mode: common.ToPtr(os.FileMode(0755)), ExistOk: true},
			},
		},
		{
			[]users.User{{Name: "root"}, {Name: "foo"}}, []osbuild.MkdirStagePath{
				{Path: "/var/roothome", Mode: common.ToPtr(os.FileMode(0700)), ExistOk: true},
				{Path: "/var/home", Mode: common.ToPtr(os.FileMode(0755)), ExistOk: true},
			},
		},
	} {
		rawBootcPipeline.OSCustomizations.Users = tc.users

		pipeline, err := rawBootcPipeline.Serialize()
		assert.NoError(t, err)

		// Use findPostInstallStages to avoid matching pre-install mkdir
		// stages (e.g. mountpoint SELinux labeling) if SELinux is ever
		// set in this test.
		postInstallMkdirStages := findPostInstallStages("org.osbuild.mkdir", pipeline.Stages)
		if len(tc.expectedMkdirPaths) > 0 {
			// ensure options got passed
			require.Greater(t, len(postInstallMkdirStages), 0)
			mkdirOptions := postInstallMkdirStages[0].Options.(*osbuild.MkdirStageOptions)
			assert.Equal(t, tc.expectedMkdirPaths, mkdirOptions.Paths)
		} else {
			assert.Equal(t, 0, len(postInstallMkdirStages))
		}
	}
}

func TestRawBootcImageSerializeCreateGroupOptions(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	for _, tc := range []struct {
		groups              []users.Group
		expectedGroupsStage bool
	}{
		{nil, false},
		{[]users.Group{{Name: "root"}}, true},
		{[]users.Group{{Name: "foo"}}, true},
		{[]users.Group{{Name: "root"}, {Name: "foo"}}, true},
	} {
		rawBootcPipeline.OSCustomizations.Groups = tc.groups

		pipeline, err := rawBootcPipeline.Serialize()
		assert.NoError(t, err)

		groupsStage := findStage("org.osbuild.groups", pipeline.Stages)
		if tc.expectedGroupsStage {
			// ensure options got passed
			require.NotNil(t, groupsStage)
			groupOptions := groupsStage.Options.(*osbuild.GroupsStageOptions)
			for _, group := range tc.groups {
				assert.NotNil(t, groupOptions.Groups[group.Name])
			}
		} else {
			require.Nil(t, groupsStage)
		}
	}
}

func assertBootcDeploymentAndBindMount(t *testing.T, stage *osbuild.Stage) {
	// check for bind mount to deployment is there so
	// that the customization actually works
	deploymentMntIdx := findMountIdx(stage.Mounts, "org.osbuild.ostree.deployment")
	assert.True(t, deploymentMntIdx >= 0)
	bindMntIdx := findMountIdx(stage.Mounts, "org.osbuild.bind")
	assert.True(t, bindMntIdx >= 0)
	// order is important, bind must happen *after* deploy
	assert.True(t, bindMntIdx > deploymentMntIdx)
}

// findPostInstallStages returns stages that come after the bootc install stage
// and have the ostree deployment + bind mount (i.e. post-install customization stages).
func findPostInstallStages(stageType string, stages []*osbuild.Stage) []*osbuild.Stage {
	// Find the bootc install stage index
	bootcIdx := -1
	for i, s := range stages {
		if s.Type == "org.osbuild.bootc.install-to-filesystem" {
			bootcIdx = i
			break
		}
	}
	if bootcIdx < 0 {
		return nil
	}
	var found []*osbuild.Stage
	for _, s := range stages[bootcIdx+1:] {
		if s.Type == stageType {
			found = append(found, s)
		}
	}
	return found
}

func TestRawBootcImageSerializeCustomizationGenCorrectStages(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	for _, tc := range []struct {
		users   []users.User
		groups  []users.Group
		SELinux string

		expectedStages []string
	}{
		{nil, nil, "", nil},
		{[]users.User{{Name: "foo"}}, nil, "", []string{"org.osbuild.mkdir", "org.osbuild.users"}},
		{[]users.User{{Name: "foo"}}, nil, "targeted", []string{"org.osbuild.mkdir", "org.osbuild.users", "org.osbuild.selinux"}},
		{[]users.User{{Name: "foo"}}, []users.Group{{Name: "bar"}}, "targeted", []string{"org.osbuild.groups", "org.osbuild.mkdir", "org.osbuild.users", "org.osbuild.selinux"}},
	} {
		rawBootcPipeline.OSCustomizations.Users = tc.users
		rawBootcPipeline.OSCustomizations.Groups = tc.groups
		rawBootcPipeline.OSCustomizations.SELinux = tc.SELinux

		pipeline, err := rawBootcPipeline.Serialize()
		assert.NoError(t, err)

		for _, expectedStage := range tc.expectedStages {
			stages := findPostInstallStages(expectedStage, pipeline.Stages)
			assert.Greater(t, len(stages), 0, "expected post-install stage %q not found", expectedStage)
			for _, stage := range stages {
				assertBootcDeploymentAndBindMount(t, stage)
			}
		}
	}
}

func RawBootcImageSerializeCommonPipelines(t *testing.T) {
	expectedCommonStages := []string{
		"org.osbuild.truncate",
		"org.osbuild.sfdisk",
		"org.osbuild.mkfs.ext4",
		"org.osbuild.mkfs.ext4",
		"org.osbuild.mkfs.fat",
		"org.osbuild.bootc.install-to-filesystem",
		"org.osbuild.fstab",
	}
	rawBootcPipeline := makeFakeRawBootcPipeline()
	pipeline, err := rawBootcPipeline.Serialize()
	assert.NoError(t, err)

	pipelineStages := make([]string, len(pipeline.Stages))
	for i, st := range pipeline.Stages {
		pipelineStages[i] = st.Type
	}
	assert.Equal(t, expectedCommonStages, pipelineStages[0:len(expectedCommonStages)])
}

func RawBootcImageSerializeFstabPipelineHasBootcMounts(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	pipeline, err := rawBootcPipeline.Serialize()
	assert.NoError(t, err)

	stage := findStage("org.osbuild.fstab", pipeline.Stages)
	assert.NotNil(t, stage)
	assertBootcDeploymentAndBindMount(t, stage)
}

func TestRawBootcImageSerializeCreateFilesDirs(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	dir1, err := fsnode.NewDirectory("/path/to/dir", nil, nil, nil, false)
	require.NoError(t, err)
	file1, err := fsnode.NewFile("/path/to/file", nil, nil, nil, []byte("file-content"))
	require.NoError(t, err)
	for _, tc := range []struct {
		dirs  []*fsnode.Directory
		files []*fsnode.File
	}{
		{nil, nil},
		{[]*fsnode.Directory{dir1}, nil},
		{nil, []*fsnode.File{file1}},
		{[]*fsnode.Directory{dir1}, []*fsnode.File{file1}},
	} {
		tcName := fmt.Sprintf("files:%v,dirs:%v", len(tc.files), len(tc.dirs))
		t.Run(tcName, func(t *testing.T) {
			rawBootcPipeline.OSCustomizations.SELinux = "/path/to/selinux"
			rawBootcPipeline.OSCustomizations.Directories = tc.dirs
			rawBootcPipeline.OSCustomizations.Files = tc.files

			pipeline, err := rawBootcPipeline.Serialize()
			assert.NoError(t, err)

			// check dirs - look for post-install mkdir stages (with deployment mounts)
			postInstallMkdirStages := findPostInstallStages("org.osbuild.mkdir", pipeline.Stages)
			if len(tc.dirs) > 0 {
				require.Greater(t, len(postInstallMkdirStages), 0)
				mkdirOptions := postInstallMkdirStages[0].Options.(*osbuild.MkdirStageOptions)
				assert.Equal(t, "/path/to/dir", mkdirOptions.Paths[0].Path)
				assertBootcDeploymentAndBindMount(t, postInstallMkdirStages[0])
			} else {
				assert.Equal(t, 0, len(postInstallMkdirStages))
			}

			// check files
			copyStage := findStage("org.osbuild.copy", pipeline.Stages)
			if len(tc.files) > 0 {
				// ensure options got passed
				require.NotNil(t, copyStage)
				copyOptions := copyStage.Options.(*osbuild.CopyStageOptions)
				assert.Equal(t, "tree:///path/to/file", copyOptions.Paths[0].To)
				assertBootcDeploymentAndBindMount(t, copyStage)
			} else {
				assert.Nil(t, copyStage)
			}

			// Pre-install SELinux stages for mountpoint labeling
			preInstallSELinuxStages := findPreInstallStages("org.osbuild.selinux", pipeline.Stages)
			assert.Equal(t, 2, len(preInstallSELinuxStages), "expected 2 pre-install selinux stages (root + boot)")

			// Post-install SELinux stages for relabeling customizations
			if len(tc.dirs) > 0 || len(tc.files) > 0 {
				postInstallSELinuxStages := findPostInstallStages("org.osbuild.selinux", pipeline.Stages)
				assert.Greater(t, len(postInstallSELinuxStages), 0, "expected post-install selinux relabeling stages")
			}

			// XXX: we should really check that the inline
			// source for files got generated but that is
			// currently very hard to test :(
		})
	}
}

func TestRawBootcPipelineFSTabStage(t *testing.T) {
	pipeline := makeFakeRawBootcPipeline()

	pipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot/efi")        // PT requires /boot/efi
	pipeline.DiskCustomizations.MountConfiguration = osbuild.MOUNT_CONFIGURATION_FSTAB // set it explicitly just to be sure

	checkStagesForFSTab(t, common.Must(pipeline.Serialize()).Stages)
}

func TestRawBootcPipelineMountUnitStages(t *testing.T) {
	pipeline := makeFakeRawBootcPipeline()

	expectedUnits := []string{"-.mount", "home.mount", "boot-efi.mount"}
	pipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/home", "/boot/efi")
	pipeline.DiskCustomizations.MountConfiguration = osbuild.MOUNT_CONFIGURATION_UNITS

	checkStagesForMountUnits(t, common.Must(pipeline.Serialize()).Stages, expectedUnits)
}

func TestRawBootcPipelineNoMountsStages(t *testing.T) {
	pipeline := makeFakeRawBootcPipeline()

	pipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/home", "/boot/efi")
	pipeline.DiskCustomizations.MountConfiguration = osbuild.MOUNT_CONFIGURATION_NONE

	checkStagesForNoMounts(t, common.Must(pipeline.Serialize()).Stages)
}

func TestRawBootcImageSerializeGrub2DStage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		grub2Config  *osbuild.GRUB2Config
		expectStage  bool
		expectedPath string
	}{
		{
			name:        "nil-config",
			grub2Config: nil,
			expectStage: false,
		},
		{
			name:        "empty-config",
			grub2Config: &osbuild.GRUB2Config{},
			expectStage: false,
		},
		{
			name: "only-unrelated-fields",
			grub2Config: &osbuild.GRUB2Config{
				Default: "saved",
				Timeout: 5,
			},
			expectStage: false,
		},
		{
			name: "serial-only",
			grub2Config: &osbuild.GRUB2Config{
				Serial: "serial --unit=0 --speed=115200",
			},
			expectStage:  true,
			expectedPath: "tree:///boot/grub2/console.cfg",
		},
		{
			name: "all-console-fields",
			grub2Config: &osbuild.GRUB2Config{
				TerminalInput:  []string{"serial", "console"},
				TerminalOutput: []string{"serial", "console"},
				Serial:         "serial --unit=0 --speed=115200",
			},
			expectStage:  true,
			expectedPath: "tree:///boot/grub2/console.cfg",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rawBootcPipeline := makeFakeRawBootcPipeline()
			rawBootcPipeline.OSCustomizations.Grub2Config = tc.grub2Config

			pipeline, err := rawBootcPipeline.Serialize()
			assert.NoError(t, err)

			grub2dStage := findStage("org.osbuild.grub2.d", pipeline.Stages)
			if tc.expectStage {
				require.NotNil(t, grub2dStage)
				opts := grub2dStage.Options.(*osbuild.Grub2DStageOptions)
				assert.Equal(t, tc.expectedPath, opts.Path)
				assert.NotNil(t, opts.Config)
				// verify bootupd mounts are set up
				assert.NotEmpty(t, grub2dStage.Devices)
				assert.NotEmpty(t, grub2dStage.Mounts)
			} else {
				assert.Nil(t, grub2dStage)
			}
		})
	}
}

// findPreInstallStages returns stages of the given type that appear before
// the bootc install stage.
func findPreInstallStages(stageType string, stages []*osbuild.Stage) []*osbuild.Stage {
	var found []*osbuild.Stage
	for _, s := range stages {
		if s.Type == "org.osbuild.bootc.install-to-filesystem" {
			break
		}
		if s.Type == stageType {
			found = append(found, s)
		}
	}
	return found
}

func TestRawBootcImageSerializeMountpointSELinuxLabeling(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.OSCustomizations.SELinux = "targeted"

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)

	// Pre-install mkdir stages: one for /boot (root-only), one for /boot/efi (root+boot)
	preInstallMkdirStages := findPreInstallStages("org.osbuild.mkdir", pipeline.Stages)
	require.Equal(t, 2, len(preInstallMkdirStages))

	// First mkdir stage creates /boot with only root mounted
	bootMkdirOpts := preInstallMkdirStages[0].Options.(*osbuild.MkdirStageOptions)
	require.Equal(t, 1, len(bootMkdirOpts.Paths))
	assert.Equal(t, "mount://-/boot", bootMkdirOpts.Paths[0].Path)
	assert.NotEmpty(t, preInstallMkdirStages[0].Devices)
	bootMkdirMountTargets := make([]string, 0)
	for _, mnt := range preInstallMkdirStages[0].Mounts {
		bootMkdirMountTargets = append(bootMkdirMountTargets, mnt.Target)
	}
	assert.Contains(t, bootMkdirMountTargets, "/")
	assert.NotContains(t, bootMkdirMountTargets, "/boot")

	// Second mkdir stage creates /boot/efi with root+boot mounted
	efiMkdirOpts := preInstallMkdirStages[1].Options.(*osbuild.MkdirStageOptions)
	require.Equal(t, 1, len(efiMkdirOpts.Paths))
	assert.Equal(t, "mount://boot/efi", efiMkdirOpts.Paths[0].Path)
	assert.NotEmpty(t, preInstallMkdirStages[1].Devices)
	efiMkdirMountTargets := make([]string, 0)
	for _, mnt := range preInstallMkdirStages[1].Mounts {
		efiMkdirMountTargets = append(efiMkdirMountTargets, mnt.Target)
	}
	assert.Contains(t, efiMkdirMountTargets, "/")
	assert.Contains(t, efiMkdirMountTargets, "/boot")

	// Pre-install selinux stages: one for root, one for /boot
	preInstallSELinuxStages := findPreInstallStages("org.osbuild.selinux", pipeline.Stages)
	require.Equal(t, 2, len(preInstallSELinuxStages))

	// First SELinux stage: labels root mount (without boot mounted)
	rootSELinuxOpts := preInstallSELinuxStages[0].Options.(*osbuild.SELinuxStageOptions)
	assert.Equal(t, "mount://-/", rootSELinuxOpts.Target)
	assert.Contains(t, rootSELinuxOpts.FileContexts, "input://tree/etc/selinux/targeted/contexts/files/file_contexts")
	// Should have inputs (tree input from source pipeline)
	assert.NotNil(t, preInstallSELinuxStages[0].Inputs)
	// Should only mount root
	rootMountTargets := make([]string, 0)
	for _, mnt := range preInstallSELinuxStages[0].Mounts {
		rootMountTargets = append(rootMountTargets, mnt.Target)
	}
	assert.Contains(t, rootMountTargets, "/")
	assert.NotContains(t, rootMountTargets, "/boot")

	// Second SELinux stage: labels /boot (with boot mounted)
	bootSELinuxOpts := preInstallSELinuxStages[1].Options.(*osbuild.SELinuxStageOptions)
	assert.Equal(t, "mount://-/boot/", bootSELinuxOpts.Target)
	assert.Contains(t, bootSELinuxOpts.FileContexts, "input://tree/etc/selinux/targeted/contexts/files/file_contexts")
	assert.NotNil(t, preInstallSELinuxStages[1].Inputs)
	// Should mount both root and boot
	bootMountTargets := make([]string, 0)
	for _, mnt := range preInstallSELinuxStages[1].Mounts {
		bootMountTargets = append(bootMountTargets, mnt.Target)
	}
	assert.Contains(t, bootMountTargets, "/")
	assert.Contains(t, bootMountTargets, "/boot")
}

func TestRawBootcImageSerializeMountpointSELinuxLabelingNoSELinux(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	// No SELinux set - no pre-install labeling stages should be generated
	rawBootcPipeline.OSCustomizations.SELinux = ""

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)

	preInstallSELinuxStages := findPreInstallStages("org.osbuild.selinux", pipeline.Stages)
	assert.Equal(t, 0, len(preInstallSELinuxStages))
	preInstallMkdirStages := findPreInstallStages("org.osbuild.mkdir", pipeline.Stages)
	assert.Equal(t, 0, len(preInstallMkdirStages))
}

func TestRawBootcImageSerializeMountpointSELinuxLabelingNoBoot(t *testing.T) {
	// Test the edge case where there is no separate /boot partition
	// (only root + EFI). Only the root SELinux stage should be generated,
	// no /boot labeling stage.
	mani := manifest.New()
	r := &runner.Linux{}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	build := manifest.NewBuildFromContainer(&mani, r, nil, nil)
	rawBootcPipeline := manifest.NewRawBootcImage(build, nil, pf)
	rawBootcPipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot/efi")
	err := rawBootcPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	require.NoError(t, err)

	rawBootcPipeline.OSCustomizations.SELinux = "targeted"

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)

	// Pre-install mkdir stage should only create /boot (no /boot/efi since
	// there's no separate boot partition to mount it on)
	preInstallMkdirStages := findPreInstallStages("org.osbuild.mkdir", pipeline.Stages)
	require.Equal(t, 1, len(preInstallMkdirStages))
	mkdirOpts := preInstallMkdirStages[0].Options.(*osbuild.MkdirStageOptions)
	var mkdirPaths []string
	for _, p := range mkdirOpts.Paths {
		mkdirPaths = append(mkdirPaths, p.Path)
	}
	assert.Contains(t, mkdirPaths, "mount://-/boot")
	assert.NotContains(t, mkdirPaths, "mount://boot/efi")

	// Only one pre-install selinux stage (root only, no /boot stage)
	preInstallSELinuxStages := findPreInstallStages("org.osbuild.selinux", pipeline.Stages)
	require.Equal(t, 1, len(preInstallSELinuxStages))

	rootSELinuxOpts := preInstallSELinuxStages[0].Options.(*osbuild.SELinuxStageOptions)
	assert.Equal(t, "mount://-/", rootSELinuxOpts.Target)
}

func TestRawBootcPXE(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.KernelVersion = "5.14.0-611.4.1.el9_7.x86_64"
	rawBootcPipeline.LiveBoot = true

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)

	// Check for mkdir stages
	mkdirPaths := collectMkdirPaths(pipeline.Stages)
	require.NotEmpty(t, mkdirPaths)
	assert.Contains(t, mkdirPaths, "/usr")
	assert.Contains(t, mkdirPaths, "/proc")
}

func TestRawBootcImageSerializeSubscriptionManagerCommands(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
	}

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)
	CheckSystemdStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager config --server.hostname 'subscription.rhsm.redhat.com'",
		`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'http://cdn.redhat.com/'`,
	})

	// registration unit in /etc, not /usr (ostree commit content)
	assert.Equal(t, osbuild.EtcUnitPath, registrationUnitPath(t, pipeline.Stages))
}

func TestRawBootcImageSerializeSubscriptionManagerInsightsCommands(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      true,
	}

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)
	CheckSystemdStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager config --server.hostname 'subscription.rhsm.redhat.com'",
		`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'http://cdn.redhat.com/'`,
		"/usr/bin/insights-client --register",
	})

	// InsightsOnBoot also materializes the insights-client drop-in
	mkdirPaths := collectMkdirPaths(pipeline.Stages)
	assert.Contains(t, mkdirPaths, "/etc/systemd/system/insights-client-boot.service.d")
	destinationPaths := collectCopyDestinationPaths(pipeline.Stages)
	assert.Contains(t, destinationPaths, "tree:///etc/systemd/system/insights-client-boot.service.d/override.conf")
}

func TestRawBootcImageSerializeRhcInsightsCommands(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      false,
		Rhc:           true,
	}
	rawBootcPipeline.OSCustomizations.PermissiveRHC = common.ToPtr(true)

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)
	CheckSystemdStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager config --server.hostname 'subscription.rhsm.redhat.com'",
		`/usr/bin/rhc connect --organization="${ORG_ID}" --activation-key="${ACTIVATION_KEY}"`,
		"/usr/sbin/semanage permissive --add rhcd_t",
	})
}

func TestRawBootcImageSerializeSubscriptionEnablesService(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
	}

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)

	stage := findStage("org.osbuild.systemd", pipeline.Stages)
	require.NotNil(t, stage)
	opts := stage.Options.(*osbuild.SystemdStageOptions)
	assert.Equal(t, []string{"osbuild-subscription-register.service"}, opts.EnabledServices)
}

// Mirrors TestAddInlineOS: the env file must be both a copy destination and an
// inline source, and is written before the blueprint's own files.
func TestRawBootcImageSerializeSubscriptionEnvFile(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	require := require.New(t)

	rawBootcPipeline.OSCustomizations.Files = createTestFilesForPipeline()
	rawBootcPipeline.OSCustomizations.Subscription = &subscription.ImageOptions{
		Organization:  "000",
		ActivationKey: "111",
	}

	expectedPaths := []string{
		"tree:///etc/osbuild-subscription-register.env", // from the subscription options
		"tree:///etc/test/one",                          // directly from the OS customizations
		"tree:///etc/test/two",
	}

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(err)

	destinationPaths := collectCopyDestinationPaths(pipeline.Stages)

	// The order is significant. Do not use ElementsMatch() or similar.
	require.Equal(expectedPaths, destinationPaths)

	expectedContents := []string{
		"ORG_ID=000\nACTIVATION_KEY=111",
		"test 1",
		"test 2",
	}

	fileContents := manifest.GetInline(rawBootcPipeline)
	// These are used to define the 'sources' part of the manifest, so the
	// order doesn't matter
	require.ElementsMatch(expectedContents, fileContents)
}

func TestRawBootcImageSerializeURIFilesError(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()

	localFile := filepath.Join(t.TempDir(), "local-file")
	require.NoError(t, os.WriteFile(localFile, []byte("some content"), 0644))
	uriFile := common.Must(fsnode.NewFileForURI("/etc/test/from-uri", nil, nil, nil, localFile))
	rawBootcPipeline.OSCustomizations.Files = []*fsnode.File{uriFile}

	_, err := rawBootcPipeline.Serialize()
	assert.EqualError(t, err, fmt.Sprintf(
		"cannot create file %q from %q: files from an URI are not supported for bootc disk images",
		"/etc/test/from-uri", localFile))
}

// registrationUnitPath returns the UnitPath of the registration unit, found by
// filename because mount units share the systemd.unit.create stage type.
func registrationUnitPath(t *testing.T, stages []*osbuild.Stage) osbuild.SystemdUnitPath {
	t.Helper()
	for _, s := range findStages("org.osbuild.systemd.unit.create", stages) {
		opts := s.Options.(*osbuild.SystemdUnitCreateStageOptions)
		if opts.Filename == "osbuild-subscription-register.service" {
			return opts.UnitPath
		}
	}
	require.Fail(t, "no osbuild-subscription-register.service unit.create stage found")
	return ""
}

func collectMkdirPaths(stages []*osbuild.Stage) []string {
	mkdirPaths := make([]string, 0)
	for _, mkdirStage := range findStages("org.osbuild.mkdir", stages) {
		mkdirStageOptions := mkdirStage.Options.(*osbuild.MkdirStageOptions)
		for _, path := range mkdirStageOptions.Paths {
			mkdirPaths = append(mkdirPaths, path.Path)
		}
	}
	return mkdirPaths
}
