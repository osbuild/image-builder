package platform_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/images/pkg/platform"
)

func TestBootloaderString(t *testing.T) {
	assert.Equal(t, "none", platform.BOOTLOADER_NONE.String())
	assert.Equal(t, "grub2", platform.BOOTLOADER_GRUB2.String())
	assert.Equal(t, "zipl", platform.BOOTLOADER_ZIPL.String())
	assert.Equal(t, "uki", platform.BOOTLOADER_UKI.String())
	assert.Equal(t, "systemd", platform.BOOTLOADER_SYSTEMD.String())
}

func TestBootloaderStringUnknown(t *testing.T) {
	assert.PanicsWithError(t, "unknown bootloader 999", func() {
		_ = platform.Bootloader(999).String()
	})
}

func TestBootloaderRoundtrip(t *testing.T) {
	bootloaders := []platform.Bootloader{
		platform.BOOTLOADER_NONE,
		platform.BOOTLOADER_GRUB2,
		platform.BOOTLOADER_ZIPL,
		platform.BOOTLOADER_UKI,
		platform.BOOTLOADER_SYSTEMD,
	}
	for _, bl := range bootloaders {
		data, err := json.Marshal(bl)
		assert.NoError(t, err)
		var got platform.Bootloader
		err = json.Unmarshal(data, &got)
		assert.NoError(t, err)
		assert.Equal(t, bl, got)
	}
}

func TestImageFormatString(t *testing.T) {
	assert.Equal(t, "unset", platform.FORMAT_UNSET.String())
	assert.Equal(t, "ova", platform.FORMAT_OVA.String())
}

func TestImageFormatStringUnknown(t *testing.T) {
	assert.PanicsWithError(t, "unknown image format 999", func() {
		_ = platform.ImageFormat(999).String()
	})
}

func TestImageFormatUnmarshal(t *testing.T) {
	ifmts := []platform.ImageFormat{
		platform.FORMAT_UNSET,
		platform.FORMAT_RAW,
		platform.FORMAT_ISO,
		platform.FORMAT_QCOW2,
		platform.FORMAT_VMDK,
		platform.FORMAT_VHD,
		platform.FORMAT_GCE,
		platform.FORMAT_OVA,
	}
	for _, ifmt := range ifmts {
		inpJSON := fmt.Sprintf("%q", ifmt.String())
		inpYAML := ifmt.String()
		// json
		var f platform.ImageFormat
		err := json.Unmarshal([]byte(inpJSON), &f)
		assert.NoError(t, err)
		assert.Equal(t, ifmt, f)
		// now YAML
		err = yaml.Unmarshal([]byte(inpYAML), &f)
		assert.NoError(t, err)
		assert.Equal(t, ifmt, f)
	}
}
