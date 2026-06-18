package fsnode

import (
	"os"
	"testing"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewDirectory(t *testing.T) {
	testCases := []struct {
		name             string
		path             string
		mode             *os.FileMode
		user             interface{}
		group            interface{}
		ensureParentDirs bool
		expected         *Directory
	}{
		{
			name:             "directory-simple",
			path:             "/etc/dir",
			mode:             nil,
			user:             nil,
			group:            nil,
			ensureParentDirs: false,
			expected:         &Directory{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/dir", Mode: nil, User: nil, Group: nil}}, ensureParentDirs: false},
		},
		{
			name:             "directory-with-mode",
			path:             "/etc/dir",
			mode:             common.ToPtr(os.FileMode(0644)),
			user:             nil,
			group:            nil,
			ensureParentDirs: false,
			expected:         &Directory{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/dir", Mode: common.ToPtr(os.FileMode(0644)), User: nil, Group: nil}}, ensureParentDirs: false},
		},
		{
			name:             "directory-with-user-and-group-string",
			path:             "/etc/dir",
			mode:             nil,
			user:             "user",
			group:            "group",
			ensureParentDirs: false,
			expected:         &Directory{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/dir", Mode: nil, User: "user", Group: "group"}}, ensureParentDirs: false},
		},
		{
			name:             "directory-with-user-and-group-int64",
			path:             "/etc/dir",
			mode:             nil,
			user:             int64(1000),
			group:            int64(1000),
			ensureParentDirs: false,
			expected:         &Directory{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/dir", Mode: nil, User: int64(1000), Group: int64(1000)}}, ensureParentDirs: false},
		},
		{
			name:             "directory-with-ensure-parent-dirs",
			path:             "/etc/dir",
			mode:             nil,
			user:             nil,
			group:            nil,
			ensureParentDirs: true,
			expected:         &Directory{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/dir", Mode: nil, User: nil, Group: nil}}, ensureParentDirs: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := NewDirectory(tc.path, tc.mode, tc.user, tc.group, tc.ensureParentDirs)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, dir)
		})
	}
}
