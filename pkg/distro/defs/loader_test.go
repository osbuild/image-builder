package defs_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/distro/generic"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func makeTestImageType(t *testing.T, fakeContent string) defs.ImageTypeYAML {
	t.Helper()

	baseDir := makeFakeDistrosYAML(t, "", fakeContent)
	restore := defs.MockDataFS(baseDir)
	t.Cleanup(restore)

	distro, err := defs.NewDistroYAML("test-distro-1")
	require.NoError(t, err)
	it, ok := distro.ImageTypes()["test_type"]
	require.True(t, ok, "cannot find test_type in %s", fakeContent)
	return it
}

func makeFakeDistrosYAML(t *testing.T, distrosContent, imgTypesContent string) string {
	t.Helper()

	// if distros is unset use a sensible default
	if distrosContent == "" {
		distrosContent = `
distros:
 - name: test-distro-1
   vendor: test-vendor
   defs_path: test-distro-1/
`
	}

	tmpdir := t.TempDir()
	distrosPath := filepath.Join(tmpdir, "distros.yaml")
	err := os.WriteFile(distrosPath, []byte(distrosContent), 0644)
	assert.NoError(t, err)

	var di struct {
		Distros []defs.DistroYAML `yaml:"distros"`
	}
	err = yaml.Unmarshal([]byte(distrosContent), &di)
	assert.NoError(t, err)
	for _, d := range di.Distros {
		p := filepath.Join(tmpdir, d.DefsPath, "imagetypes.yaml")
		err = os.MkdirAll(filepath.Dir(p), 0755)
		assert.NoError(t, err)
		err = os.WriteFile(p, []byte(`---`+"\n"+imgTypesContent), 0644)
		assert.NoError(t, err)
	}

	return tmpdir
}

func makeFakeDistrosWithImageYAMLFiles(t *testing.T, distrosContent string, defsFiles map[string]string) string {
	t.Helper()

	if distrosContent == "" {
		distrosContent = `
distros:
 - name: test-distro-1
   vendor: test-vendor
   defs_path: test-distro-1/
`
	}

	tmpdir := t.TempDir()
	distrosPath := filepath.Join(tmpdir, "distros.yaml")
	err := os.WriteFile(distrosPath, []byte(distrosContent), 0644)
	assert.NoError(t, err)

	var di struct {
		Distros []defs.DistroYAML `yaml:"distros"`
	}
	err = yaml.Unmarshal([]byte(distrosContent), &di)
	assert.NoError(t, err)
	require.NotEmpty(t, di.Distros)

	for _, d := range di.Distros {
		p := filepath.Join(tmpdir, d.DefsPath)
		err = os.MkdirAll(p, 0755)
		assert.NoError(t, err)
	}

	defsBase := filepath.Join(tmpdir, di.Distros[0].DefsPath)
	for name, content := range defsFiles {
		p := filepath.Join(defsBase, name)
		err = os.WriteFile(p, []byte(content), 0644)
		assert.NoError(t, err)
	}

	return tmpdir
}

func TestYamlLintClean(t *testing.T) {
	_, err := exec.LookPath("yamllint")
	if errors.Is(err, exec.ErrNotFound) {
		t.Skip("this test needs yamllint")
	}
	require.NoError(t, err)

	pl, err := filepath.Glob("*/*.yaml")
	require.NoError(t, err)
	for _, p := range pl {
		cmd := exec.Command("yamllint", "--format=parsable", p)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		assert.NoError(t, err)
	}
}

func TestLoadConditionDistro(t *testing.T) {
	fakePkgsSetYaml := `
image_types:
  test_type:
    package_sets:
      os:
        - include: [inc1]
          exclude: [exc1]
          conditions:
            "some-description-1":
              when:
                distro_name: "test-distro"
              append:
                include: [from-condition-inc2]
                exclude: [from-condition-exc2]
            "some-description-2":
              when:
                distro_name: "other-distro"
              append:
                include: [inc3]
                exclude: [exc3]
      container:
        - include: [inc-cnt1]
          exclude: [exc-cnt1]
`
	it := makeTestImageType(t, fakePkgsSetYaml)

	pkgSet := it.PackageSets(distro.ID{Name: "test-distro", MajorVersion: 1}, "x86_64")
	assert.Equal(t, map[string]rpmmd.PackageSet{
		"os": {
			Include: []string{"from-condition-inc2", "inc1"},
			Exclude: []string{"exc1", "from-condition-exc2"},
		},
		"container": {
			Include: []string{"inc-cnt1"},
			Exclude: []string{"exc-cnt1"},
		},
	}, pkgSet)
}

func TestLoadExperimentalYamldirIsHonored(t *testing.T) {
	fakeImgTypesYAML := `
image_types:
  test_type:
    filename: foo
    image_func: disk
    package_sets:
     os:
      - include:
          - inc1
        exclude:
          - exc1
    platforms:
      - arch: x86_64

  unrelated:
    filename: bar
    image_func: disk
    package_sets:
     os:
      - include:
          - inc2
        exclude:
          - exc2
    platforms:
      - arch: x86_64
`
	fakeBaseDir := makeFakeDistrosYAML(t, "", fakeImgTypesYAML)

	t.Setenv("IMAGE_BUILDER_EXPERIMENTAL", fmt.Sprintf("yamldir=%s", fakeBaseDir))
	dist := generic.DistroFactory("test-distro-1")
	assert.NotNil(t, dist)
	ar, err := dist.GetArch("x86_64")
	assert.NoError(t, err)
	it, err := ar.GetImageType("test_type")
	assert.NoError(t, err)
	assert.Equal(t, "foo", it.Filename())
}

func TestLoadYamlMergingWorks(t *testing.T) {
	fakePkgsSetYaml := `
.common:
  base: &base_pkgset
    include: [from-base-inc]
    exclude: [from-base-exc]
    conditions:
      "some description 1":
        when:
          distro_name: "test-distro"
        append:
          include: [from-base-condition-inc]
          exclude: [from-base-condition-exc]
image_types:
  other_type:
    package_sets:
     os:
      - &other_type_pkgset
        include: [from-other-type-inc]
        exclude: [from-other-type-exc]
  test_type:
    package_sets:
     os:
      - *base_pkgset
      - *other_type_pkgset
      - include: [from-type-inc]
        exclude: [from-type-exc]
        conditions:
          "some description 2":
            when:
              distro_name: "test-distro"
            append:
              include: [from-condition-inc]
              exclude: [from-condition-exc]
`
	it := makeTestImageType(t, fakePkgsSetYaml)

	pkgSet := it.PackageSets(distro.ID{Name: "test-distro", MajorVersion: 1}, "x86_64")
	assert.Equal(t, map[string]rpmmd.PackageSet{
		"os": {
			Include: []string{"from-base-condition-inc", "from-base-inc", "from-condition-inc", "from-other-type-inc", "from-type-inc"},
			Exclude: []string{"from-base-condition-exc", "from-base-exc", "from-condition-exc", "from-other-type-exc", "from-type-exc"},
		},
	}, pkgSet)
}

func TestDefsPartitionTable(t *testing.T) {
	fakeDistroYaml := `
image_types:
  test_type:
    partition_table:
      test_arch:
        size: 1_000_000_000
        uuid: "D209C89E-EA5E-4FBD-B161-B461CCE297E0"
        type: "gpt"
        partitions:
          - size: 1_048_576
            bootable: true
            attrs:
              - 50
              - 51
          - payload_type: filesystem
            size: 209_715_200
            payload:
              type: vfat
              mountpoint: "/boot/efi"
              label: "ESP"
              fstab_options: "defaults,uid=0,gid=0,umask=077,shortname=winnt"
              fstab_freq: 0
              fstab_passno: 2
          - payload_type: "luks"
            payload:
              label: "crypt_root"
              cipher: "cipher_null"
              passphrase: "osbuild"
              pbkdf:
                iterations: 4
              clevis:
                pin: "null"
                remove_passphrase: true
              payload_type: "lvm"
              payload:
                name: "rootvg"
                description: "bla"
                logical_volumes:
                  - size: 8_589_934_592  # 8 * datasizes.GibiByte,
                    name: rootlv
                    payload_type: "filesystem"
                    payload:
                      type: ext4
                      mountpoint: "/"
`
	it := makeTestImageType(t, fakeDistroYaml)

	partTable, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &disk.PartitionTable{
		Size: 1_000_000_000,
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Size:     1048576,
				Bootable: true,
				Attrs:    []uint{50, 51},
			},
			{
				Size: 209_715_200,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					Mountpoint:   "/boot/efi",
					Label:        "ESP",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			}, {
				Payload: &disk.LUKSContainer{
					Label:      "crypt_root",
					Cipher:     "cipher_null",
					Passphrase: "osbuild",
					PBKDF: disk.Argon2id{
						Iterations: 4,
					},
					Clevis: &disk.ClevisBind{
						Pin:              "null",
						RemovePassphrase: true,
					},
					Payload: &disk.LVMVolumeGroup{
						Name:        "rootvg",
						Description: "bla",
						LogicalVolumes: []disk.LVMLogicalVolume{
							{
								Name: "rootlv",
								Size: 8_589_934_592,
								Payload: &disk.Filesystem{
									Type:       "ext4",
									Mountpoint: "/",
								},
							},
						},
					},
				},
			},
		},
	}, partTable)
}

func TestDefsPartitionTableFilesystemDistroDefault(t *testing.T) {
	fakeDistrosYaml := `
distros:
  - name: test-distro-1
    defs_path: test-distro
    default_fs_type: ext4
`
	fakeImageTypesYaml := `
image_types:
  test_type:
    filename: test.img
    platforms:
      - arch: x86_64
    partition_table:
      test_arch:
        partitions:
          - payload_type: filesystem
            payload:
              # note that no "type: <fstype>" is set here
              mountpoint: "/"
`
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYaml, fakeImageTypesYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()
	td, err := defs.NewDistroYAML("test-distro-1")
	require.NoError(t, err)
	it := td.ImageTypes()["test_type"]
	require.NotNil(t, it)

	partTable, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &disk.PartitionTable{
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: "/",
				},
			},
		},
	}, partTable)
}

func TestDefsPartitionTableFilesystemPartTableOverride(t *testing.T) {
	fakeDistrosYaml := `
distros:
  - name: test-distro-1
    defs_path: test-distro
    default_fs_type: ext4
`
	fakeImageTypesYaml := `
image_types:
  test_type:
    filename: test.img
    platforms:
      - arch: x86_64
    partition_table:
      x86_64:
        partitions:
    partition_tables_override:
      conditions:
        "test condition":
          when:
            distro_name: test-distro
          override:
            x86_64:
              partitions:
                - payload_type: filesystem
                  payload:
                    # note that no "type: <fstype>" is set here
                    mountpoint: "/"
`
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYaml, fakeImageTypesYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()
	td, err := defs.NewDistroYAML("test-distro-1")
	require.NoError(t, err)
	it := td.ImageTypes()["test_type"]
	require.NotNil(t, it)

	partTable, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "x86_64")
	require.NoError(t, err)
	assert.Equal(t, &disk.PartitionTable{
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: "/",
				},
			},
		},
	}, partTable)
}

func TestDefsPartitionTableFilesystemDistroDefaultErr(t *testing.T) {
	fakeDistrosYaml := `
distros:
  - name: test-distro-1
    defs_path: test-distro
`
	fakeImageTypesYaml := `
image_types:
  test_type:
    filename: test.img
    platforms:
      - arch: x86_64
    partition_table:
      test_arch:
        partitions:
          - payload_type: filesystem
            payload:
              # note that no "type: <fstype>" is set here
              mountpoint: "/"
`
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYaml, fakeImageTypesYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()
	_, err := defs.NewDistroYAML("test-distro-1")
	assert.EqualError(t, err, `no default fs set: mount "/" requires a filesystem but none set`)
}

var fakeImageTypesYaml = `
image_types:
  test_type:
    filename: "disk.img"
    image_func: "disk"
    platforms:
      - arch: x86_64
    partition_table:
      test_arch: &test_arch_pt
        size: 1_000_000_000
        uuid: "D209C89E-EA5E-4FBD-B161-B461CCE297E0"
        type: "gpt"
        partitions:
          - &default_part_0
            size: 1_048_576
            bootable: true
          - &default_part_1
            size: 2_147_483_648
            payload_type: "filesystem"
            payload: &default_part_1_payload
              type: "ext4"
              label: "root"
              mountpoint: "/"
              fstab_options: "defaults"
    partition_tables_override:
      conditions:
        "some description-0":
          when:
            version_equal: "0"
          override:
            test_arch:
              <<: *test_arch_pt
              partitions:
                - <<: *default_part_0
                  size: 111_111_111
        "some description-1":
          when:
            version_greater_or_equal: "1"
            version_less_than: "2"
          override:
            test_arch:
              <<: *test_arch_pt
              partitions:
                - <<: *default_part_0
                  size: 222_222_222
                - <<: *default_part_1
                  payload:
                    <<: *default_part_1_payload
                    fstab_options: "defaults,ro"
        "some description-2":
          when:
            version_greater_or_equal: "2"
          override:
            test_arch:
              <<: *test_arch_pt
              partitions:
                - <<: *default_part_0
                  size: 333_333_333
                - *default_part_1
`

func TestDefsPartitionTableOverrideGreatEqual(t *testing.T) {
	it := makeTestImageType(t, fakeImageTypesYaml)

	partTable, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &disk.PartitionTable{
		Size: 1_000_000_000,
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Size:     222_222_222,
				Bootable: true,
			},
			{
				Size: 2_147_483_648,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults,ro",
				},
			},
		},
	}, partTable)
}

func TestDefsPartitionTableOverridelessThan(t *testing.T) {
	fakeDistroYaml := `
image_types:
  test_type:
    partition_table:
      test_arch: &test_arch_pt
        size: 1_000_000_000
        uuid: "D209C89E-EA5E-4FBD-B161-B461CCE297E0"
        type: "gpt"
        partitions:
          - &default_part_0
            size: 1_048_576
            bootable: true
          - &default_part_1
            size: 2_147_483_648
            payload_type: "filesystem"
            payload: &default_part_1_payload
              type: "ext4"
              label: "root"
              mountpoint: "/"
              fstab_options: "defaults"
    partition_tables_override:
      conditions:
       "some description-2":
          when:
            version_less_than: "2"
          override:
            test_arch:
              <<: *test_arch_pt
              partitions:
                - <<: *default_part_0
                  size: 333_333_333
                - *default_part_1
`
	it := makeTestImageType(t, fakeDistroYaml)

	partTable, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &disk.PartitionTable{
		Size: 1_000_000_000,
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Size:     333_333_333,
				Bootable: true,
			},
			{
				Size: 2_147_483_648,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
				},
			},
		},
	}, partTable)
}

func TestDefsPartitionTableOverrideDistoName(t *testing.T) {
	fakeDistroYaml := `
image_types:
  test_type:
    partition_table:
      test_arch: &test_arch_pt
        partitions:
          - &default_part_0
            size: 1_048_576
            bootable: true
    partition_tables_override:
      conditions:
        "some description":
          when:
            distro_name: "test-distro"
          override:
            test_arch:
              partitions:
                - <<: *default_part_0
                  size: 111_111_111
`
	it := makeTestImageType(t, fakeDistroYaml)

	partTable, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &disk.PartitionTable{
		Partitions: []disk.Partition{
			{
				Size:     111_111_111,
				Bootable: true,
			},
		},
	}, partTable)
}

func TestDefsDistroImageConfig(t *testing.T) {
	fakeDistroYaml := `
distros:
  - name: test-distro-1
    vendor: test-vendor
    defs_path: test-distro-1/
    image_config:
      default:
        locale: "C.UTF-8"
        timezone: "OverrideTZ"
        users:
          - name: testuser
      conditions:
        "centos oscap datastream path":
          when:
            distro_name: "centos"
          shallow_merge:
            timezone: "OverrideTZ"
`

	fakeImageTypeYaml := `
image_types:
  test_type:
    filename: foo
`
	baseDir := makeFakeDistrosYAML(t, fakeDistroYaml, fakeImageTypeYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()
	dist, err := defs.NewDistroYAML("test-distro-1")
	assert.NoError(t, err)
	assert.Equal(t, dist.ImageConfig(), &distro.ImageConfig{
		Locale:   common.ToPtr("C.UTF-8"),
		Timezone: common.ToPtr("OverrideTZ"),
		Users:    []users.User{{Name: "testuser"}},
	})
}

func TestDefsPartitionTableErrorsNotForImageType(t *testing.T) {
	badDistroYamlMissingPartitionTable := `
image_types:
  test_type:
`
	badDistroYamlUnknownArch := `
image_types:
  test_type:
    partition_table:
      other_arch:
        partitions:
          - size: 1_048_576
`
	for _, tc := range []struct {
		badYaml     string
		expectedErr error
	}{
		{badDistroYamlMissingPartitionTable, defs.ErrNoPartitionTableForImgType},
		{badDistroYamlUnknownArch, defs.ErrNoPartitionTableForArch},
	} {
		it := makeTestImageType(t, tc.badYaml)

		_, err := it.PartitionTable(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
		assert.ErrorIs(t, err, tc.expectedErr)
	}
}

func TestImageTypeImageConfig(t *testing.T) {
	fakeDistroYaml := `
image_types:
  test_type:
    image_config:
      hostname: "foo"
      locale: "C.UTF-8"
      timezone: "DefaultTZ"
      default_kernel: "kernel"
      conditions:
        "some description for version lt":
          when:
            version_less_than: "2"
          shallow_merge:
            timezone: "OverrideTZ"
        "test-distro is version '1' (no minor) so considered '1' is > '1.4'":
          when:
            version_less_than: "1.4"
          shallow_merge:
            default_kernel: kernel-lt-14
        "some description for distro_name":
          when:
            distro_name: "test-distro"
          shallow_merge:
            locale: "en_US.UTF-8"
        "some description for architecture":
          when:
            arch: "test_arch"
          shallow_merge:
            hostname: "test-arch-hn"
        "some description for version":
          when:
            version_less_than: "2"
          shallow_merge:
            default_kernel: "kernel-lt-2"
`
	it := makeTestImageType(t, fakeDistroYaml)

	imgConfig := it.ImageConfig(distro.ID{Name: "test-distro", MajorVersion: 1, MinorVersion: -1}, "test_arch")
	assert.Equal(t, &distro.ImageConfig{
		Hostname:      common.ToPtr("test-arch-hn"),
		Locale:        common.ToPtr("en_US.UTF-8"),
		Timezone:      common.ToPtr("OverrideTZ"),
		DefaultKernel: common.ToPtr("kernel-lt-2"),
	}, imgConfig)
}

func TestImageTypes(t *testing.T) {
	fakeDistroYaml := `
image_types:
  test_type:
    name_aliases: ["qcow2"]
    filename: "disk.qcow2"
    compression: xz
    mime_type: "application/x-qemu-disk"
    environment:
      packages: ["cloud-init"]
      services: ["cloud-init.service"]
    bootable: true
    boot_iso: true
    rpm_ostree: false
    iso_label: "Workstation"
    default_size: 5_368_709_120  # 5 * datasizes.GibiByte
    image_func: "disk"
    exports: ["qcow2"]
    required_partition_sizes:
      "/": 1_073_741_824  # 1 * datasizes.GiB
    platforms:
      - arch: ppc64le
        bios_platform: "powerpc-ieee1275"
        image_format: "qcow2"
        qcow2_compat: "1.1"
        uefi_vendor: "{{.DistroVendor}}"
`
	makeTestImageType(t, fakeDistroYaml)

	distro, err := defs.NewDistroYAML("test-distro-1")
	require.NoError(t, err)

	imgTypes := distro.ImageTypes()
	assert.Len(t, imgTypes, 1)
	imgType := imgTypes["test_type"]
	assert.Equal(t, "test_type", imgType.Name())
	assert.Equal(t, []string{"qcow2"}, imgType.NameAliases)
	assert.Equal(t, "disk.qcow2", imgType.Filename)
	assert.Equal(t, "xz", imgType.Compression)
	assert.Equal(t, "application/x-qemu-disk", imgType.MimeType)
	assert.Equal(t, []string{"cloud-init"}, imgType.Environment.GetPackages())
	assert.Len(t, imgType.Environment.GetRepos(), 0)
	assert.Equal(t, []string{"cloud-init.service"}, imgType.Environment.GetServices())
	assert.Equal(t, true, imgType.Bootable)
	assert.Equal(t, true, imgType.BootISO)
	assert.Equal(t, false, imgType.RPMOSTree)
	assert.Equal(t, "Workstation", imgType.ISOLabel)
	assert.Equal(t, datasizes.Size(5*datasizes.GibiByte), imgType.DefaultSize)
	assert.Equal(t, "disk", imgType.Image)
	assert.Equal(t, []string{"qcow2"}, imgType.Exports)
	assert.Equal(t, map[string]datasizes.Size{"/": 1 * datasizes.GiB}, imgType.RequiredPartitionSizes)
	assert.Equal(t, []platform.Data{
		{
			Arch:         arch.ARCH_PPC64LE,
			BIOSPlatform: "powerpc-ieee1275",
			ImageFormat:  platform.FORMAT_QCOW2,
			QCOW2Compat:  "1.1",
			UEFIVendor:   "test-vendor",
		},
	}, imgType.InternalPlatforms)
}

func TestImageTypesUEFIVendorErrorWhenEmpty(t *testing.T) {
	fakeDistroYaml := `
image_types:
  test_type:
    filename: foo
    platforms:
      - arch: x86_64
        uefi_vendor: "{{.unavailable}}"
`
	baseDir := makeFakeDistrosYAML(t, "", fakeDistroYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()

	_, err := defs.NewDistroYAML("test-distro-1")
	require.ErrorContains(t, err, `cannot execute template for "vendor" field (is it set?)`)
}

var fakeDistroYamlISOConf = `
image_types:
  test_type:
    iso_config:
      rootfs_type: "squashfs"
`

func TestImageTypeISOConfig(t *testing.T) {
	it := makeTestImageType(t, fakeDistroYamlISOConf)

	installerConfig := it.ISOConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	assert.Equal(t, &distro.ISOConfig{
		RootfsType: common.ToPtr(manifest.SquashfsRootfs),
	}, installerConfig)
}

var fakeDistroYamlISOConfErofs = `
image_types:
  test_type:
    iso_config:
      rootfs_type: "erofs"
      erofs_options:
        compression:
          method: "zstd"
          level: 8
        options:
          - "all-fragments"
          - "dedupe"
        cluster-size: 262144
`

func TestImageTypeISOConfigErofs(t *testing.T) {
	it := makeTestImageType(t, fakeDistroYamlISOConfErofs)

	installerConfig := it.ISOConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	clusterSize := 262144
	compressionLevel := 8
	assert.Equal(t, &distro.ISOConfig{
		RootfsType: common.ToPtr(manifest.ErofsRootfs),
		ErofsOptions: &osbuild.ErofsStageOptions{
			Compression: &osbuild.ErofsCompression{
				Method: "zstd",
				Level:  &compressionLevel,
			},
			ExtendedOptions: []string{"all-fragments", "dedupe"},
			ClusterSize:     &clusterSize,
		},
	}, installerConfig)
}

var fakeDistroYamlInstallerConf = `
image_types:
  test_type:
    installer_config:
      additional_dracut_modules:
        - base-dracut-mod1
      additional_drivers:
        - base-drv1
      default_menu: 1
`

func TestImageTypeInstallerConfig(t *testing.T) {
	it := makeTestImageType(t, fakeDistroYamlInstallerConf)

	installerConfig, err := it.InstallerConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &distro.InstallerConfig{
		AdditionalDracutModules: []string{"base-dracut-mod1"},
		AdditionalDrivers:       []string{"base-drv1"},
		DefaultMenu:             common.ToPtr(1),
	}, installerConfig)
}

var fakeDistroYamlInstallerConfError = `
image_types:
  test_type:
    installer_config:
      flatpaks:
        - registry:
            url: oci+https://registry.fedora-project.org
            remote_name: fedora
          references:
            - ""
`

func TestImageTypeInstallerConfigError(t *testing.T) {
	it := makeTestImageType(t, fakeDistroYamlInstallerConfError)

	_, err := it.InstallerConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.Error(t, err)
	assert.ErrorContains(t, err, "empty flatpak ref after expansion")
}

func TestImageTypeInstallerConfigMergeVerLT(t *testing.T) {
	fakeDistroYaml := fakeDistroYamlInstallerConf + `
      conditions:
        "some description":
          when:
            version_less_than: "2"
          shallow_merge:
            additional_dracut_modules:
              - override-dracut-mod1
            default_menu: 2
`
	it := makeTestImageType(t, fakeDistroYaml)

	installerConfig, err := it.InstallerConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &distro.InstallerConfig{
		// AdditionalDrivers merged from parent
		AdditionalDrivers:       []string{"base-drv1"},
		DefaultMenu:             common.ToPtr(2),
		AdditionalDracutModules: []string{"override-dracut-mod1"},
	}, installerConfig)
}

func TestImageTypeInstallerConfigMergeDistroName(t *testing.T) {
	fakeDistroYaml := fakeDistroYamlInstallerConf + `
      conditions:
        "some description":
          when:
            distro_name: "test-distro"
          shallow_merge:
            additional_dracut_modules:
              - override-dracut-mod1
            additional_drivers:
              - override-drv1
`
	it := makeTestImageType(t, fakeDistroYaml)

	installerConfig, err := it.InstallerConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &distro.InstallerConfig{
		AdditionalDracutModules: []string{"override-dracut-mod1"},
		AdditionalDrivers:       []string{"override-drv1"},
		DefaultMenu:             common.ToPtr(1),
	}, installerConfig)
}

func TestImageTypeInstallerConfigMergeArch(t *testing.T) {
	fakeDistroYaml := fakeDistroYamlInstallerConf + `
      conditions:
        "some description":
          when:
            arch: "test_arch"
          shallow_merge:
            additional_drivers:
              - override-drv1
`
	it := makeTestImageType(t, fakeDistroYaml)

	installerConfig, err := it.InstallerConfig(distro.ID{Name: "test-distro", MajorVersion: 1}, "test_arch")
	require.NoError(t, err)
	assert.Equal(t, &distro.InstallerConfig{
		AdditionalDrivers: []string{"override-drv1"},
		// AdditionalDracutModules,ISORootfsType merged from parent
		AdditionalDracutModules: []string{"base-dracut-mod1"},
		DefaultMenu:             common.ToPtr(1),
	}, installerConfig)
}

var fakeDistrosYAML = `
distros:
  - &fedora_rawhide
    name: fedora-43
    preview: true
    os_version: 43
    release_version: 43
    module_platform_id: platform:f43
    product: "Fedora"
    defs_path: fedora
    iso_label_tmpl: "{{.Product}}-ISO"
    runner: &fedora_runner
      name: org.osbuild.fedora43
      build_packages: ["glibc"]
    bootstrap_containers:
      x86_64: "registry.fedoraproject.org/fedora-toolbox:{{.MajorVersion}}"
    oscap_profiles_allowlist:
      - "xccdf_org.ssgproject.content_profile_ospp"

  - &fedora_stable
    <<: *fedora_rawhide
    name: "fedora-{{.MajorVersion}}"
    match: "fedora-[0-9]*"
    preview: false
    os_version: "{{.MajorVersion}}"
    release_version: "{{.MajorVersion}}"
    module_platform_id: "platform:f{{.MajorVersion}}"
    runner:
      <<: *fedora_runner
      name: org.osbuild.fedora{{.MajorVersion}}
    bootstrap_containers:
      x86_64: "registry.fedoraproject.org/fedora-toolbox:{{.MajorVersion}}"

  - name: centos-10
    product: "CentOS Stream"
    os_version: "10-stream"
    release_version: 10
    module_platform_id: "platform:el10"
    vendor: "centos"
    default_fs_type: "xfs"
    defs_path: rhel-10

  - name: "rhel-{{.MajorVersion}}.{{.MinorVersion}}"
    match: "rhel-10.*"
    product: "Red Hat Enterprise Linux"
    os_version: "{{.MajorVersion}}.{{.MinorVersion}}"
    release_version: "{{.MajorVersion}}"
    module_platform_id: "platform:el{{.MajorVersion}}"
    vendor: "redhat"
    default_fs_type: "xfs"
    defs_path: rhel-10
    tweaks:
      rpmkeys:
        binary_path: "/chickens"
        ignore_build_import_failures: true
`

func TestDistrosLoadingExact(t *testing.T) {
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYAML, "")
	restore := defs.MockDataFS(baseDir)
	defer restore()

	dist, err := defs.NewDistroYAML("fedora-43")
	require.NoError(t, err)
	assert.Equal(t, dist, &defs.DistroYAML{
		Name:             "fedora-43",
		Preview:          true,
		OsVersion:        "43",
		ReleaseVersion:   "43",
		ModulePlatformID: "platform:f43",
		Product:          "Fedora",
		DefsPath:         "fedora",
		ISOLabelTmpl:     "{{.Product}}-ISO",
		Runner: runner.RunnerConf{
			Name:          "org.osbuild.fedora43",
			BuildPackages: []string{"glibc"},
		},
		BootstrapContainers: map[arch.Arch]string{
			arch.ARCH_X86_64: "registry.fedoraproject.org/fedora-toolbox:43",
		},
		OscapProfilesAllowList: []oscap.Profile{
			oscap.Ospp,
		},
		ID: distro.ID{Name: "fedora", MajorVersion: 43, MinorVersion: -1},
	})

	dist, err = defs.NewDistroYAML("centos-10")
	require.NoError(t, err)
	assert.Equal(t, dist, &defs.DistroYAML{
		Name:             "centos-10",
		Vendor:           "centos",
		OsVersion:        "10-stream",
		ReleaseVersion:   "10",
		ModulePlatformID: "platform:el10",
		Product:          "CentOS Stream",
		DefsPath:         "rhel-10",
		DefaultFSType:    disk.FS_XFS,
		ID:               distro.ID{Name: "centos", MajorVersion: 10, MinorVersion: -1},
	})
}

func TestDistrosLoadingFactoryCompat(t *testing.T) {
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYAML, "")
	restore := defs.MockDataFS(baseDir)
	defer restore()

	dist, err := defs.NewDistroYAML("rhel-10.1")
	require.NoError(t, err)
	assert.Equal(t, dist, &defs.DistroYAML{
		Name:             "rhel-10.1",
		Match:            "rhel-10.*",
		Vendor:           "redhat",
		OsVersion:        "10.1",
		ReleaseVersion:   "10",
		ModulePlatformID: "platform:el10",
		Product:          "Red Hat Enterprise Linux",
		DefsPath:         "rhel-10",
		DefaultFSType:    disk.FS_XFS,
		ID:               distro.ID{Name: "rhel", MajorVersion: 10, MinorVersion: 1},
		Tweaks: &distro.Tweaks{
			RPMKeys: &distro.RPMKeysTweaks{
				BinPath:                   "/chickens",
				IgnoreBuildImportFailures: true,
			},
		},
	})

	dist, err = defs.NewDistroYAML("fedora-40")
	require.NoError(t, err)
	assert.Equal(t, dist, &defs.DistroYAML{
		Name:             "fedora-40",
		Match:            "fedora-[0-9]*",
		OsVersion:        "40",
		ReleaseVersion:   "40",
		ModulePlatformID: "platform:f40",
		Product:          "Fedora",
		DefsPath:         "fedora",
		ISOLabelTmpl:     "{{.Product}}-ISO",
		Runner: runner.RunnerConf{
			Name:          "org.osbuild.fedora40",
			BuildPackages: []string{"glibc"},
		},
		BootstrapContainers: map[arch.Arch]string{
			arch.ARCH_X86_64: "registry.fedoraproject.org/fedora-toolbox:40",
		},
		OscapProfilesAllowList: []oscap.Profile{
			oscap.Ospp,
		},
		ID: distro.ID{Name: "fedora", MajorVersion: 40, MinorVersion: -1},
	})
}

func TestDistroYAMLCondition(t *testing.T) {
	fakeImageTypesYaml := `
image_types:
  ec2:
    filename: "disk.raw"
    image_func: "disk"
    exports: ["image"]
    platforms:
      - arch: x86_64
        uefi_vendor: "some-uefi-vendor"
  container:
    filename: "container.tar.gz"
    image_func: "container"
    exports: ["archive"]
    platforms:
      - arch: x86_64
`

	fakeDistrosYAML := `
distros:
 - &rhel8
   name: rhel-8
   conditions:
     "some image types are rhel-only":
       when:
         not_distro_name: "rhel"
       ignore_image_types:
         - ec2
   defs_path: test-distro/
 - <<: *rhel8
   name: centos-8
   defs_path: test-distro/
`
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYAML, fakeImageTypesYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()

	for _, tc := range []struct {
		distroNameVer    string
		expectedImgTypes []string
	}{
		{"rhel-8", []string{"container", "ec2"}},
		{"centos-8", []string{"container"}},
	} {
		t.Run(tc.distroNameVer, func(t *testing.T) {
			// Note that we load from the "generic" distro here as
			// the resolving of available image types happens on
			// this layer. XXX: consolidate it to the YAML level
			// already?

			distro := generic.DistroFactory(tc.distroNameVer)
			require.NotNil(t, distro)
			assert.Equal(t, tc.distroNameVer, distro.Name())
			a, err := distro.GetArch("x86_64")
			require.NoError(t, err)

			assert.Equal(t, tc.expectedImgTypes, a.ListImageTypes())
		})
	}
}

func TestDistrosLoadingNotFound(t *testing.T) {
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYAML, "")
	restore := defs.MockDataFS(baseDir)
	defer restore()

	distro, err := defs.NewDistroYAML("non-exiting")
	assert.Nil(t, err)
	assert.Nil(t, distro)
}

func TestWhenConditionEvalEmpty(t *testing.T) {
	wc := &defs.WhenCondition{}
	assert.Equal(t, wc.Eval(distro.ID{Name: "foo"}, "arch"), true)
}

func TestWhenConditionEvalSimple(t *testing.T) {
	wc := &defs.WhenCondition{DistroName: "distro"}
	assert.Equal(t, wc.Eval(distro.ID{Name: "distro"}, "other-arch"), true)
}

func TestWhenConditionEvalAnd(t *testing.T) {
	wc := &defs.WhenCondition{DistroName: "distro", Architecture: "arch"}
	assert.Equal(t, wc.Eval(distro.ID{Name: "distro"}, "other-arch"), false)
	assert.Equal(t, wc.Eval(distro.ID{Name: "distro"}, "arch"), true)
}

func TestImageTypesPlatformOverrides(t *testing.T) {
	fakeImageTypesYaml := `
image_types:
  server-qcow2:
    filename: "disk.qcow2"
    exports: ["qcow2"]
    platforms_override:
      conditions:
        "test platform override, simulate old distro is bios only":
          when:
            version_less_than: "2"
          override:
            - arch: x86_64
              # note no uefi_vendor here
    platforms:
      - arch: x86_64
        uefi_vendor: "some-uefi-vendor"
`

	fakeDistrosYAML := `
distros:
 - name: test-distro-1
   vendor: test-vendor
   defs_path: test-distro/
 - name: test-distro-2
   vendor: test-vendor
   defs_path: test-distro/
`
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYAML, fakeImageTypesYaml)
	restore := defs.MockDataFS(baseDir)
	defer restore()

	for _, tc := range []struct {
		distroNameVer      string
		expectedUEFIVendor string
	}{
		{"test-distro-1", ""},
		{"test-distro-2", "some-uefi-vendor"},
	} {

		distro, err := defs.NewDistroYAML(tc.distroNameVer)
		require.NoError(t, err)

		imgTypes := distro.ImageTypes()
		assert.Len(t, imgTypes, 1)
		imgType := imgTypes["server-qcow2"]
		platforms, err := imgType.PlatformsFor(distro.ID)
		assert.NoError(t, err)
		assert.Equal(t, []platform.Data{
			{
				Arch:       arch.ARCH_X86_64,
				UEFIVendor: tc.expectedUEFIVendor,
			},
		}, platforms)
	}
}

func TestImageTypesPlatformOverridesMultiMarchError(t *testing.T) {
	fakeImageTypesYaml := `
image_types:
  test_type:
    filename: "disk.qcow2"
    exports: ["qcow2"]
    platforms:
      - arch: x86_64
    platforms_override:
      conditions:
        "this is true":
          when:
            version_less_than: "2"
          override:
            - arch: x86_64
              uefi_vendor: "uefi-for-ver-2"
        "this is also true":
          when:
            version_less_than: "3"
          override:
            - arch: x86_64
              uefi_vendor: "uefi-for-ver-3"
`
	makeTestImageType(t, fakeImageTypesYaml)

	distro, err := defs.NewDistroYAML("test-distro-1")
	assert.NoError(t, err)
	require.NotNil(t, distro)
	imgTypes := distro.ImageTypes()
	assert.Len(t, imgTypes, 1)
	imgType := imgTypes["test_type"]
	_, err = imgType.PlatformsFor(distro.ID)
	assert.EqualError(t, err, `platform conditionals for image type "test_type" should match only once but matched 2 times`)
}

func TestLoadImageTypesMergesMultipleYAMLFiles(t *testing.T) {
	common := `
.common:
  shared_pkgset: &shared_pkgset
    include:
      - qemu-guest-agent
    exclude:
      - dracut-config-rescue
`
	fakeImageTypesYaml1 := `
image_types:
  "ec2":
    filename: "image.raw.xz"
    image_func: "disk"
    package_sets:
      os:
        - *shared_pkgset
        - include:
            - "@core"
            - "bash-completion"
            - "grubby"
            - "fwupd-efi"
`
	fakeImageTypesYaml2 := `
image_types:
  "iot":
    filename: "image.qcow2"
    image_func: disk
    package_sets:
      os:
        - *shared_pkgset
        - include:
            - "acl"
            - "bootc"
            - "bootupd"
            - "container-selinux"
            - "crun"
            - "cryptsetup"
            - "dnf"
          exclude:
            - "initial-setup-gui"
`
	baseDir := makeFakeDistrosWithImageYAMLFiles(t, "", map[string]string{
		"_common.yaml": common,
		"cloud.yaml":   fakeImageTypesYaml1,
		"iot.yaml":     fakeImageTypesYaml2,
	})
	restore := defs.MockDataFS(baseDir)
	t.Cleanup(restore)

	d, err := defs.LoadDistroWithoutImageTypes("test-distro-1")
	require.NoError(t, err)
	err = d.LoadImageTypes()
	require.NoError(t, err)

	imageTypes := d.ImageTypes()
	require.Len(t, imageTypes, 2)
	assert.Equal(t, "image.raw.xz", imageTypes["ec2"].Filename)
	assert.Equal(t, "image.qcow2", imageTypes["iot"].Filename)

	ec2Img := imageTypes["ec2"]
	ec2OS := (&ec2Img).PackageSets(d.ID, "x86_64")["os"]
	assert.Equal(t, []string{"@core", "bash-completion", "fwupd-efi", "grubby", "qemu-guest-agent"}, ec2OS.Include)
	assert.Equal(t, []string{"dracut-config-rescue"}, ec2OS.Exclude)

	iotImg := imageTypes["iot"]
	iotOS := (&iotImg).PackageSets(d.ID, "x86_64")["os"]
	assert.Equal(t, []string{"acl", "bootc", "bootupd", "container-selinux", "crun", "cryptsetup", "dnf", "qemu-guest-agent"}, iotOS.Include)
	assert.Equal(t, []string{"dracut-config-rescue", "initial-setup-gui"}, iotOS.Exclude)
}

func TestLoadImageTypesDuplicateImageTypeError(t *testing.T) {
	dup := `
image_types:
  "ec2":
    filename: "image.raw.xz"
    image_func: "disk"
    package_sets:
      os:
        - include:
            - "@core"
            - "bash-completion"
            - "grubby"
            - "fwupd-efi"
          exclude:
            - "dracut-config-rescue"
`
	baseDir := makeFakeDistrosWithImageYAMLFiles(t, "", map[string]string{
		"cloud.yaml":  dup,
		"cloud2.yaml": dup,
	})
	restore := defs.MockDataFS(baseDir)
	t.Cleanup(restore)

	d, err := defs.LoadDistroWithoutImageTypes("test-distro-1")
	require.NoError(t, err)
	err = d.LoadImageTypes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `duplicate image type ec2 found`)
}

func TestLoadImageTypesPrependsCommonYAML(t *testing.T) {
	common := `
.common:
  cloud_core_pkgset: &cloud_core_pkgset
    include:
      - qemu-guest-agent
    exclude:
      - dracut-config-rescue
`
	cloud := `
image_types:
  "ec2":
    filename: "image.raw.xz"
    image_func: "disk"
    package_sets:
      os:
        - *cloud_core_pkgset
        - include:
            - "@core"
            - "bash-completion"
            - "grubby"
            - "fwupd-efi"
`
	baseDir := makeFakeDistrosWithImageYAMLFiles(t, "", map[string]string{
		"_common.yaml": common,
		"main.yaml":    cloud,
	})
	restore := defs.MockDataFS(baseDir)
	t.Cleanup(restore)

	d, err := defs.LoadDistroWithoutImageTypes("test-distro-1")
	require.NoError(t, err)
	require.NoError(t, d.LoadImageTypes())

	imageType, ok := d.ImageTypes()["ec2"]
	require.True(t, ok)
	osSet := imageType.PackageSets(d.ID, "x86_64")["os"]
	assert.Equal(t, []string{"@core", "bash-completion", "fwupd-efi", "grubby", "qemu-guest-agent"}, osSet.Include)
	assert.Equal(t, []string{"dracut-config-rescue"}, osSet.Exclude)
}

func TestLoadImageTypesInvalidYAMLReturnsError(t *testing.T) {
	baseDir := makeFakeDistrosWithImageYAMLFiles(t, "", map[string]string{
		"broken.yaml": "image_types: [some invalid structure is living here",
	})
	restore := defs.MockDataFS(baseDir)
	t.Cleanup(restore)

	d, err := defs.LoadDistroWithoutImageTypes("test-distro-1")
	require.NoError(t, err)
	err = d.LoadImageTypes()
	require.Error(t, err)
}

func TestLoadImageTypesWithNoImageTypesLeavesImageTypesUnset(t *testing.T) {
	baseDir := makeFakeDistrosWithImageYAMLFiles(t, "", map[string]string{})
	restore := defs.MockDataFS(baseDir)
	t.Cleanup(restore)

	d, err := defs.LoadDistroWithoutImageTypes("test-distro-1")
	require.NoError(t, err)
	require.NoError(t, d.LoadImageTypes())
	assert.Nil(t, d.ImageTypes())
}

func TestDistrosLoadingMatchTransforms(t *testing.T) {
	fakeDistrosYAML := `
distros:
  - name: "rhel-{{.MajorVersion}}.{{.MinorVersion}}"
    match: '(?P<name>rhel)-(?P<major>8)\.?(?P<minor>[0-9]+)'
    os_version: "{{.MajorVersion}}.{{.MinorVersion}}"
    release_version: "{{.MajorVersion}}"
    module_platform_id: "platform:el{{.MajorVersion}}"
    defs_path: rhel-8
`
	baseDir := makeFakeDistrosYAML(t, fakeDistrosYAML, "")
	restore := defs.MockDataFS(baseDir)
	defer restore()

	for _, tc := range []struct {
		nameVer               string
		expectedDistroNameVer string
		expectedOsVersion     string
	}{
		{"rhel-8.1", "rhel-8.1", "8.1"},
		{"rhel-81", "rhel-8.1", "8.1"},
		{"rhel-8.9", "rhel-8.9", "8.9"},
		{"rhel-89", "rhel-8.9", "8.9"},
		{"rhel-8.10", "rhel-8.10", "8.10"},
		{"rhel-810", "rhel-8.10", "8.10"},
	} {
		dist, err := defs.NewDistroYAML(tc.nameVer)
		require.NoError(t, err)
		assert.Equal(t, dist, &defs.DistroYAML{
			Name:             tc.expectedDistroNameVer,
			Match:            `(?P<name>rhel)-(?P<major>8)\.?(?P<minor>[0-9]+)`,
			OsVersion:        tc.expectedOsVersion,
			ReleaseVersion:   "8",
			ModulePlatformID: "platform:el8",
			DefsPath:         "rhel-8",
			ID:               *common.Must(distro.ParseID(tc.expectedDistroNameVer)),
		})
	}
}
