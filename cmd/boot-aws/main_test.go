package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
}

func TestCreateUserData(t *testing.T) {
	tmpdir := t.TempDir()
	publicKeyFile := filepath.Join(tmpdir, "id.pub")
	publicKey := "ssh-ed25519 AAAA:test/key user:comment"
	writeTestFile(t, publicKeyFile, publicKey+"\n")

	repoFile1 := filepath.Join(tmpdir, "baseos.repo")
	repoContent1 := "[baseos]\nname=BaseOS: test\nbaseurl=https://example.com/$basearch/os\nenabled=1\n"
	writeTestFile(t, repoFile1, repoContent1)
	repoFile2 := filepath.Join(tmpdir, "appstream.repo")
	repoContent2 := "[appstream]\nbaseurl=https://example.com/appstream\ngpgcheck=0\n"
	writeTestFile(t, repoFile2, repoContent2)

	userData, err := createUserData("test:user", publicKeyFile, []string{repoFile1, repoFile2})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(userData, "#cloud-config\n"))

	var config cloudConfig
	require.NoError(t, yaml.Unmarshal([]byte(userData), &config))
	require.Equal(t, "test:user", config.User)
	require.Equal(t, []string{publicKey}, config.SSHAuthorizedKeys)
	require.Equal(t, []cloudConfigFile{
		{
			Path:        "/etc/yum.repos.d/baseos.repo",
			Content:     repoContent1,
			Owner:       "root:root",
			Permissions: "0644",
		},
		{
			Path:        "/etc/yum.repos.d/appstream.repo",
			Content:     repoContent2,
			Owner:       "root:root",
			Permissions: "0644",
		},
	}, config.WriteFiles)
}

func TestCreateUserDataWithoutRepositories(t *testing.T) {
	publicKeyFile := filepath.Join(t.TempDir(), "id.pub")
	writeTestFile(t, publicKeyFile, "ssh-ed25519 AAAA test\n")

	userData, err := createUserData("test", publicKeyFile, nil)
	require.NoError(t, err)

	var config cloudConfig
	require.NoError(t, yaml.Unmarshal([]byte(userData), &config))
	require.Empty(t, config.WriteFiles)
	require.NotContains(t, userData, "write_files")
}

func TestCreateUserDataRejectsUnreadableRepositoryFile(t *testing.T) {
	tmpdir := t.TempDir()
	publicKeyFile := filepath.Join(tmpdir, "id.pub")
	writeTestFile(t, publicKeyFile, "ssh-ed25519 AAAA test\n")
	repoFile := filepath.Join(tmpdir, "missing.repo")

	_, err := createUserData("test", publicKeyFile, []string{repoFile})
	require.ErrorContains(t, err, "cannot read repository file")
}
