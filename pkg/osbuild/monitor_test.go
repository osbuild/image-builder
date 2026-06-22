package osbuild_test

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	_ "embed"

	"github.com/osbuild/image-builder/v73/pkg/osbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/monitor-simple.seq.json
var osbuildMonitorLines_curl []byte

func TestScannerSimple(t *testing.T) {
	ts1 := 1731589338.8252223 * 1000
	ts2 := 1731589338.8256931 * 1000
	ts3 := 1731589407.0338647 * 1000

	scanner := osbuild.NewStatusScanner(bytes.NewBuffer(osbuildMonitorLines_curl))
	// first line
	st, err := scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Trace: "source/org.osbuild.curl (org.osbuild.curl): Downloaded https://rpmrepo.osbuild.org/v2/mirror/public/f39/f39-x86_64-fedora-20231109/Packages/k/kpartx-0.9.5-2.fc39.x86_64.rpm",
		Progress: &osbuild.Progress{
			Done:    0,
			Total:   4,
			Message: "Pipeline source org.osbuild.curl",
		},
		Pipeline:  "source org.osbuild.curl",
		Timestamp: time.UnixMilli(int64(ts1)),
	}, st)
	// second line
	st, err = scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Trace: "source/org.osbuild.curl (org.osbuild.curl): Downloaded https://rpmrepo.osbuild.org/v2/mirror/public/f39/f39-x86_64-fedora-20231109/Packages/l/langpacks-fonts-en-4.0-9.fc39.noarch.rpm",
		Progress: &osbuild.Progress{
			Done:    0,
			Total:   4,
			Message: "Pipeline source org.osbuild.curl",
		},
		Pipeline:  "source org.osbuild.curl",
		Timestamp: time.UnixMilli(int64(ts2)),
	}, st)
	// third line
	st, err = scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Message: "Starting pipeline build",
		Progress: &osbuild.Progress{
			Done:    1,
			Total:   4,
			Message: "Pipeline build",
			SubProgress: &osbuild.Progress{
				Done:  0,
				Total: 2,
			},
		},
		Pipeline:  "build",
		Timestamp: time.UnixMilli(int64(ts3)),
	}, st)
	// end
	st, err = scanner.Status()
	assert.NoError(t, err)
	assert.Nil(t, st)
}

//go:embed testdata/monitor-subprogress.seq.json
var osbuildMontiorLines_subprogress []byte

func TestScannerSubprogress(t *testing.T) {
	ts1 := 1731600115.14839 * 1000

	scanner := osbuild.NewStatusScanner(bytes.NewBuffer(osbuildMontiorLines_subprogress))
	st, err := scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Message: "Starting module org.osbuild.rpm",
		Progress: &osbuild.Progress{
			Done:    1,
			Total:   4,
			Message: "Pipeline build",
			SubProgress: &osbuild.Progress{
				Done:    2,
				Total:   8,
				Message: "Stage org.osbuild.rpm",
				SubProgress: &osbuild.Progress{
					Done:    4,
					Total:   16,
					Message: "Stage org.osbuild.rpm",
				},
			},
		},
		Pipeline:  "build",
		Timestamp: time.UnixMilli(int64(ts1)),
	}, st)
}

func TestScannerSmoke(t *testing.T) {
	f, err := os.Open("../../test/data/osbuild-monitor-output.json")
	require.NoError(t, err)
	defer f.Close()

	scanner := osbuild.NewStatusScanner(f)
	for {
		st, err := scanner.Status()
		assert.NoError(t, err)
		if st == nil {
			break
		}
		assert.NotEqual(t, time.Time{}, st.Timestamp)
	}
}

func TestScannerVeryLongLines(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	fmt.Fprint(buf, `{"message": "`)
	fmt.Fprint(buf, strings.Repeat("1", 16*1024*1024))
	fmt.Fprint(buf, `"}`)

	r := bytes.NewBufferString(buf.String())
	scanner := osbuild.NewStatusScanner(r)
	st, err := scanner.Status()
	assert.NoError(t, err)
	require.NotNil(t, st)
	assert.Equal(t, 16*1024*1024, len(st.Trace))
}

//go:embed testdata/monitor-duration.seq.json
var osbuildMonitorDuration_selinux []byte

func TestScannerDuration(t *testing.T) {
	ts1 := 1757401310.172594 * 1000
	dur1 := 0.5353707351023331

	scanner := osbuild.NewStatusScanner(bytes.NewBuffer(osbuildMonitorDuration_selinux))
	st, err := scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Trace: "Finished module org.osbuild.selinux",
		Progress: &osbuild.Progress{
			Total: 2,
			Done:  1,
			SubProgress: &osbuild.Progress{
				Total: 3,
				Done:  3,
			},
		},
		Timestamp: time.UnixMilli(int64(ts1)),
		Duration:  time.Duration(dur1 * float64(time.Second)),
	}, st)
}

//go:embed testdata/monitor-fedora-42-raw-v197.seq.json
var osbuildMonitor_fedora []byte

func TestScannerPipelineName(t *testing.T) {
	scanner := osbuild.NewStatusScanner(bytes.NewBuffer(osbuildMonitor_fedora))

	// the first line is context-less
	_, err := scanner.Status()
	assert.NoError(t, err)

	var pipelines []string
	for {
		st, err := scanner.Status()
		assert.NoError(t, err)
		if st == nil {
			break
		}

		assert.NotEmpty(t, st.Pipeline)
		pipelines = append(pipelines, st.Pipeline)
	}

	assert.Equal(t, []string{
		"source org.osbuild.inline",
		"source org.osbuild.librepo",
		"build",
	}, slices.Compact(pipelines))

	res, err := scanner.Result()
	assert.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Metadata["build"], "org.osbuild.rpm")
	metadata, ok := res.Metadata["build"]["org.osbuild.rpm"].(*osbuild.RPMStageMetadata)
	assert.True(t, ok)
	assert.Len(t, metadata.Packages, 162)
}

func TestScannerEmpty(t *testing.T) {
	r := bytes.NewBufferString("{}")
	scanner := osbuild.NewStatusScanner(r)
	st, err := scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Progress: &osbuild.Progress{
			Total: 0,
			Done:  0,
		},
	}, st)
}

//go:embed testdata/monitor-validation.json
var osbuildMonitorValidation []byte

func TestScannerValidationFailure(t *testing.T) {
	scanner := osbuild.NewStatusScanner(bytes.NewBuffer(osbuildMonitorValidation))

	st, err := scanner.Status()
	assert.NoError(t, err)
	assert.Equal(t, &osbuild.Status{
		Progress: &osbuild.Progress{
			Total: 0,
			Done:  0,
		},
	}, st)

	res, err := scanner.Result()
	assert.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, &osbuild.Result{
		Type:     "https://osbuild.org/validation-error",
		Success:  false,
		Error:    nil,
		Log:      nil,
		Metadata: nil,
		Errors: []osbuild.ValidationError{
			{
				Message: "{'type': 'org.osbuild.files', 'origin': 'org.osbuild.source', 'somekey': 'bad', 'references': [{}] is not valid under any of the given schemas}",
				Path:    []string{"pipelines", "[0]", "stages", "[0]", "inputs", "packages"},
			},
			{
				Message: "Additional properties are not allowed ('somekey' was unexpected)",
				Path:    []string{"pipelines", "[0]", "stages", "[0]", "inputs", "packages"},
			},
		},
		Title: "JSON Schema validation failed",
	}, res)
}
