package check_test

import (
	"strings"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/buildconfig"
	"github.com/osbuild/images/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// joinArgs is a test helper function that joins a name and a list of arguments into a single string
func joinArgs(name string, arg ...string) string {
	if len(arg) == 0 {
		return name
	}
	return name + " " + strings.Join(arg, " ")
}

// buildConfig is a test helper function that builds a buildconfig.BuildConfig with a given customizations
func buildConfig(customizations *blueprint.Customizations) *buildconfig.BuildConfig {
	return &buildconfig.BuildConfig{
		Blueprint: &blueprint.Blueprint{
			Customizations: customizations,
		},
	}
}

// buildConfigWithBlueprint is a test helper function that builds a buildconfig.BuildConfig with a Blueprint builder function
func buildConfigWithBlueprint(fn func(*blueprint.Blueprint)) *buildconfig.BuildConfig {
	bp := &blueprint.Blueprint{}
	fn(bp)
	return &buildconfig.BuildConfig{
		Blueprint: bp,
	}
}

func TestParseOSRelease(t *testing.T) {
	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})
	release, err := check.ParseOSRelease("/etc/os-release")
	require.NoError(t, err)

	assert.Equal(t, "rhel", release.ID)
	assert.Equal(t, "9.0", release.VersionID)
	assert.Equal(t, "9.0 (Plow)", release.Version)
	assert.Equal(t, 9, release.MajorVersion)
}

func TestGetDatastreamFilename(t *testing.T) {
	tests := []struct {
		name     string
		release  *check.OSRelease
		expected string
		wantErr  bool
	}{
		{
			name: "RHEL 9",
			release: &check.OSRelease{
				ID:           "rhel",
				VersionID:    "9.0",
				MajorVersion: 9,
			},
			expected: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
		{
			name: "RHEL 8",
			release: &check.OSRelease{
				ID:           "rhel",
				VersionID:    "8.6",
				MajorVersion: 8,
			},
			expected: "/usr/share/xml/scap/ssg/content/ssg-rhel8-ds.xml",
		},
		{
			name: "CentOS 9",
			release: &check.OSRelease{
				ID:           "centos",
				VersionID:    "9",
				MajorVersion: 9,
			},
			expected: "/usr/share/xml/scap/ssg/content/ssg-cs9-ds.xml",
		},
		{
			name: "CentOS 8",
			release: &check.OSRelease{
				ID:           "centos",
				VersionID:    "8.5",
				MajorVersion: 8,
			},
			expected: "/usr/share/xml/scap/ssg/content/ssg-centos8-ds.xml",
		},
		{
			name: "Fedora",
			release: &check.OSRelease{
				ID:           "fedora",
				VersionID:    "39",
				MajorVersion: 39,
			},
			expected: "/usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml",
		},
		{
			name: "Unsupported OS",
			release: &check.OSRelease{
				ID:           "ubuntu",
				VersionID:    "22.04",
				MajorVersion: 22,
			},
			wantErr: true,
		},
		{
			name: "Unsupported version",
			release: &check.OSRelease{
				ID:           "rhel",
				VersionID:    "7",
				MajorVersion: 7,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename, err := check.GetDatastreamFilename(tt.release)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, filename)
		})
	}
}
