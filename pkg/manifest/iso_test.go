package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestISOBoot(t *testing.T) {
	options := xorrisofsStageOptions("boot.iso", ISOCustomizations{Label: "test-iso-1", BootType: Grub2UEFIOnlyISOBoot})
	assert.Nil(t, options.Boot)
	assert.Equal(t, "", options.IsohybridMBR)
	assert.Equal(t, "", options.Grub2MBR)
	assert.Equal(t, "images/efiboot.img", options.EFI)

	options = xorrisofsStageOptions("boot.iso", ISOCustomizations{Label: "test-iso-1", BootType: SyslinuxISOBoot})
	require.NotNil(t, options.Boot)
	assert.Equal(t, "isolinux/isolinux.bin", options.Boot.Image)
	assert.Equal(t, "/usr/share/syslinux/isohdpfx.bin", options.IsohybridMBR)
	assert.Equal(t, "", options.Grub2MBR)
	assert.Equal(t, "images/efiboot.img", options.EFI)

	options = xorrisofsStageOptions("boot.iso", ISOCustomizations{Label: "test-iso-1", BootType: Grub2ISOBoot})
	require.NotNil(t, options.Boot)
	assert.Equal(t, "images/eltorito.img", options.Boot.Image)
	assert.Equal(t, "/usr/lib/grub/i386-pc/boot_hybrid.img", options.Grub2MBR)
	assert.Equal(t, "", options.IsohybridMBR)
	assert.Equal(t, "images/efiboot.img", options.EFI)

	options = xorrisofsStageOptions("boot.iso", ISOCustomizations{Label: "test-iso-1", BootType: Grub2ISOBoot, Preparer: "Test", Publisher: "Tester"})
	require.NotNil(t, options.Boot)
	assert.Equal(t, "images/eltorito.img", options.Boot.Image)
	assert.Equal(t, "/usr/lib/grub/i386-pc/boot_hybrid.img", options.Grub2MBR)
	assert.Equal(t, "", options.IsohybridMBR)
	assert.Equal(t, "Test", options.Prep)
	assert.Equal(t, "Tester", options.Pub)
	assert.Equal(t, "images/efiboot.img", options.EFI)

	options = xorrisofsStageOptions("boot.iso", ISOCustomizations{Label: "test-iso-1", BootType: Grub2PPCISOBoot, Preparer: "Test", Publisher: "Tester"})
	require.Nil(t, options.Boot)
	assert.Equal(t, "", options.Grub2MBR)
	assert.Equal(t, "", options.IsohybridMBR)
	assert.Equal(t, "Test", options.Prep)
	assert.Equal(t, "Tester", options.Pub)
	assert.Equal(t, true, options.CHRPBoot)
	assert.Equal(t, "test-iso-1", options.VolSet)

	options = xorrisofsStageOptions("boot.iso", ISOCustomizations{Label: "test-iso-1", BootType: S390ISOBoot, Preparer: "Test", Publisher: "Tester"})
	require.NotNil(t, options.Boot)
	assert.Equal(t, "images/cdboot.img", options.Boot.Image)
	assert.Equal(t, "", options.IsohybridMBR)
	assert.Equal(t, "", options.Grub2MBR)
	assert.Equal(t, "", options.EFI)
}
