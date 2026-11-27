package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultMachineType(t *testing.T) {
	mt, err := DefaultMachineType("x86_64")
	require.NoError(t, err)
	require.Equal(t, "e2-medium", mt)

	mt, err = DefaultMachineType("aarch64")
	require.NoError(t, err)
	require.Equal(t, "t2a-standard-1", mt)

	_, err = DefaultMachineType("s390x")
	require.Error(t, err)
}

func TestLocalUserStartupScript(t *testing.T) {
	val, err := LocalUserStartupScript("fedora", "ssh-rsa AAAAB3")
	require.NoError(t, err)
	require.Contains(t, val, "useradd --create-home --shell /bin/bash fedora")
	require.Contains(t, val, "c3NoLXJzYSBBQUFBQjM=")

	_, err = LocalUserStartupScript("", "ssh-rsa AAAAB3")
	require.Error(t, err)
}
