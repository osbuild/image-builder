package distroidparser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/pkg/distro"
	"github.com/osbuild/image-builder/v73/pkg/distro/defs"
)

func TestDefaltParser(t *testing.T) {
	type testCase struct {
		idStr    string
		expected *distro.ID
		err      bool
	}

	testCases := []testCase{
		// Fedora
		{
			idStr:    "fedora-41",
			expected: &distro.ID{Name: "fedora", MajorVersion: 41, MinorVersion: -1},
		},
		{
			idStr:    "fedora-41.1",
			expected: &distro.ID{Name: "fedora", MajorVersion: 41, MinorVersion: 1},
		},
		{
			idStr: "fedora-41.1.1",
			err:   true,
		},
		// RHEL-7
		{
			idStr:    "rhel-7",
			expected: &distro.ID{Name: "rhel", MajorVersion: 7, MinorVersion: -1},
		},
		{
			idStr:    "rhel-79",
			expected: &distro.ID{Name: "rhel", MajorVersion: 79, MinorVersion: -1},
		},
		{
			idStr:    "rhel-7.9",
			expected: &distro.ID{Name: "rhel", MajorVersion: 7, MinorVersion: 9},
		},
		// RHEL-8
		{
			idStr:    "rhel-8",
			expected: &distro.ID{Name: "rhel", MajorVersion: 8, MinorVersion: -1},
		},
		{
			idStr:    "rhel-80",
			expected: &distro.ID{Name: "rhel", MajorVersion: 8, MinorVersion: 0},
		},
		{
			idStr:    "rhel-8.0",
			expected: &distro.ID{Name: "rhel", MajorVersion: 8, MinorVersion: 0},
		},
		{
			idStr:    "rhel-810",
			expected: &distro.ID{Name: "rhel", MajorVersion: 8, MinorVersion: 10},
		},
		{
			idStr:    "rhel-8.10",
			expected: &distro.ID{Name: "rhel", MajorVersion: 8, MinorVersion: 10},
		},
		{
			idStr:    "rhel-8100",
			expected: &distro.ID{Name: "rhel", MajorVersion: 8100, MinorVersion: -1},
		},
		{
			idStr: "rhel-8.1.1",
			err:   true,
		},
		// CentOS-8
		{
			idStr:    "centos-8",
			expected: &distro.ID{Name: "centos", MajorVersion: 8, MinorVersion: -1},
		},
		{
			idStr:    "centos-8.2",
			expected: &distro.ID{Name: "centos", MajorVersion: 8, MinorVersion: 2},
		},
		{
			idStr: "centos-8.2.2",
			err:   true,
		},
		// RHEL-9
		{
			idStr:    "rhel-9",
			expected: &distro.ID{Name: "rhel", MajorVersion: 9, MinorVersion: -1},
		},
		{
			idStr:    "rhel-90",
			expected: &distro.ID{Name: "rhel", MajorVersion: 9, MinorVersion: 0},
		},
		{
			idStr:    "rhel-9.0",
			expected: &distro.ID{Name: "rhel", MajorVersion: 9, MinorVersion: 0},
		},
		{
			idStr:    "rhel-910",
			expected: &distro.ID{Name: "rhel", MajorVersion: 9, MinorVersion: 10},
		},
		{
			idStr:    "rhel-9.10",
			expected: &distro.ID{Name: "rhel", MajorVersion: 9, MinorVersion: 10},
		},
		{
			idStr:    "rhel-9100",
			expected: &distro.ID{Name: "rhel", MajorVersion: 9100, MinorVersion: -1},
		},
		{
			idStr: "rhel-9.1.1",
			err:   true,
		},
		// CentOS-9
		{
			idStr:    "centos-9",
			expected: &distro.ID{Name: "centos", MajorVersion: 9, MinorVersion: -1},
		},
		{
			idStr:    "centos-9.2",
			expected: &distro.ID{Name: "centos", MajorVersion: 9, MinorVersion: 2},
		},
		{
			idStr: "centos-9.2.2",
			err:   true,
		},
		// RHEL-10
		{
			idStr:    "rhel-10",
			expected: &distro.ID{Name: "rhel", MajorVersion: 10, MinorVersion: -1},
		},
		{
			idStr:    "rhel-10.0",
			expected: &distro.ID{Name: "rhel", MajorVersion: 10, MinorVersion: 0},
		},
		{
			idStr:    "rhel-10.10",
			expected: &distro.ID{Name: "rhel", MajorVersion: 10, MinorVersion: 10},
		},
		{
			idStr: "rhel-10.1.1",
			err:   true,
		},
		// CentOS-10
		{
			idStr:    "centos-10",
			expected: &distro.ID{Name: "centos", MajorVersion: 10, MinorVersion: -1},
		},
		{
			idStr:    "centos-10.2",
			expected: &distro.ID{Name: "centos", MajorVersion: 10, MinorVersion: 2},
		},
		{
			idStr: "centos-10.2.2",
			err:   true,
		},
		// Non-existing distro
		{
			idStr:    "tuxdistro-1",
			expected: &distro.ID{Name: "tuxdistro", MajorVersion: 1, MinorVersion: -1},
		},
		{
			idStr:    "tuxdistro-1.2",
			expected: &distro.ID{Name: "tuxdistro", MajorVersion: 1, MinorVersion: 2},
		},
		{
			idStr:    "tuxdistro-123.321",
			expected: &distro.ID{Name: "tuxdistro", MajorVersion: 123, MinorVersion: 321},
		},
		{
			idStr: "tuxdistro-1.2.3",
			err:   true,
		},
	}

	parser := NewDefaultParser()
	for _, tc := range testCases {
		t.Run(tc.idStr, func(t *testing.T) {
			id, err := parser.Parse(tc.idStr)

			if tc.err {
				require.Error(t, err)
				require.Nil(t, id)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, id)
		})
	}
}

func TestParserDoubleMatch(t *testing.T) {
	loader := defs.BuiltinLoader()
	Parser := New(loader.ParseID, loader.ParseID)

	require.Panics(t, func() {
		_, _ = Parser.Parse("rhel-90")
	}, "Parser should panic when rhel-9.0 is matched by multiple parsers")
}

func TestNewParserWithLoader(t *testing.T) {
	loader := defs.BuiltinLoader()
	parser := NewParserWithLoader(loader)

	id, err := parser.Parse("rhel-810")
	require.NoError(t, err)
	require.Equal(t, &distro.ID{Name: "rhel", MajorVersion: 8, MinorVersion: 10}, id)

	std, err := parser.Standardize("rhel-810")
	require.NoError(t, err)
	require.Equal(t, "rhel-8.10", std)
}
