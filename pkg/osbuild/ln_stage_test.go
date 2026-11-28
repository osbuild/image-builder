package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestGenSymlinkNodesStages(t *testing.T) {
	require.Empty(t, osbuild.GenSymlinkNodesStages(nil))
	stages := osbuild.GenSymlinkNodesStages([]*fsnode.Symlink{
		{Path: "/init", Target: "usr/lib/systemd/systemd"},
		{Path: "/etc/example", Target: "/usr/share/example"},
	})
	require.Len(t, stages, 1)
	data, err := json.Marshal(stages[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "org.osbuild.ln",
		"options": {"paths": [
			{"target": "usr/lib/systemd/systemd", "link_name": "tree:///init", "symbolic": true},
			{"target": "/usr/share/example", "link_name": "tree:///etc/example", "symbolic": true}
		]}
	}`, string(data))
}
