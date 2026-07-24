package disk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/osbuild/image-builder/pkg/disk"
)

func TestPartitionTableTypeUnmarshalYAML(t *testing.T) {
	inputYAML := `dos`
	var partType disk.PartitionTableType

	err := yaml.Unmarshal([]byte(inputYAML), &partType)
	require.NoError(t, err)
	assert.Equal(t, disk.PT_DOS, partType)
}

func TestPartitionTableUnmarshalYAMLSimple(t *testing.T) {
	inputYAML := `
guids:
  - &bios_boot_partition_guid "21686148-6449-6E6F-744E-656564454649"
  - &efi_system_partition_guid "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
  - &filesystem_data_guid "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
uuids:
  - &bios_boot_partition_uuid "FAC7F1FB-3E8D-4137-A512-961DE09A5549"
  - &root_partition_uuid "6264D520-3FB9-423F-8AB8-7A0A8E3D3562"
  - &data_partition_uuid "CB07C243-BC44-4717-853E-28852021225B"
  - &efi_system_partition_uuid "68B2905B-DF3E-4FB3-80FA-49D1E773AA33"
  - &efi_filesystem_uuid "7B77-95E7"
partition_table:
  uuid: "D209C89E-EA5E-4FBD-B161-B461CCE297E0"
  type: "gpt"
  start_offset: "8 MiB"
  partitions:
    - size: 1_048_576  # 1 MiB
      bootable: true
      type: *bios_boot_partition_guid
      uuid: *bios_boot_partition_uuid
    - &default_partition_table_part_efi
      size: 209_715_200  # 200 MiB
      type: *efi_system_partition_guid
      uuid: *efi_system_partition_uuid
      payload_type: "filesystem"
      payload:
        type: vfat
        uuid: *efi_filesystem_uuid
        mountpoint: "/boot/efi"
        label: "ESP"
        fstab_options: "defaults,uid=0,gid=0,umask=077,shortname=winnt"
        fstab_freq: 0
        fstab_passno: 2
    - &default_partition_table_part_root
      size: "2 GiB"
      type: *filesystem_data_guid
      uuid: *root_partition_uuid
      payload_type: "filesystem"
      payload:
        type: "ext4"
        label: "root"
        mountpoint: "/"
        fstab_options: "defaults"
        fstab_freq: 0
        fstab_passno: 0
`
	var ptWrapper struct {
		PartitionTable disk.PartitionTable `yaml:"partition_table"`
	}

	err := yaml.Unmarshal([]byte(inputYAML), &ptWrapper)
	require.NoError(t, err)
	expected := disk.PartitionTable{
		UUID:        "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:        disk.PT_GPT,
		StartOffset: 8 * datasizes.MiB,
		Partitions: []disk.Partition{
			{
				Start:    0,
				Size:     1048576,
				Type:     "21686148-6449-6E6F-744E-656564454649",
				Bootable: true,
				UUID:     "FAC7F1FB-3E8D-4137-A512-961DE09A5549",
			},
			{
				Start:    0,
				Size:     209715200,
				Type:     "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
				Bootable: false,
				UUID:     "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         "7B77-95E7",
					Label:        "ESP",
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			}, {
				Start:    0,
				Size:     2147483648,
				Type:     "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
				Bootable: false,
				UUID:     "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
				Payload: &disk.Filesystem{
					Type:         "ext4",
					UUID:         "",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	}
	assert.Equal(t, expected, ptWrapper.PartitionTable)
}

func TestPartitionTableUnmarshalYAMLwithLUKS(t *testing.T) {
	inputYAML := `
partition_table:
  uuid: "D209C89E-EA5E-4FBD-B161-B461CCE297E0"
  type: "gpt"
  partitions:
    - size: 1_048_576  # 1 MiB
      bootable: true
    - payload_type: "luks"
      size: 987654321
      payload:
        label: "crypt_root"
        cipher: "cipher_null"
        passphrase: "osbuild"
        pbkdf:
          iterations: 4
        clevis:
          pin: "null"
        payload_type: "lvm"
        payload:
          name: "rootvg"
          description: "bla"
          logical_volumes:
            - size: 123456789
              name: "rootlv"
              payload_type: "filesystem"
              payload:
                type: "ext4"
                mountpoint: "/"
`
	var ptWrapper struct {
		PartitionTable disk.PartitionTable `yaml:"partition_table"`
	}

	err := yaml.Unmarshal([]byte(inputYAML), &ptWrapper)
	require.NoError(t, err)
	expected := disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: 2,
		Partitions: []disk.Partition{
			{
				Size:     1048576,
				Bootable: true,
			}, {
				Size: 987654321,
				Payload: &disk.LUKSContainer{
					Label:      "crypt_root",
					Cipher:     "cipher_null",
					Passphrase: "osbuild",
					PBKDF: disk.Argon2id{
						Iterations: 4,
					},
					Clevis: &disk.ClevisBind{
						Pin: "null",
					},
					Payload: &disk.LVMVolumeGroup{
						Name:        "rootvg",
						Description: "bla",
						LogicalVolumes: []disk.LVMLogicalVolume{
							{
								Name: "rootlv",
								Size: 123456789,
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
	}
	assert.Equal(t, expected, ptWrapper.PartitionTable)
}

func TestPartitionTableUnmarshalYAMLGrowRootToFillDisk(t *testing.T) {
	tests := map[string]struct {
		yaml     string
		expected *bool
	}{
		"absent": {
			yaml: `
partition_table:
  type: "gpt"
  partitions:
    - size: "1 MiB"
      payload_type: "filesystem"
      payload:
        type: "ext4"
        mountpoint: "/"
`,
			expected: nil,
		},
		"true": {
			yaml: `
partition_table:
  type: "gpt"
  grow_root_to_fill_disk: true
  partitions:
    - size: "1 MiB"
      payload_type: "filesystem"
      payload:
        type: "ext4"
        mountpoint: "/"
`,
			expected: common.ToPtr(true),
		},
		"false": {
			yaml: `
partition_table:
  type: "gpt"
  grow_root_to_fill_disk: false
  partitions:
    - size: "1 MiB"
      payload_type: "filesystem"
      payload:
        type: "ext4"
        mountpoint: "/"
`,
			expected: common.ToPtr(false),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var ptWrapper struct {
				PartitionTable disk.PartitionTable `yaml:"partition_table"`
			}
			err := yaml.Unmarshal([]byte(tc.yaml), &ptWrapper)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ptWrapper.PartitionTable.GrowRootToFillDisk)
		})
	}
}
