package manifest_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
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

		mkdirStage := findStage("org.osbuild.mkdir", pipeline.Stages)
		if len(tc.expectedMkdirPaths) > 0 {
			// ensure options got passed
			require.NotNil(t, mkdirStage)
			mkdirOptions := mkdirStage.Options.(*osbuild.MkdirStageOptions)
			assert.Equal(t, tc.expectedMkdirPaths, mkdirOptions.Paths)
		} else {
			require.Nil(t, mkdirStage)
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
		{[]users.User{{Name: "foo"}}, []users.Group{{Name: "bar"}}, "targeted", []string{"org.osbuild.mkdir", "org.osbuild.users", "org.osbuild.users", "org.osbuild.selinux"}},
	} {
		rawBootcPipeline.OSCustomizations.Users = tc.users
		rawBootcPipeline.OSCustomizations.SELinux = tc.SELinux

		pipeline, err := rawBootcPipeline.Serialize()
		assert.NoError(t, err)

		for _, expectedStage := range tc.expectedStages {
			stage := findStage(expectedStage, pipeline.Stages)
			assert.NotNil(t, stage)
			assertBootcDeploymentAndBindMount(t, stage)
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

			// check dirs
			mkdirStage := findStage("org.osbuild.mkdir", pipeline.Stages)
			if len(tc.dirs) > 0 {
				// ensure options got passed
				require.NotNil(t, mkdirStage)
				mkdirOptions := mkdirStage.Options.(*osbuild.MkdirStageOptions)
				assert.Equal(t, "/path/to/dir", mkdirOptions.Paths[0].Path)
				assertBootcDeploymentAndBindMount(t, mkdirStage)
			} else {
				assert.Nil(t, mkdirStage)
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

			selinuxStage := findStage("org.osbuild.selinux", pipeline.Stages)

			assert.NotNil(t, selinuxStage)

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

func TestRawBootcPXE(t *testing.T) {
	rawBootcPipeline := makeFakeRawBootcPipeline()
	rawBootcPipeline.KernelVersion = "5.14.0-611.4.1.el9_7.x86_64"
	rawBootcPipeline.LiveBoot = true

	pipeline, err := rawBootcPipeline.Serialize()
	require.NoError(t, err)

	// Check for mkdir stages
	mkdirStages := findStages("org.osbuild.mkdir", pipeline.Stages)
	require.Greater(t, len(mkdirStages), 0)
	var mkdirPaths []string
	for _, s := range mkdirStages {
		opts := s.Options.(*osbuild.MkdirStageOptions)
		for _, p := range opts.Paths {
			mkdirPaths = append(mkdirPaths, p.Path)
		}
	}
	assert.Contains(t, mkdirPaths, "/usr")
	assert.Contains(t, mkdirPaths, "/proc")
}
