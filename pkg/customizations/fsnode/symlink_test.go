package fsnode_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
)

func TestSymlinkYAML(t *testing.T) {
	for _, target := range []string{"usr/lib/systemd/systemd", "../lib/example", "/usr/bin/example"} {
		var link fsnode.Symlink
		require.NoError(t, yaml.Unmarshal([]byte("path: /init\ntarget: "+target+"\n"), &link))
		assert.Equal(t, target, link.Target)
	}
	for _, data := range []string{
		"path: init\ntarget: example",
		"path: /etc/../init\ntarget: example",
		"path: /\ntarget: example",
		"path: /init",
		"path: /init\ntarget: example\ntypo: true",
	} {
		var link fsnode.Symlink
		assert.Error(t, yaml.Unmarshal([]byte(data), &link), data)
	}
	_, err := fsnode.NewSymlink("/init", "")
	require.Error(t, err)
}

func TestNewSymlink(t *testing.T) {
	for _, target := range []string{"../lib/example", "/usr/bin/example", "./target", "missing"} {
		link, err := fsnode.NewSymlink("/etc/link", target)
		require.NoError(t, err)
		assert.Equal(t, &fsnode.Symlink{Path: "/etc/link", Target: target}, link)
	}
	for _, tc := range []struct{ path, target string }{
		{"", "target"}, {"relative", "target"}, {"/", "target"},
		{"/etc/../link", "target"}, {"/etc//link", "target"},
		{"/etc/link/", "target"}, {"/etc/./link", "target"},
	} {
		link, err := fsnode.NewSymlink(tc.path, tc.target)
		require.Error(t, err)
		assert.Nil(t, link)
		data, err := json.Marshal(fsnode.Symlink{Path: tc.path, Target: tc.target})
		require.NoError(t, err)
		var decoded fsnode.Symlink
		assert.Error(t, json.Unmarshal(data, &decoded))
	}
}

func TestSymlinkJSON(t *testing.T) {
	link, err := fsnode.NewSymlink("/etc/link", "../usr/share/example")
	require.NoError(t, err)
	data, err := json.Marshal(link)
	require.NoError(t, err)
	var decoded fsnode.Symlink
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, *link, decoded)
	assert.Error(t, json.Unmarshal([]byte(`{"path":"/link","target":"target","typo":true}`), &decoded))
	assert.Error(t, json.Unmarshal([]byte(`{"path":3,"target":"target"}`), &decoded))
}
