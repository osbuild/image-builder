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

	userData, err := createUserData("test:user", publicKeyFile)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(userData, "#cloud-config\n"))

	var config cloudConfig
	require.NoError(t, yaml.Unmarshal([]byte(userData), &config))
	require.Equal(t, "test:user", config.User)
	require.Equal(t, []string{publicKey}, config.SSHAuthorizedKeys)
}
