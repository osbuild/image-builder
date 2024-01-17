package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	os.Clearenv()

	config := ImageBuilderConfig{
		ListenAddress: "localhost",
		LogLevel:      "DEBUG",
	}
	err := LoadConfigFromEnv(&config)
	require.NoError(t, err)
	require.Equal(t, config.ListenAddress, "localhost")
	require.Equal(t, config.LogLevel, "DEBUG")
	require.False(t, config.FedoraAuth)
	require.Empty(t, config.LogGroup)
	require.Empty(t, config.CwRegion)
	require.Empty(t, config.CwAccessKeyID)
	require.Empty(t, config.CwSecretAccessKey)
}

func TestEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("LISTEN_ADDRESS", "localhost:8000")
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("FEDORA_AUTH", "true")

	config := ImageBuilderConfig{
		ListenAddress: "localhost",
		LogLevel:      "DEBUG",
	}
	err := LoadConfigFromEnv(&config)
	require.NoError(t, err)
	require.Equal(t, config.ListenAddress, "localhost:8000")
	require.Equal(t, config.LogLevel, "INFO")
	require.True(t, config.FedoraAuth)
	require.Empty(t, config.LogGroup)
	require.Empty(t, config.CwRegion)
	require.Empty(t, config.CwAccessKeyID)
	require.Empty(t, config.CwSecretAccessKey)
}

func TestEnvPointerValues(t *testing.T) {
	os.Clearenv()
	os.Setenv("CW_LOG_GROUP", "somegroup")
	os.Setenv("CW_AWS_REGION", "us-east-1")

	var config ImageBuilderConfig
	err := LoadConfigFromEnv(&config)
	require.NoError(t, err)
	require.Empty(t, config.ListenAddress)
	require.Empty(t, config.LogLevel)
	require.Equal(t, config.LogGroup, "somegroup")
	require.Equal(t, config.CwRegion, "us-east-1")
	require.Empty(t, config.CwAccessKeyID)
	require.Empty(t, config.CwSecretAccessKey)
}
