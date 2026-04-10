package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/images/internal/common"
)

func TestCurrentArchAMD64(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "amd64"
	assert.Equal(t, "x86_64", Current().String())
	assert.True(t, IsX86_64())
}

func TestCurrentArchARM(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "arm"
	assert.Equal(t, "arm", Current().String())
	assert.True(t, IsArm())
}

func TestCurrentArchARM64(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "arm64"
	assert.Equal(t, "aarch64", Current().String())
	assert.True(t, IsAarch64())
}

func TestCurrentArchPPC64LE(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "ppc64le"
	assert.Equal(t, "ppc64le", Current().String())
	assert.True(t, IsPPC())
}

func TestCurrentArchS390X(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "s390x"
	assert.Equal(t, "s390x", Current().String())
	assert.True(t, IsS390x())
}

func TestCurrentArchRiscv64(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "riscv64"
	assert.Equal(t, "riscv64", Current().String())
	assert.True(t, IsRISCV64())
}

func TestCurrentArchUnsupported(t *testing.T) {
	origRuntimeGOARCH := runtimeGOARCH
	defer func() { runtimeGOARCH = origRuntimeGOARCH }()
	runtimeGOARCH = "UNKNOWN"
	assert.PanicsWithError(t, `unsupported architecture "UNKNOWN"`, func() { Current() })
}

func TestFromStringUnsupported(t *testing.T) {
	_, err := FromString("UNKNOWN")
	assert.EqualError(t, err, `unsupported architecture "UNKNOWN"`)
}

func TestFromString(t *testing.T) {
	assert.Equal(t, ARCH_ARM, common.Must(FromString("arm")))
	assert.Equal(t, ARCH_ARM, common.Must(FromString("armv7")))
	assert.Equal(t, ARCH_AARCH64, common.Must(FromString("arm64")))
	assert.Equal(t, ARCH_AARCH64, common.Must(FromString("aarch64")))
	assert.Equal(t, ARCH_X86_64, common.Must(FromString("amd64")))
	assert.Equal(t, ARCH_X86_64, common.Must(FromString("x86_64")))
	assert.Equal(t, ARCH_S390X, common.Must(FromString("s390x")))
	assert.Equal(t, ARCH_PPC64LE, common.Must(FromString("ppc64le")))
	assert.Equal(t, ARCH_RISCV64, common.Must(FromString("riscv64")))
}

func TestUnmarshal(t *testing.T) {
	for _, tc := range []struct {
		inp      string
		expected Arch
	}{
		{"arch: arm", ARCH_ARM},
		{"arch: arm64", ARCH_AARCH64},
		{"arch: amd64", ARCH_X86_64},
		{"arch: s390x", ARCH_S390X},
		{"arch: ppc64le", ARCH_PPC64LE},
		{"arch: riscv64", ARCH_RISCV64},
	} {
		var v struct {
			Arch Arch
		}
		err := yaml.Unmarshal([]byte(tc.inp), &v)
		assert.NoError(t, err)
		assert.Equal(t, tc.expected, v.Arch)
	}
}
