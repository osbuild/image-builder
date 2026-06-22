package imagefilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/pkg/distrofactory"
)

func TestImageFilterFilter(t *testing.T) {
	fac := distrofactory.NewTestDefault()

	for _, tc := range []struct {
		searchExpr            []string
		distro, arch, imgType string
		expectsMatch          bool
	}{
		// no prefix is a "fuzzy" filter and will check distro/arch/imgType
		{[]string{"foo"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"test-distro-1"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"test-distro*"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"test_arch3"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"qcow2"}, "test-distro-1", "test_arch3", "qcow2", true},
		// distro: prefix (exact matches only)
		{[]string{"distro:bar"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"distro:test-distro-1"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"distro:test-distro"}, "test-distro-1", "test_arch3", "qcow2", false},
		// arch: prefix
		{[]string{"arch:amd64"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"arch:test_arch3"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"arch:test_ar"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"arch:test_ar*"}, "test-distro-1", "test_arch3", "qcow2", true},
		// type: prefix
		{[]string{"type:ami"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"type:qcow2"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"type:qcow"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"type:qcow?"}, "test-distro-1", "test_arch3", "qcow2", true},
		// type: alias tests
		{[]string{"type:aws"}, "test-distro-3", "test_arch3", "ami", true},
		{[]string{"type:guest-image"}, "test-distro-3", "test_arch3", "qcow2", true},
		{[]string{"type:vsphere"}, "test-distro-3", "test_arch3", "vmdk", true},
		{[]string{"type:gcp"}, "test-distro-3", "test_arch3", "gce", true},
		{[]string{"type:non-existent-alias"}, "test-distro-3", "test_arch3", "qcow2", false},
		// bootmode: prefix
		{[]string{"bootmode:uefi"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"bootmode:hybrid"}, "test-distro-1", "test_arch3", "qcow2", true},
		// multiple filters are AND
		{[]string{"distro:test-distro-1", "type:ami"}, "test-distro-1", "test_arch3", "qcow2", false},
		{[]string{"distro:test-distro-1", "type:qcow2"}, "test-distro-1", "test_arch3", "qcow2", true},
		{[]string{"distro:test-distro-1", "arch:amd64", "type:qcow2"}, "test-distro-1", "test_arch3", "qcow2", false},
	} {
		// XXX: it would be nice if TestDistro would support constructing
		// like GetDistro("rhel-8.1:i386,amd64:ami,qcow2") instead of
		// the current very static setup
		di := fac.GetDistro(tc.distro)
		require.NotNil(t, di)
		ar, err := di.GetArch(tc.arch)
		require.NoError(t, err)
		im, err := ar.GetImageType(tc.imgType)
		require.NoError(t, err)
		ff, err := newFilter(tc.searchExpr...)
		require.NoError(t, err)

		match := ff.Matches(di, ar, im)
		assert.Equal(t, tc.expectsMatch, match, tc)
	}
}

func TestImageFilterError(t *testing.T) {
	_, err := newFilter("random:filter")
	require.EqualError(t, err, `unsupported filter prefix: "random" (supported: distro,arch,type,bootmode)`)
}

func TestSupportedFilters(t *testing.T) {
	assert.Contains(t, SupportedFilters(), "distro")
}
