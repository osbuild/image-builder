package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	os.Clearenv()

	config := ImageBuilderConfig{
		ListenAddress: "localhost",
		LogLevel: "DEBUG",
	}
	err := LoadConfigFromEnv(&config)
	require.NoError(t, err)
	require.Equal(t, config.ListenAddress, "localhost")
	require.Equal(t, config.LogLevel, "DEBUG")
	require.Nil(t, config.LogGroup)
	require.Nil(t, config.Region)
	require.Nil(t, config.AccessKeyID)
	require.Nil(t, config.SecretAccessKey)
}

func TestEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("LISTEN_ADDRESS", "localhost:8000")
	os.Setenv("LOG_LEVEL", "INFO")

	config := ImageBuilderConfig{
		ListenAddress: "localhost",
		LogLevel: "DEBUG",
	}
	err := LoadConfigFromEnv(&config)
	require.NoError(t, err)
	require.Equal(t, config.ListenAddress, "localhost:8000")
	require.Equal(t, config.LogLevel, "INFO")
	require.Nil(t, config.LogGroup)
	require.Nil(t, config.Region)
	require.Nil(t, config.AccessKeyID)
	require.Nil(t, config.SecretAccessKey)
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
	require.Equal(t, *config.LogGroup, "somegroup")
	require.Equal(t, *config.Region, "us-east-1")
	require.Nil(t, config.AccessKeyID)
	require.Nil(t, config.SecretAccessKey)
}

func TestErrors(t *testing.T) {
	os.Clearenv()
	os.Setenv("BAD_TYPE", "1000")

	config := struct {
		BadType int `env:"BAD_TYPE"`
	}{}
	err := LoadConfigFromEnv(&config)
	require.Error(t, err, "Unsupported type")

	config2 := struct {
		BadType *int `env:"BAD_TYPE"`
	}{}
	err = LoadConfigFromEnv(&config2)
	require.Error(t, err, "Unsupported type")
}
