package experimentalflags_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/experimentalflags"
)

func TestExperimentalBool(t *testing.T) {
	for _, tc := range []struct {
		envStr   string
		expected bool
	}{
		{"", false},
		{"skip-arch-checks=0", false},
		{"skip-arch-checks=f", false},
		{"skip-arch-checks=false", false},
		{"skip-arch-checks=1", true},
		{"skip-arch-checks=true", true},
		{"skip-arch-checks=t", true},
		{"skip-arch-checks", true},
		{"unrelated,skip-arch-checks=1", true},
	} {
		t.Run(tc.envStr, func(t *testing.T) {
			t.Setenv("IMAGE_BUILDER_EXPERIMENTAL", tc.envStr)

			assert.Equal(t, tc.expected, experimentalflags.Bool("skip-arch-checks"))
		})
	}

}

func TestExperimentalString(t *testing.T) {
	for _, tc := range []struct {
		envStr   string
		expected string
	}{
		{"", ""},
		{"unrelated", ""},
		{"unrelated=stropt", ""},
		{"stropt=val", "val"},
		{"unrelated,stropt=val", "val"},
		{"stropt=val=val", "val=val"},
	} {
		t.Run(tc.envStr, func(t *testing.T) {
			t.Setenv("IMAGE_BUILDER_EXPERIMENTAL", tc.envStr)

			assert.Equal(t, tc.expected, experimentalflags.String("stropt"))
		})
	}

}
