package logger

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNewLoggerDEBUG(t *testing.T) {
	result, err := NewLogger("DEBUG", "", "", "", "")

	require.NoError(t, err)
	require.Equal(t, logrus.DebugLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerERROR(t *testing.T) {
	result, err := NewLogger("ERROR", "", "", "", "")

	require.NoError(t, err)
	require.Equal(t, logrus.ErrorLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerINFO(t *testing.T) {
	result, err := NewLogger("INFO", "", "", "", "")

	require.NoError(t, err)
	require.Equal(t, logrus.InfoLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerUnknownLevel(t *testing.T) {
	result, err := NewLogger("DummyLevel", "", "", "", "")

	require.NoError(t, err)
	require.Equal(t, logrus.InfoLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerWithInvalidKeyAndSecret(t *testing.T) {
	result, err := NewLogger("DEBUG", "testKey", "testSecret", "us-east-1", "image-builder")

	require.Nil(t, result)
	require.Error(t, err, "The security token included in the request is invalid")
}
