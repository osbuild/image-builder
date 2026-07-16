package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGrub2DStage(t *testing.T) {
	opts := &Grub2DStageOptions{
		Path: "tree:///boot/grub2/console.cfg",
		Config: &Grub2DConfig{
			Serial: "serial --unit=0 --speed=115200",
		},
	}
	stage := NewGrub2DStage(opts)
	assert.Equal(t, "org.osbuild.grub2.d", stage.Type)
	assert.Equal(t, opts, stage.Options)
}

func TestNewGrub2DConfigFromGrub2Config(t *testing.T) {
	tests := map[string]struct {
		input    *GRUB2Config
		expected *Grub2DConfig
	}{
		"nil": {
			input:    nil,
			expected: nil,
		},
		"empty": {
			input:    &GRUB2Config{},
			expected: nil,
		},
		"only-unrelated-fields": {
			// GRUB2Config fields that are not console-related
			// should not produce a Grub2DConfig
			input: &GRUB2Config{
				Default:     "saved",
				Distributor: "$(sed 's, release .*$,,g' /etc/system-release)",
				Timeout:     5,
			},
			expected: nil,
		},
		"serial-only": {
			input: &GRUB2Config{
				Serial: "serial --unit=0 --speed=115200",
			},
			expected: &Grub2DConfig{
				Serial: "serial --unit=0 --speed=115200",
			},
		},
		"terminal-input-only": {
			input: &GRUB2Config{
				TerminalInput: []string{"serial", "console"},
			},
			expected: &Grub2DConfig{
				TerminalInput: []string{"serial", "console"},
			},
		},
		"terminal-output-only": {
			input: &GRUB2Config{
				TerminalOutput: []string{"serial"},
			},
			expected: &Grub2DConfig{
				TerminalOutput: []string{"serial"},
			},
		},
		"all-console-fields": {
			input: &GRUB2Config{
				TerminalInput:  []string{"serial", "console"},
				TerminalOutput: []string{"serial", "console"},
				Serial:         "serial --unit=0 --speed=115200",
				// unrelated fields should be ignored
				Default: "saved",
				Timeout: 10,
			},
			expected: &Grub2DConfig{
				TerminalInput:  []string{"serial", "console"},
				TerminalOutput: []string{"serial", "console"},
				Serial:         "serial --unit=0 --speed=115200",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := NewGrub2DConfigFromGrub2Config(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGrub2DStageOptionsJSON(t *testing.T) {
	opts := &Grub2DStageOptions{
		Path: "tree:///boot/grub2/console.cfg",
		Config: &Grub2DConfig{
			TerminalInput:  []string{"serial", "console"},
			TerminalOutput: []string{"serial", "console"},
			Serial:         "serial --unit=0 --speed=115200",
		},
	}

	data, err := json.MarshalIndent(opts, "", "  ")
	require.NoError(t, err)

	expected := `{
  "path": "tree:///boot/grub2/console.cfg",
  "config": {
    "terminal_input": [
      "serial",
      "console"
    ],
    "terminal_output": [
      "serial",
      "console"
    ],
    "serial": "serial --unit=0 --speed=115200"
  }
}`
	assert.Equal(t, expected, string(data))
}

func TestGrub2DStageOptionsJSONOmitEmpty(t *testing.T) {
	// only serial set, terminal fields should be omitted
	opts := &Grub2DStageOptions{
		Path: "tree:///boot/grub2/console.cfg",
		Config: &Grub2DConfig{
			Serial: "serial --unit=0 --speed=9600",
		},
	}

	data, err := json.MarshalIndent(opts, "", "  ")
	require.NoError(t, err)

	expected := `{
  "path": "tree:///boot/grub2/console.cfg",
  "config": {
    "serial": "serial --unit=0 --speed=9600"
  }
}`
	assert.Equal(t, expected, string(data))
}
