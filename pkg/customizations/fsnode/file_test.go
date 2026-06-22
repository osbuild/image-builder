package fsnode

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewFile(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		data     []byte
		mode     *os.FileMode
		user     interface{}
		group    interface{}
		expected *File
	}{
		{
			name:     "empty-file",
			path:     "/etc/file",
			data:     nil,
			mode:     nil,
			user:     nil,
			group:    nil,
			expected: &File{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/file", Mode: nil, User: nil, Group: nil}}, data: nil},
		},
		{
			name:     "file-with-data",
			path:     "/etc/file",
			data:     []byte("data"),
			mode:     nil,
			user:     nil,
			group:    nil,
			expected: &File{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/file", Mode: nil, User: nil, Group: nil}}, data: []byte("data")},
		},
		{
			name:     "file-with-mode",
			path:     "/etc/file",
			data:     nil,
			mode:     common.ToPtr(os.FileMode(0644)),
			user:     nil,
			group:    nil,
			expected: &File{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/file", Mode: common.ToPtr(os.FileMode(0644)), User: nil, Group: nil}}, data: nil},
		},
		{
			name:     "file-with-user-and-group-string",
			path:     "/etc/file",
			data:     nil,
			mode:     nil,
			user:     "user",
			group:    "group",
			expected: &File{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/file", Mode: nil, User: "user", Group: "group"}}, data: nil},
		},
		{
			name:     "file-with-user-and-group-int64",
			path:     "/etc/file",
			data:     nil,
			mode:     nil,
			user:     int64(1000),
			group:    int64(1000),
			expected: &File{baseFsNode: baseFsNode{baseFsNodeJSON{Path: "/etc/file", Mode: nil, User: int64(1000), Group: int64(1000)}}, data: nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := NewFile(tc.path, tc.mode, tc.user, tc.group, tc.data)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, file)
		})
	}
}

func TestNewFileForURI(t *testing.T) {
	testFile1 := filepath.Join(t.TempDir(), "test1.txt")
	err := os.WriteFile(testFile1, nil, 0511)
	assert.NoError(t, err)

	file, err := NewFileForURI("/target/path", nil, nil, nil, testFile1)
	assert.NoError(t, err)
	assert.Equal(t, testFile1, file.URI())
	assert.Equal(t, "/target/path", file.Path())
	assert.Equal(t, os.FileMode(0511), file.Mode().Perm())
	// user/group are not take from the local file, just default to root
	assert.Equal(t, nil, file.User())
	assert.Equal(t, nil, file.Group())
}

func TestNewFileForURIBadURIs(t *testing.T) {
	tmpdir := t.TempDir()

	for _, tc := range []struct {
		ref         string
		expectedErr string
	}{
		{"/not/exists", `cannot include blueprint file: stat /not/exists: no such file or directory`},
		{"file://%g", `parse "file://%g": invalid URL escape "%g"`},
		{"gopher://foo.txt", "unsupported scheme for gopher://foo.txt (try file://)"},
		{tmpdir, fmt.Sprintf("%s is not a regular file", tmpdir)},
	} {

		_, err := NewFileForURI("/target/path", nil, nil, nil, tc.ref)
		assert.EqualError(t, err, tc.expectedErr)
	}
}
