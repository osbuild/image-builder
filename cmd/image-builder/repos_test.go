package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/rpmmd"
)

func TestParseRepoURLsHappy(t *testing.T) {
	checkGPG := false

	cfg, err := parseRepoURLs([]string{
		"file:///path/to/repo",
		"https://example.com/repo",
	}, "forced")
	assert.NoError(t, err)
	assert.Equal(t, []rpmmd.RepoConfig{
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
	}, cfg)
}

func TestParseExtraRepoSad(t *testing.T) {
	_, err := parseRepoURLs([]string{"/just/a/path"}, "forced")
	assert.EqualError(t, err, `scheme missing in "/just/a/path", please prefix with e.g. file:// or https://`)

	_, err = parseRepoURLs([]string{"https://example.com", "/just/a/path"}, "forced")
	assert.EqualError(t, err, `scheme missing in "/just/a/path", please prefix with e.g. file:// or https://`)
}

func TestNewRepoRegistryImplSmoke(t *testing.T) {
	registry, err := newRepoRegistryImpl("", nil)
	require.NoError(t, err)
	repos, err := registry.DistroHasRepos("rhel-10.2", "x86_64")
	require.NoError(t, err)
	assert.True(t, len(repos) > 0)
}

func TestNewRepoRegistryImplExtraReposGetAppended(t *testing.T) {
	registry, err := newRepoRegistryImpl("", []string{"https://example.com/my/repo"})
	require.NoError(t, err)
	repos, err := registry.DistroHasRepos("rhel-10.2", "x86_64")
	require.NoError(t, err)
	assert.Equal(t, repos[len(repos)-1].BaseURLs[0], "https://example.com/my/repo")
}

func TestNewRepoRegistryImplRepodir(t *testing.T) {
	// prereq test: no testdistro-1 in the default repos
	registry, err := newRepoRegistryImpl("", nil)
	require.NoError(t, err)
	assert.NotContains(t, registry.ListDistros(), "testdistro-1")
	_, err = registry.DistroHasRepos("testdistro-1", "x86_64")
	require.EqualError(t, err, `requested repository not found: for distribution "testdistro-1"`)

	// create a custom repodir with testdistro-1.json, the basefilename
	// must match a distro nameVer
	repoDir := t.TempDir()
	repoFile := filepath.Join(repoDir, "repositories", "testdistro-1.json")
	err = os.Mkdir(filepath.Dir(repoFile), 0755)
	require.NoError(t, err)
	repoContents := `{
	"x86_64": [
		{
			"name": "testdistro-1-repo",
			"baseurl": "https://example.com/test/test/distro/1"
		}
	]
}
`
	err = os.WriteFile(repoFile, []byte(repoContents), 0644)
	require.NoError(t, err)

	// and ensure we have testdistro-1 now
	registry, err = newRepoRegistryImpl(repoDir, nil)
	require.NoError(t, err)
	repos, err := registry.DistroHasRepos("testdistro-1", "x86_64")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, repos[0].Name, "testdistro-1-repo")
}

func TestNewRepoRegistryImplRepodirNoSubDir(t *testing.T) {
	// prereq test: no testdistro-1 in the default repos
	registry, err := newRepoRegistryImpl("", nil)
	require.NoError(t, err)
	assert.NotContains(t, registry.ListDistros(), "testdistro-1")
	_, err = registry.DistroHasRepos("testdistro-1", "x86_64")
	require.EqualError(t, err, `requested repository not found: for distribution "testdistro-1"`)

	// create a custom repodir with testdistro-1.json, the basefilename
	// must match a distro nameVer
	repoDir := t.TempDir()
	repoFile := filepath.Join(repoDir, "testdistro-1.json")
	repoContents := `{
	"x86_64": [
		{
			"name": "testdistro-1-repo",
			"baseurl": "https://example.com/test/test/distro/1"
		}
	]
}
`
	err = os.WriteFile(repoFile, []byte(repoContents), 0644)
	require.NoError(t, err)

	// and ensure we have testdistro-1 now
	registry, err = newRepoRegistryImpl(repoDir, nil)
	require.NoError(t, err)
	repos, err := registry.DistroHasRepos("testdistro-1", "x86_64")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, repos[0].Name, "testdistro-1-repo")
}
