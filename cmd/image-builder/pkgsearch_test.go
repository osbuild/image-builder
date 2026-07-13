package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	testrepos "github.com/osbuild/image-builder/test/data/repositories"

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

var lastCapturedCacheDir string

func fakePkgSearcher(_ distro.Distro, _, cacheDir string, _ []rpmmd.RepoConfig, _ []string) (rpmmd.PackageList, error) {
	lastCapturedCacheDir = cacheDir
	return rpmmd.PackageList{
		{Name: "bash", Version: "5.1.8", Release: "2.el9", Arch: "x86_64"},
		{Name: "zsh", Version: "5.8", Release: "7.el9", Arch: "x86_64"},
	}, nil
}

func TestCmdPkgSearch(t *testing.T) {
	restorePkgSearcher := main.MockPkgSearcher(fakePkgSearcher)
	defer restorePkgSearcher()

	restoreRepoRegistry := main.MockNewRepoRegistry(testrepos.New)
	defer restoreRepoRegistry()

	for _, tc := range []struct {
		name             string
		args             []string
		expectedPkgs     int
		expectedErr      string
		expectedCacheDir string
		mockHostDistro   string
	}{
		{
			name:         "with type",
			args:         []string{"pkgsearch", "bash", "--distro=centos-9", "--arch=x86_64", "--type=qcow2"},
			expectedPkgs: 2,
		},
		{
			name:         "without type",
			args:         []string{"pkgsearch", "bash", "--distro=centos-9", "--arch=x86_64"},
			expectedPkgs: 2,
		},
		{
			name:         "with json format",
			args:         []string{"pkgsearch", "bash", "--distro=centos-9", "--arch=x86_64", "--format=json"},
			expectedPkgs: 2,
		},
		{
			name:         "multiple packages",
			args:         []string{"pkgsearch", "bash", "zsh", "--distro=centos-9", "--arch=x86_64"},
			expectedPkgs: 2,
		},
		{
			name:        "unsupported format",
			args:        []string{"pkgsearch", "bash", "--distro=centos-9", "--arch=x86_64", "--format=text"},
			expectedErr: "unsupported",
		},
		{
			name:        "invalid distro",
			args:        []string{"pkgsearch", "bash", "--distro=not-a-distro", "--arch=x86_64"},
			expectedErr: "not-a-distro",
		},
		{
			name:        "invalid type",
			args:        []string{"pkgsearch", "bash", "--distro=centos-9", "--arch=x86_64", "--type=not-a-type"},
			expectedErr: "not-a-type",
		},
		{
			name:         "zero args (no packages)",
			args:         []string{"pkgsearch", "--distro=centos-9", "--arch=x86_64"},
			expectedPkgs: 2,
		},
		{
			name:             "with rpmmd-cache",
			args:             []string{"pkgsearch", "bash", "--distro=centos-9", "--arch=x86_64", "--rpmmd-cache=/tmp/test-cache"},
			expectedPkgs:     2,
			expectedCacheDir: "/tmp/test-cache",
		},
		{
			name:           "default distro from host",
			args:           []string{"pkgsearch", "bash", "--arch=x86_64"},
			expectedPkgs:   2,
			mockHostDistro: "centos-9",
		},
		{
			name:         "default arch from host",
			args:         []string{"pkgsearch", "bash", "--distro=centos-9"},
			expectedPkgs: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockHostDistro != "" {
				restore := main.MockDistroGetHostDistroName(func() (string, error) {
					return tc.mockHostDistro, nil
				})
				defer restore()
			}

			restore := main.MockOsArgs(tc.args)
			defer restore()

			var fakeStdout bytes.Buffer
			restore = main.MockOsStdout(&fakeStdout)
			defer restore()

			err := main.Run()
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErr)
				return
			}

			require.NoError(t, err)

			var result struct {
				Packages []struct {
					Name string `json:"name"`
				} `json:"packages"`
			}
			err = json.Unmarshal(fakeStdout.Bytes(), &result)
			require.NoError(t, err)
			assert.Len(t, result.Packages, tc.expectedPkgs)

			if tc.expectedCacheDir != "" {
				assert.Equal(t, tc.expectedCacheDir, lastCapturedCacheDir)
			}
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
