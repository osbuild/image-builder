package util_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder-cli/pkg/util"
)

func TestOutputErrPassthrough(t *testing.T) {
	err := fmt.Errorf("boom")
	assert.Equal(t, util.OutputErr(err), err)
}

func TestOutputErrExecError(t *testing.T) {
	_, err := exec.CommandContext(t.Context(), "bash", "-c", ">&2 echo some-stderr; exit 1").Output()
	assert.Equal(t, "exit status 1, stderr:\nsome-stderr\n", util.OutputErr(err).Error())
}

func TestShortenString(t *testing.T) {
	for _, tc := range []struct {
		input    string
		length   int
		expected string
	}{
		{"", 0, ""},
		{"", 1, ""},
		{"short", 10, "short"},
		{"exactlyten", 10, "exactlyten"},
		{"12345678901234", 10, "123456789…"}, // returns 10 not 11 chars
		{"new\nline", 10, "new\nline"},
		{"new\nline that is way too long", 10, "new\nline …"},
		{"xx", 1, "…"},
		{"xx", 0, "xx"},
	} {
		t.Run(fmt.Sprintf("%q/%d", tc.input, tc.length), func(t *testing.T) {
			assert.Equal(t, tc.expected, util.ShortenString(tc.input, tc.length))
		})
	}
}
