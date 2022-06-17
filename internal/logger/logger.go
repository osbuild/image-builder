package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/redhatinsights/platform-go-middlewares/logging/cloudwatch"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/syslog"
)

// If CW_AWS_ACCESS_KEY_ID is set in the environment it will assume that the
// logging sink is an aws cloudwatch instance. The following variables have to
// be set as well:
// CW_AWS_SECRET_ACCESS_KEY
// CW_AWS_REGION
// CW_LOG_GROUP

// If CW_AWS_ACCESS_KEY_ID is not set, this logs to stdout with the following format:
// time="$timestamp" level=(debug|info|...) msg="error message" \
// func=$caller file=*.go

var logLevel logrus.Level

type Formatter struct {
	Hostname string
}

// NewCloudwatchFormatter creates a new log formatter
func NewCloudwatchFormatter() *Formatter {
	f := &Formatter{}

	var err error
	if f.Hostname, err = os.Hostname(); err != nil {
		f.Hostname = "unknown"
	}

	return f
}

//Format is the log formatter for the entry
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	now := time.Now()

	hostname, err := os.Hostname()
	if err == nil {
		f.Hostname = hostname
	}

	// Based on https://github.com/RedHatInsights/insights-ingress-go/blob/master/logger/logger.go
	data := map[string]interface{}{
		"@timestamp":  now.Format("2006-01-02T15:04:05.999Z"),
		"@version":    1,
		"message":     entry.Message,
		"levelname":   entry.Level.String(),
		"source_host": f.Hostname,
		"app":         "image-builder",
		"caller":      entry.Caller.Func.Name(),
	}

	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}

	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	b.Write(j)
	b.WriteRune('\n')

	return b.Bytes(), nil
}

func NewLogger(level, key, secret, region, group, syslog_server string) (*logrus.Logger, error) {

	switch level {
	case "DEBUG":
		logLevel = logrus.DebugLevel
	case "ERROR":
		logLevel = logrus.ErrorLevel
	case "INFO":
		fallthrough
	default:
		logLevel = logrus.InfoLevel
	}

	log := logrus.Logger{
		Out:          os.Stdout,
		Level:        logLevel,
		Hooks:        make(logrus.LevelHooks),
		ReportCaller: true,
	}

	if key != "" {
		f := NewCloudwatchFormatter()
		log.SetFormatter(f)
		cred := credentials.NewStaticCredentials(key, secret, "")
		awsconf := aws.NewConfig().WithRegion(region).WithCredentials(cred)
		// avoid the cloudwatch sequence token to get out of sync using the unique hostname per pod
		hook, err := cloudwatch.NewBatchingHook(group, f.Hostname, awsconf, 10*time.Second)
		if err != nil {
			return nil, err
		}
		log.Hooks.Add(hook)
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
		})
	}

	if len(syslog_server) > 0 {
		hook, err := syslog.NewSyslogHook("tcp", syslog_server, 0, "image-builder")
		if err != nil {
			return nil, err
		}

		logrus.AddHook(hook)
	}

	return &log, nil
}
