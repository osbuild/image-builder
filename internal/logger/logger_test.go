package logger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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

func TestSplunkLogger(t *testing.T) {
	ch := make(chan bool)
	time.AfterFunc(time.Second*10, func() {
		ch <- false
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Splunk", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var sp SplunkPayload
		err := json.NewDecoder(r.Body).Decode(&sp)
		require.NoError(t, err)
		require.Equal(t, "test-host", sp.Host)
		require.Equal(t, "test-host", sp.Event.Host)
		require.Equal(t, "image-builder", sp.Event.Ident)
		require.Equal(t, "message", sp.Event.Message)
		ch <- true
	}))
	sl := NewSplunkLogger(srv.URL, "", "image-builder", "test-host")
	sl.LogWithTime(time.Now(), "message")
	require.True(t, <-ch)
}
