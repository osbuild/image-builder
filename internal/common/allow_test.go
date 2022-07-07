package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var mockAllowList = AllowList{"000000": {"fedora-*", "centos-*"}, "000001": {}}
var mockEmptyAllowList = AllowList{}

func TestIsAllowed(t *testing.T) {
	t.Run("orgId in allow list, allowed", func(t *testing.T) {
		actual, _ := mockAllowList.isAllowed("000000", "fedora-36")
		expected := true
		require.Equal(t, expected, actual)
	})

	t.Run("orgId in allow list, forbidden", func(t *testing.T) {
		actual, _ := mockAllowList.isAllowed("000001", "fedora-36")
		expected := false
		require.Equal(t, expected, actual)
	})

	t.Run("orgId not in allow list (forbidden)", func(t *testing.T) {
		actual, _ := mockAllowList.isAllowed("123456", "fedora-36")
		expected := false
		require.Equal(t, expected, actual)
	})
}

func TestLoadAllowList(t *testing.T) {
	t.Run("no allow file", func(t *testing.T) {
		actual, err := LoadAllowList("")
		expected := AllowList{}
		require.Nil(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("allow file does not exist", func(t *testing.T) {
		actual, err := LoadAllowList("testdata/nonexistantfile.json")
		msg := "No allow file found at testdata/nonexistantfile.json"
		require.Nil(t, actual)
		require.Error(t, err, msg)
	})

	t.Run("valid allowFile exists", func(t *testing.T) {
		actual, _ := LoadAllowList("testdata/allow.json")
		expected := AllowList{"000000": {"fedora-*", "centos-*"}, "000001": {}}
		require.Equal(t, expected, actual)
	})
}

func TestCheckAllow(t *testing.T) {
	distsDir := "../distribution/testdata/distributions"

	t.Run("unrestricted distro", func(t *testing.T) {
		actual, _ := CheckAllow("000000", "rhel-90", distsDir, mockEmptyAllowList)
		expected := true
		require.Equal(t, expected, actual)
	})

	t.Run("restricted distro, allowed", func(t *testing.T) {
		actual, _ := CheckAllow("000000", "centos-8", distsDir, mockAllowList)
		expected := true
		require.Equal(t, expected, actual)
	})

	t.Run("restricted distro, forbidden", func(t *testing.T) {
		actual, _ := CheckAllow("000001", "centos-8", distsDir, mockEmptyAllowList)
		expected := false
		require.Equal(t, expected, actual)
	})
}
