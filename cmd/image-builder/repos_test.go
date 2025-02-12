package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/rpmmd"
)

func TestParseRepoURLsHappy(t *testing.T) {
	checkGPG := false

	cfg, err := parseRepoURLs([]string{
		"file:///path/to/repo",
		"https://example.com/repo",
	}, "forced")
	assert.NoError(t, err)
	assert.Equal(t, cfg, []rpmmd.RepoConfig{
		{
			Id:           "forced-repo-0",
			Name:         "forced repo#0 /path/to/repo",
			BaseURLs:     []string{"file:///path/to/repo"},
			CheckGPG:     &checkGPG,
			CheckRepoGPG: &checkGPG,
		},
		{
			Id:           "forced-repo-1",
			Name:         "forced repo#1 example.com/repo",
			BaseURLs:     []string{"https://example.com/repo"},
			CheckGPG:     &checkGPG,
			CheckRepoGPG: &checkGPG,
		},
	})
}

func TestParseExtraRepoSad(t *testing.T) {
	_, err := parseRepoURLs([]string{"/just/a/path"}, "forced")
	assert.EqualError(t, err, `scheme missing in "/just/a/path", please prefix with e.g. file:// or https://`)

	_, err = parseRepoURLs([]string{"https://example.com", "/just/a/path"}, "forced")
	assert.EqualError(t, err, `scheme missing in "/just/a/path", please prefix with e.g. file:// or https://`)
}
