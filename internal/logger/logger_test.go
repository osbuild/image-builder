package logger

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func CreateLogger() *logrus.Logger {
	return &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
}

func TestNewLoggerDEBUG(t *testing.T) {
	log := CreateLogger()
	err := ConfigLogger(log, "DEBUG", "")

	require.NoError(t, err)
	require.Equal(t, logrus.DebugLevel, log.Level)
	require.IsType(t, &logrus.TextFormatter{}, log.Formatter)
}

func TestNewLoggerERROR(t *testing.T) {
	log := CreateLogger()
	err := ConfigLogger(log, "ERROR", "")

	require.NoError(t, err)
	require.Equal(t, logrus.ErrorLevel, log.Level)
	require.IsType(t, &logrus.TextFormatter{}, log.Formatter)
}

func TestNewLoggerINFO(t *testing.T) {
	log := CreateLogger()
	err := ConfigLogger(log, "INFO", "")

	require.NoError(t, err)
	require.Equal(t, logrus.InfoLevel, log.Level)
	require.IsType(t, &logrus.TextFormatter{}, log.Formatter)
}

func TestNewLoggerUnknownLevel(t *testing.T) {
	log := CreateLogger()
	err := ConfigLogger(log, "DummyLevel", "")

	require.NoError(t, err)
	require.Equal(t, logrus.InfoLevel, log.Level)
	require.IsType(t, &logrus.TextFormatter{}, log.Formatter)
}

func TestNewLoggerWithInvalidKeyAndSecret(t *testing.T) {
	log := CreateLogger()
	err := ConfigLogger(log, "DEBUG", "")
	require.NoError(t, err)
	err = AddCloudWatchHook(log, "testSecret", "us-east-1", "image-builder", "")

	require.Error(t, err, "The security token included in the request is invalid")
}
