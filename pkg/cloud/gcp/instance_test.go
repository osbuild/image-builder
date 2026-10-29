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

func TestFormatSSHKeysMetadata(t *testing.T) {
	val, err := FormatSSHKeysMetadata("fedora", "ssh-rsa AAAAB3")
	require.NoError(t, err)
	require.Equal(t, "fedora:ssh-rsa AAAAB3", val)

	val, err = FormatSSHKeysMetadata("fedora", "fedora:ssh-rsa AAAAB3")
	require.NoError(t, err)
	require.Equal(t, "fedora:ssh-rsa AAAAB3", val)

	_, err = FormatSSHKeysMetadata("", "ssh-rsa AAAAB3")
	require.Error(t, err)
}
