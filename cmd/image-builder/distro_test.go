package main_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func TestFindDistro(t *testing.T) {
	for _, tc := range []struct {
		argDistro      string
		bpDistro       string
		expectedDistro string
		expectedErr    string
	}{
		{"arg", "", "arg", ""},
		{"", "bp", "bp", ""},
		{"arg", "bp", "", `error selecting distro name, cmdline argument "arg" is different from blueprint "bp"`},
		// the argDistro,bpDistro == "" case is tested below
	} {
		distro, err := main.FindDistro(tc.argDistro, tc.bpDistro)
		if tc.expectedErr != "" {
			assert.Equal(t, tc.expectedErr, err.Error())
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedDistro, distro)
		}
	}
}

func TestFindDistroAutoDetect(t *testing.T) {
	var buf bytes.Buffer
	restore := main.MockOsStderr(&buf)
	defer restore()

	restore = main.MockDistroGetHostDistroName(func() (string, error) {
		return "mocked-host-distro", nil
	})
	defer restore()

	distro, err := main.FindDistro("", "")
	assert.NoError(t, err)
	assert.Equal(t, "mocked-host-distro", distro)
	assert.Equal(t, "No distro name specified, selecting \"mocked-host-distro\" based on host, use --distro to override\n", buf.String())
}
