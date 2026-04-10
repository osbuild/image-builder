package osinfo

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
)

func writeOSRelease(t *testing.T, root, id, versionID, name, platformID, variantID, idLike string) {
	t.Helper()

	err := os.MkdirAll(path.Join(root, "etc"), 0755)
	require.NoError(t, err)

	var buf string
	if id != "" {
		buf += "ID=" + id + "\n"
	}
	if versionID != "" {
		buf += "VERSION_ID=" + versionID + "\n"
	}
	if name != "" {
		buf += "NAME=" + name + "\n"
	}
	if platformID != "" {
		buf += "PLATFORM_ID=" + platformID + "\n"
	}
	if variantID != "" {
		buf += "VARIANT_ID=" + variantID + "\n"
	}
	if idLike != "" {
		buf += "ID_LIKE=" + idLike + "\n"
	}

	err = os.WriteFile(path.Join(root, "etc/os-release"), []byte(buf), 0644)
	require.NoError(t, err)
}

func createBootupdEFI(t *testing.T, root, uefiVendor string) {
	t.Helper()

	err := os.MkdirAll(path.Join(root, "usr/lib/bootupd/updates/EFI/BOOT"), 0755)
	require.NoError(t, err)
	err = os.Mkdir(path.Join(root, "usr/lib/bootupd/updates/EFI", uefiVendor), 0755)
	require.NoError(t, err)
}

func createImageCustomization(t *testing.T, root, custType string) {
	t.Helper()

	bibDir := path.Join(root, "usr/lib/bootc-image-builder/")
	err := os.MkdirAll(bibDir, 0755)
	require.NoError(t, err)

	var buf string
	var filename string
	switch custType {
	case "json":
		buf = `{
			"customizations": {
				"disk": {
					"partitions": [
						{
							"label": "var",
							"mountpoint": "/var",
							"fs_type": "ext4",
							"minsize": "3 GiB",
							"part_type": "01234567-89ab-cdef-0123-456789abcdef"
							}
					]
				}
			}
		}`
		filename = "config.json"
	case "toml":
		buf = `[[customizations.disk.partitions]]
label = "var"
mountpoint = "/var"
fs_type = "ext4"
minsize = "3 GiB"
part_type = "01234567-89ab-cdef-0123-456789abcdef"
`
		filename = "config.toml"
	case "broken":
		buf = "{"
		filename = "config.json"
	default:
		t.Errorf("unsupported customization type %s", custType)
	}

	err = os.WriteFile(path.Join(bibDir, filename), []byte(buf), 0644)
	require.NoError(t, err)
}

func TestLoadInfo(t *testing.T) {
	cases := []struct {
		desc       string
		id         string
		versionID  string
		name       string
		uefiVendor string
		platformID string
		variantID  string
		idLike     string
		custType   string
		errorStr   string
	}{
		{"happy", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "json", ""},
		{"happy-no-uefi", "fedora", "40", "Fedora Linux", "", "platform:f40", "coreos", "", "json", ""},
		{"happy-no-variant_id", "fedora", "40", "Fedora Linux", "", "platform:f40", "", "", "json", ""},
		{"happy-no-id", "fedora", "43", "Fedora Linux", "fedora", "", "", "", "json", ""},
		{"happy-with-id-like", "centos", "9", "CentOS Stream", "", "platform:el9", "", "rhel fedora", "json", ""},
		{"happy-no-cust", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "", ""},
		{"happy-toml", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "toml", ""},
		{"sad-no-id", "", "40", "Fedora Linux", "fedora", "platform:f40", "", "", "json", "missing ID in os-release"},
		{"sad-no-id", "fedora", "", "Fedora Linux", "fedora", "platform:f40", "", "", "json", "missing VERSION_ID in os-release"},
		{"sad-no-id", "fedora", "40", "", "fedora", "platform:f40", "", "", "json", "missing NAME in os-release"},
		{"sad-broken-json", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "broken", "cannot decode \"$ROOT/usr/lib/bootc-image-builder/config.json\": unexpected EOF"},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			root := t.TempDir()
			writeOSRelease(t, root, c.id, c.versionID, c.name, c.platformID, c.variantID, c.idLike)
			if c.uefiVendor != "" {
				createBootupdEFI(t, root, c.uefiVendor)

			}
			if c.custType != "" {
				createImageCustomization(t, root, c.custType)

			}

			info, err := Load(root)

			if c.errorStr != "" {
				require.EqualError(t, err, strings.ReplaceAll(c.errorStr, "$ROOT", root))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.id, info.OSRelease.ID)
			assert.Equal(t, c.versionID, info.OSRelease.VersionID)
			assert.Equal(t, c.name, info.OSRelease.Name)
			assert.Equal(t, c.uefiVendor, info.UEFIVendor)
			assert.Equal(t, c.platformID, info.OSRelease.PlatformID)
			assert.Equal(t, c.variantID, info.OSRelease.VariantID)
			if c.custType != "" {
				assert.NotNil(t, info.ImageCustomization)
				assert.NotNil(t, info.ImageCustomization.Disk)
				assert.NotEmpty(t, info.ImageCustomization.Disk.Partitions)
				part := info.ImageCustomization.Disk.Partitions[0]
				assert.Equal(t, part.Label, "var")
				assert.Equal(t, part.MinSize, uint64(3*1024*1024*1024))
				assert.Equal(t, part.FSType, "ext4")
				assert.Equal(t, part.Mountpoint, "/var")
				// TODO: Validate part.PartType when it is fixed
			} else {
				assert.Nil(t, info.ImageCustomization)
			}
			if c.idLike == "" {
				assert.Equal(t, len(info.OSRelease.IDLike), 0)
			} else {
				expected := strings.Split(c.idLike, " ")
				assert.Equal(t, expected, info.OSRelease.IDLike)
			}
		})
	}
}

func TestLoadInfoKernel(t *testing.T) {
	type testCase struct {
		desc     string
		dirs     []string
		files    []string
		expected *KernelInfo
	}

	cases := []testCase{
		// Incorrect kernel trees
		{"nodir", []string{}, []string{"not-a-dir"}, nil},
		{"novmlinuz", []string{"6.15.9-201.fc42.x86_64"}, []string{}, nil},
		{"novmlinuz2", []string{"6.15.9-201.fc42.x86_64", "6.14.11-300.fc42.x86_64"}, []string{"not-a-dir"}, nil},
		{"novmlinuz3", []string{"6.15.9-201.fc42.x86_64", "6.14.11-300.fc42.x86_64"}, []string{"6.15.9-201.fc42.x86_64/not-vmlinuz"}, nil},
		// Correct kernel trees
		{"noaboot", []string{"6.15.9-201.fc42.x86_64"}, []string{"6.15.9-201.fc42.x86_64/vmlinuz"}, &KernelInfo{"6.15.9-201.fc42.x86_64", false}},
		{"aboot", []string{"6.15.9-201.fc42.x86_64"}, []string{"6.15.9-201.fc42.x86_64/vmlinuz", "6.15.9-201.fc42.x86_64/aboot.img"}, &KernelInfo{"6.15.9-201.fc42.x86_64", true}},
		{"severaldirs", []string{"6.15.9-201.fc42.x86_64", "6.14.11-300.fc42.x86_64"}, []string{"6.14.11-300.fc42.x86_64/vmlinuz"}, &KernelInfo{"6.14.11-300.fc42.x86_64", false}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			root := t.TempDir()
			baseDir := path.Join(root, "usr/lib/modules")
			require.NoError(t, os.MkdirAll(baseDir, 0755))
			for _, dir := range c.dirs {
				dirPath := path.Join(baseDir, dir)
				require.NoError(t, os.MkdirAll(dirPath, 0755))
			}
			for _, file := range c.files {
				filePath := path.Join(baseDir, file)
				require.NoError(t, os.WriteFile(filePath, nil, 0644))
			}
			info, err := readKernelInfo(root)
			if c.expected == nil {
				require.Error(t, err)
				assert.Nil(t, info)
			} else {
				require.NoError(t, err)
				assert.Equal(t, info, c.expected)
			}
		})
	}
}

var fakePartitionTableYAML = `
.common:
  partitioning:
    guids:
      - &bios_boot_partition_guid "21686148-6449-6E6F-744E-656564454649"

partition_table:
  type: "gpt"
  partitions:
    - &bios_boot_partition
      size: 1 MiB
      uuid: 2866630c-0c7e-469c-bc82-c458e3fd6223
      bootable: true
      type: *bios_boot_partition_guid
`

func createPartitionTable(t *testing.T, root, fakePartitionTableYAML string, dest string) {
	t.Helper()

	dst := path.Join(root, dest)
	err := os.MkdirAll(path.Dir(dst), 0755)
	require.NoError(t, err)
	err = os.WriteFile(dst, []byte(fakePartitionTableYAML), 0644)
	require.NoError(t, err)
}

func TestLoadInfoPartitionTableHappy(t *testing.T) {
	dests := []string{
		"/usr/lib/bootc-image-builder/disk.yaml",
		"/usr/lib/image-builder/bootc/disk.yaml",
	}

	for _, dest := range dests {
		root := t.TempDir()
		writeOSRelease(t, root, "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos")
		createPartitionTable(t, root, fakePartitionTableYAML, dest)

		info, err := Load(root)
		require.NoError(t, err)
		assert.Equal(t, &disk.PartitionTable{
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Bootable: true,
					Size:     1 * datasizes.MiB,
					Type:     "21686148-6449-6E6F-744E-656564454649",
					UUID:     "2866630c-0c7e-469c-bc82-c458e3fd6223",
				},
			},
		}, info.PartitionTable)
	}
}

func TestLoadInfoPartitionTableSad(t *testing.T) {
	root := t.TempDir()
	writeOSRelease(t, root, "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos")
	createPartitionTable(t, root, "@invalidYAML", "/usr/lib/bootc-image-builder/disk.yaml")

	_, err := Load(root)
	assert.EqualError(t, err, fmt.Sprintf(`cannot parse disk definitions from "%s/usr/lib/bootc-image-builder/disk.yaml": yaml: found character that cannot start any token`, root))
}

var fakeISOYAML = `
label: "My-ISO"
kernel_args:
- "root=live:CDLABEL=My-ISO"
- "foo"
grub2:
  default: 5
  timeout: 30
  entries:
    - name: "My First Entry"
      linux: "/foo"
      initrd: "/bar"
    - name: "My Second Entry"
      linux: "/bar"
      initrd: "/foo"
`

func createISO(t *testing.T, root, fakeISOYAML string, dest string) {
	t.Helper()

	dst := path.Join(root, dest)
	err := os.MkdirAll(path.Dir(dst), 0755)
	require.NoError(t, err)
	err = os.WriteFile(dst, []byte(fakeISOYAML), 0644)
	require.NoError(t, err)
}

func TestLoadInfoISOHappy(t *testing.T) {
	dests := []string{
		"/usr/lib/bootc-image-builder/iso.yaml",
		"/usr/lib/image-builder/bootc/iso.yaml",
	}

	for _, dest := range dests {
		root := t.TempDir()
		writeOSRelease(t, root, "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos")
		createISO(t, root, fakeISOYAML, dest)

		info, err := Load(root)
		require.NoError(t, err)

		assert.Equal(t, "My-ISO", info.ISOInfo.Label)
		assert.Equal(t, []string{"root=live:CDLABEL=My-ISO", "foo"}, info.ISOInfo.KernelArgs)

		assert.Equal(t, 30, *info.ISOInfo.Grub2.Timeout)
		assert.Equal(t, 5, *info.ISOInfo.Grub2.Default)

		assert.Equal(t, 2, len(info.ISOInfo.Grub2.Entries))

		assert.Equal(t, "My First Entry", info.ISOInfo.Grub2.Entries[0].Name)
		assert.Equal(t, "/foo", info.ISOInfo.Grub2.Entries[0].Linux)
		assert.Equal(t, "/bar", info.ISOInfo.Grub2.Entries[0].Initrd)

		assert.Equal(t, "My Second Entry", info.ISOInfo.Grub2.Entries[1].Name)
		assert.Equal(t, "/bar", info.ISOInfo.Grub2.Entries[1].Linux)
		assert.Equal(t, "/foo", info.ISOInfo.Grub2.Entries[1].Initrd)
	}
}

func TestLoadInfoISOSad(t *testing.T) {
	root := t.TempDir()
	writeOSRelease(t, root, "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos")
	createISO(t, root, "@invalidYAML", "/usr/lib/bootc-image-builder/iso.yaml")

	_, err := Load(root)
	assert.EqualError(t, err, fmt.Sprintf(`cannot parse iso definitions from "%s/usr/lib/bootc-image-builder/iso.yaml": yaml: found character that cannot start any token`, root))
}

func TestLoadInfoUEFIVendorSearchPath(t *testing.T) {
	root := t.TempDir()

	writeOSRelease(t, root, "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos")
	err := os.MkdirAll(path.Join(root, "usr/lib/efi/shim/1.64/EFI/fedora"), 0755)
	assert.NoError(t, err)

	info, err := Load(root)
	assert.NoError(t, err)
	assert.Equal(t, "fedora", info.UEFIVendor)
}

func TestHasModules(t *testing.T) {
	type testCase struct {
		desc    string
		info    Info
		modules []string
		result  bool
		err     error
	}

	cases := []testCase{
		{"both empty", Info{}, []string{}, true, nil},
		{"empty initrd",
			Info{},
			[]string{"livenet"},
			false, fmt.Errorf("The initrd module list is empty")},
		{"empty request",
			Info{InitrdModules: []string{"livenet"}},
			[]string{},
			true, nil},
		{"missing module",
			Info{InitrdModules: []string{"qemu", "livenet"}},
			[]string{"livenet", "dmsquash-live"},
			false, nil},
		{"modules ok",
			Info{InitrdModules: []string{"qemu", "livenet", "dmsquash-live"}},
			[]string{"livenet", "dmsquash-live"},
			true, nil},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			r, err := c.info.HasModules(c.modules)
			assert.Equal(t, c.result, r)
			assert.Equal(t, err, c.err)
		})
	}
}
