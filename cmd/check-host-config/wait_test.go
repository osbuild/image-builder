package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/test"
)

func joinArgs(name string, arg ...string) string {
	if len(arg) == 0 {
		return name
	}
	return name + " " + strings.Join(arg, " ")
}

func TestRunningWait(t *testing.T) {
	responses := make(chan []byte, 2)
	responses <- []byte("starting\n")
	responses <- []byte("running\n")

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		return <-responses, nil, 0, nil // reading 3rd time will block and fail test
	})

	// XXX: use synctest.Run to speed it up after Go 1.24+ upgrade
	if err := runningWait(1*time.Second, 20*time.Millisecond); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestGetActivatingUnits(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "single unit",
			output:   "foo.service loaded activating auto vendor preset: enabled\n",
			expected: "foo.service",
		},
		{
			name: "multiple units",
			output: `foo.service loaded activating auto vendor preset: enabled
bar.service loaded activating auto vendor preset: enabled
baz.service loaded activating auto vendor preset: enabled
`,
			expected: "foo.service bar.service baz.service",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "whitespace only",
			output:   "   \n  \n",
			expected: "",
		},
		{
			name:     "unit with spaces in name",
			output:   "foo-bar.service loaded activating auto vendor preset: enabled\n",
			expected: "foo-bar.service",
		},
		{
			name: "mixed with empty lines",
			output: `foo.service loaded activating auto vendor preset: enabled

bar.service loaded activating auto vendor preset: enabled
`,
			expected: "foo.service bar.service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
				if name == "systemctl" {
					return []byte(tt.output), nil, 0, nil
				}
				return nil, nil, 0, nil
			})

			result := strings.Join(listBadUnits(), " ")
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPrintUnitJournal(t *testing.T) {
	const unit = "foo.service"
	const journalOut = "line 1\nline 2\n"

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		if joinArgs(name, arg...) != joinArgs("journalctl", "-u", unit, "-o", "cat") {
			return nil, nil, 1, nil
		}
		return []byte(journalOut), nil, 0, nil
	})

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	printUnitJournal(unit)
	w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	want := journalOut + "\n" // Fprintf uses "%s\n"
	if got := buf.String(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
