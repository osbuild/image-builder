package fsnode

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/internal/common"
)

func TestBaseFsNodeValidate(t *testing.T) {
	testCases := []struct {
		Node  baseFsNode
		Error bool
	}{
		// PATH
		// relative path is not allowed
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "relative/path/file",
				},
			},
			Error: true,
		},
		// path ending with slash is not allowed
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/dir/with/trailing/slash/",
				},
			},
			Error: true,
		},
		// empty path is not allowed
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "",
				},
			},
			Error: true,
		},
		// path must be canonical
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/dir/../file",
				},
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/dir/./file",
				},
			},
			Error: true,
		},
		// valid paths
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
				},
			},
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/dir",
				},
			},
		},
		// MODE
		// invalid mode
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					Mode: common.ToPtr(os.FileMode(os.ModeDir)),
				},
			},
			Error: true,
		},
		// valid mode
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					Mode: common.ToPtr(os.FileMode(0o644)),
				},
			},
		},
		// USER
		// invalid user
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					User: "",
				},
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					User: "invalid@@@user",
				},
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					User: int64(-1),
				},
			},
			Error: true,
		},
		// valid user
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					User: "osbuild",
				},
			},
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path: "/etc/file",
					User: int64(0),
				},
			},
		},
		// GROUP
		// invalid group
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path:  "/etc/file",
					Group: "",
				},
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path:  "/etc/file",
					Group: "invalid@@@group",
				},
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path:  "/etc/file",
					Group: int64(-1),
				},
			},
			Error: true,
		},
		// valid group
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path:  "/etc/file",
					Group: "osbuild",
				},
			},
		},
		{
			Node: baseFsNode{
				baseFsNodeJSON{
					Path:  "/etc/file",
					Group: int64(0),
				},
			},
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			err := testCase.Node.validate()
			if testCase.Error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFsNodeUnmarshalDir(t *testing.T) {
	inputYAML := `
path: /some/path
mode: 0644
user: 1000
group: group
ensure_parent_dirs: true
`
	var fsn Directory
	err := yaml.Unmarshal([]byte(inputYAML), &fsn)
	assert.NoError(t, err)
	expected, err := NewDirectory("/some/path", common.ToPtr(os.FileMode(0644)), float64(1000), "group", true)
	assert.NoError(t, err)
	assert.Equal(t, expected, &fsn)
}

func TestFsNodeUnmarshalFile(t *testing.T) {
	inputYAML := `
path: /some/path
mode: 0644
user: 1000
group: group
data: some-data
`
	var fsn File
	err := yaml.Unmarshal([]byte(inputYAML), &fsn)
	assert.NoError(t, err)
	expected, err := NewFile("/some/path", common.ToPtr(os.FileMode(0644)), float64(1000), "group", []byte("some-data"))
	assert.NoError(t, err)
	assert.Equal(t, expected, &fsn)
}

func TestFsNodeUnmarshalBadFile(t *testing.T) {
	for _, tc := range []struct {
		inputYAML   string
		expectedErr string
	}{
		{`path: 123`, `json: cannot unmarshal number into Go struct field .*.[pP]ath of type string`},
		{`mode: -rw-rw-r--`, `json: cannot unmarshal string into Go struct field .*.[mM]ode of type fs.FileMode`},
		{`mode: -1`, `cannot unmarshal number -1 into Go struct field .*.[mM]ode of type fs.FileMode`},
		{`mode: 5_000_000_000`, `json: cannot unmarshal number 5000000000 into Go struct field .*.[mM]ode of type fs.FileMode`},
		{"path: /foo\nuser: 3.14", `user ID must be int`},
		{"path: /foo\ngroup: 2.71", `group ID must be int`},
		{"path: /foo\nuser: -1", `user ID must be non-negative`},
		{"path: /foo\ngroup: a!b", `group name "a!b" doesn't conform to validating regex`},
		{"path: /foo\ndata: 1.61", `cannot unmarshal number into Go struct field .*.data of type string`},
		{"path: /foo\nextra: field", `unknown field "extra"`},
	} {
		var fsn File
		err := yaml.Unmarshal([]byte(tc.inputYAML), &fsn)
		assert.Error(t, err)
		assert.Regexp(t, tc.expectedErr, err.Error())
	}
}

func TestFsNodeUnmarshalBadDir(t *testing.T) {
	for _, tc := range []struct {
		inputYAML   string
		expectedErr string
	}{
		{"path: /foo\nensure_parent_dirs: maybe", `json: cannot unmarshal string into Go struct field .ensure_parent_dirs of type bool`},
		{"path: /foo\nextra: field", `unknown field "extra"`},
	} {
		var fsn Directory
		err := yaml.Unmarshal([]byte(tc.inputYAML), &fsn)
		assert.ErrorContains(t, err, tc.expectedErr)
	}
}
