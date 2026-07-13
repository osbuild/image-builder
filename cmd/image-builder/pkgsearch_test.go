package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/rpmmd"

	main "github.com/osbuild/image-builder/cmd/image-builder"
)

func TestNewPkgSearchFormatter(t *testing.T) {
	for _, tc := range []struct {
		name        string
		format      string
		expectedErr string
	}{
		{
			name:   "json",
			format: "json",
		},
		{
			name:   "empty defaults to json",
			format: "",
		},
		{
			name:        "unsupported format",
			format:      "text",
			expectedErr: "unsupported",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fmter, err := main.NewPkgSearchFormatter(tc.format)
			if tc.expectedErr != "" {
				assert.Nil(t, fmter)
				assert.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, fmter)
		})
	}
}

func TestPkgSearchFormatterJSONOutput(t *testing.T) {
	for _, tc := range []struct {
		name         string
		pkgs         rpmmd.PackageList
		expectedPkgs int
		expectedName string
	}{
		{
			name: "multiple packages",
			pkgs: rpmmd.PackageList{
				{Name: "bash", Version: "5.1.8", Release: "2.el9", Arch: "x86_64"},
				{Name: "zsh", Version: "5.8", Release: "7.el9", Arch: "x86_64"},
			},
			expectedPkgs: 2,
			expectedName: "bash",
		},
		{
			name:         "empty package list",
			pkgs:         rpmmd.PackageList{},
			expectedPkgs: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fmter, err := main.NewPkgSearchFormatter("json")
			require.NoError(t, err)

			var buf bytes.Buffer
			err = fmter.Output(&buf, tc.pkgs)
			require.NoError(t, err)

			var result struct {
				Packages []struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Release string `json:"release"`
					Arch    string `json:"arch"`
				} `json:"packages"`
			}
			err = json.Unmarshal(buf.Bytes(), &result)
			require.NoError(t, err)
			assert.Len(t, result.Packages, tc.expectedPkgs)

			if tc.expectedName != "" {
				assert.Equal(t, tc.expectedName, result.Packages[0].Name)
			}
		})
	}
}
