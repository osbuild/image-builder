package logger

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNewLoggerDEBUG(t *testing.T) {
	result, err := NewLogger("DEBUG", nil, nil, nil, nil)

	require.NoError(t, err)
	require.Equal(t, logrus.DebugLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerERROR(t *testing.T) {
	result, err := NewLogger("ERROR", nil, nil, nil, nil)

	require.NoError(t, err)
	require.Equal(t, logrus.ErrorLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerINFO(t *testing.T) {
	result, err := NewLogger("INFO", nil, nil, nil, nil)

	require.NoError(t, err)
	require.Equal(t, logrus.InfoLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerUnknownLevel(t *testing.T) {
	result, err := NewLogger("DummyLevel", nil, nil, nil, nil)

	require.NoError(t, err)
	require.Equal(t, logrus.InfoLevel, result.Level)
	require.IsType(t, &logrus.TextFormatter{}, result.Formatter)
}

func TestNewLoggerWithInvalidKeyAndSecret(t *testing.T) {
	myKey := "testKey"
	mySecret := "testSecret"
	myRegion := "us-east-1"
	myGroup := "image-builder"
	result, err := NewLogger("DEBUG", &myKey, &mySecret, &myRegion, &myGroup)

	require.Nil(t, result)
	require.Error(t, err, "The security token included in the request is invalid")
}
