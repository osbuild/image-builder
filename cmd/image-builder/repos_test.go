package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/rpmmd"
)

func TestParseExtraRepoHappy(t *testing.T) {
	checkGPG := false

	cfg, err := parseExtraRepo("file:///path/to/repo")
	assert.NoError(t, err)
	assert.Equal(t, cfg, []rpmmd.RepoConfig{
		{
			Id:           "file:///path/to/repo",
			Name:         "file:///path/to/repo",
			BaseURLs:     []string{"file:///path/to/repo"},
			CheckGPG:     &checkGPG,
			CheckRepoGPG: &checkGPG,
		},
	})
}

func TestParseExtraRepoSad(t *testing.T) {
	_, err := parseExtraRepo("/just/a/path")
	assert.EqualError(t, err, `scheme missing in "/just/a/path", please prefix with e.g. file:`)
}
