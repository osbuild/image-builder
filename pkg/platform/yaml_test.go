package platform_test

import (
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/platform"
)

func TestPlatformYamlSmoke(t *testing.T) {
	inputYAML := []byte(`
        arch: "x86_64"
        bios_platform: i386-pc
        uefi_vendor: "fedora"
        image_format: "qcow2"
        qcow2_compat: "1.1"
        packages:
          bios:
            - grub2-pc
        build_packages:
          bios:
            - grub2-pc-as-bp
        boot_files:
          - ["/usr/share/uboot/rpi_arm64/u-boot.bin", "/boot/efi/rpi-u-boot.bin"]
        extra_uefi_architectures:
          - "ia32"
`)
	var pc platform.Data
	err := yaml.Unmarshal(inputYAML, &pc)
	assert.NoError(t, err)
	expected := platform.Data{
		Arch:         common.Must(arch.FromString("x86_64")),
		BIOSPlatform: "i386-pc",
		UEFIVendor:   "fedora",
		ImageFormat:  platform.FORMAT_QCOW2,
		QCOW2Compat:  "1.1",
		Packages: map[string][]string{
			"bios": []string{"grub2-pc"},
		},
		BuildPackages: map[string][]string{
			"bios": []string{"grub2-pc-as-bp"},
		},
		BootFiles: [][2]string{
			{"/usr/share/uboot/rpi_arm64/u-boot.bin", "/boot/efi/rpi-u-boot.bin"},
		},
		FIPSMenu:               false,
		ExtraUEFIArchitectures: []string{"ia32"},
	}
	assert.Equal(t, expected, pc)
}

func TestPlatformYamlFIPSMenu(t *testing.T) {
	inputYAML := []byte(`
        arch: "x86_64"
        bios_platform: i386-pc
        uefi_vendor: "fedora"
        fips_menu: true
`)
	var pd platform.Data
	err := yaml.Unmarshal(inputYAML, &pd)
	assert.NoError(t, err)
	expected := platform.Data{
		Arch:         common.Must(arch.FromString("x86_64")),
		BIOSPlatform: "i386-pc",
		UEFIVendor:   "fedora",
		FIPSMenu:     true,
	}
	assert.Equal(t, expected, pd)
}
