package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestResourcesRegionRoundTrip(t *testing.T) {
	region := "us-east-1"
	securityGroup := "sg-123"
	instance := "i-123"
	written := resources{
		Region:        region,
		SecurityGroup: &securityGroup,
		InstanceID:    &instance,
	}

	data, err := json.Marshal(&written)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"region": "us-east-1",
		"security-group": "sg-123",
		"instance": "i-123"
	}`, string(data))

	var read resources
	require.NoError(t, json.Unmarshal(data, &read))
	require.Equal(t, written, read)
}

func TestRequiredFlagsBySubcommand(t *testing.T) {
	testCases := map[string][]string{
		"setup":    {"ami", "arch", "region", "ssh-pubkey", "username"},
		"run":      {"ami", "arch", "region", "ssh-privkey", "ssh-pubkey", "username"},
		"teardown": {},
	}

	for subcommand, expected := range testCases {
		t.Run(subcommand, func(t *testing.T) {
			root := setupCLI()
			cmd, _, err := root.Find([]string{subcommand})
			require.NoError(t, err)

			required := []string{}
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				if values := flag.Annotations[cobra.BashCompOneRequiredFlag]; len(values) > 0 && values[0] == "true" {
					required = append(required, flag.Name)
				}
			})
			sort.Strings(required)
			require.Equal(t, expected, required)
		})
	}
}

func TestNetworkFlagsRequiredTogether(t *testing.T) {
	for _, subcommand := range []string{"setup", "run"} {
		t.Run(subcommand, func(t *testing.T) {
			testCases := []struct {
				name      string
				vpcID     string
				subnetID  string
				expectErr bool
			}{
				{name: "neither flag"},
				{name: "both flags", vpcID: "vpc-123", subnetID: "subnet-123"},
				{name: "VPC only", vpcID: "vpc-123", expectErr: true},
				{name: "subnet only", subnetID: "subnet-123", expectErr: true},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					root := setupCLI()
					cmd, _, err := root.Find([]string{subcommand})
					require.NoError(t, err)

					if tc.vpcID != "" {
						require.NoError(t, cmd.Flags().Set("vpc-id", tc.vpcID))
					}
					if tc.subnetID != "" {
						require.NoError(t, cmd.Flags().Set("subnet-id", tc.subnetID))
					}

					err = cmd.ValidateFlagGroups()
					if tc.expectErr {
						require.ErrorContains(t, err, "they must all be set")
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}
