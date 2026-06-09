package bootc_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/runner"

	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/distro/bootc"
)

func TestGetDistroAndRunner(t *testing.T) {
	cases := []struct {
		id             string
		versionID      string
		expectedDistro manifest.Distro
		expectedRunner runner.Runner
		expectedErr    string
	}{
		// Happy
		{"fedora", "40", manifest.DISTRO_FEDORA, &runner.Fedora{Version: 40}, ""},
		{"centos", "9", manifest.DISTRO_EL9, &runner.CentOS{Version: 9}, ""},
		{"centos", "10", manifest.DISTRO_EL10, &runner.CentOS{Version: 10}, ""},
		{"centos", "11", manifest.DISTRO_NULL, &runner.CentOS{Version: 11}, ""},
		{"ol", "9.7", manifest.DISTRO_EL9, &runner.Ol{Version: 9}, ""},
		{"ol", "10.1", manifest.DISTRO_EL10, &runner.Ol{Version: 10}, ""},
		{"ol", "10.asdf", manifest.DISTRO_EL10, &runner.Ol{Version: 10}, ""},
		{"ol", "10.1.2", manifest.DISTRO_EL10, &runner.Ol{Version: 10}, ""},
		{"ol", "11.1", manifest.DISTRO_NULL, &runner.Ol{Version: 11}, ""},
		{"rhel", "9.4", manifest.DISTRO_EL9, &runner.RHEL{Major: 9, Minor: 4}, ""},
		{"rhel", "10.4", manifest.DISTRO_EL10, &runner.RHEL{Major: 10, Minor: 4}, ""},
		{"rhel", "11.4", manifest.DISTRO_NULL, &runner.RHEL{Major: 11, Minor: 4}, ""},
		{"toucanos", "42", manifest.DISTRO_NULL, &runner.Linux{}, ""},

		// Sad
		{"fedora", "asdf", manifest.DISTRO_NULL, nil, "cannot parse Fedora version (asdf)"},
		{"centos", "asdf", manifest.DISTRO_NULL, nil, "cannot parse CentOS version (asdf)"},
		{"ol", "10", manifest.DISTRO_NULL, nil, "invalid Oracle Linux version format: 10"},
		{"ol", "asdf.1", manifest.DISTRO_NULL, nil, "cannot parse Oracle Linux major version (asdf)"},
		{"rhel", "10", manifest.DISTRO_NULL, nil, "invalid RHEL version format: 10"},
		{"rhel", "10.asdf", manifest.DISTRO_NULL, nil, "cannot parse RHEL minor version (asdf)"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s-%s", c.id, c.versionID), func(t *testing.T) {
			osRelease := osinfo.OSRelease{
				ID:        c.id,
				VersionID: c.versionID,
			}
			distro, runner, err := bootc.GetDistroAndRunner(osRelease)
			if c.expectedErr != "" {
				assert.ErrorContains(t, err, c.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.expectedDistro, distro)
				assert.Equal(t, c.expectedRunner, runner)
			}
		})
	}
}
