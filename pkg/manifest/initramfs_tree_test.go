package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestInitramfsServices(t *testing.T) {
	for name, options := range map[string]osbuild.SystemdStageOptions{
		"without services": {},
		"enabled":          {EnabledServices: []string{"systemd-sysext-sysroot.service"}},
		"disabled":         {DisabledServices: []string{"systemd-homed.service"}},
		"masked":           {MaskedServices: []string{"initrd-cleanup.service"}},
		"combined": {
			EnabledServices:  []string{"systemd-sysext-sysroot.service"},
			DisabledServices: []string{"systemd-homed.service"},
			MaskedServices:   []string{"initrd-cleanup.service"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := manifest.New()
			build := manifest.NewBuild(&m, &runner.Linux{}, nil, nil)
			tree := manifest.NewInitramfsTree(build, &platform.Data{}, nil)
			tree.Customizations.EnabledServices = options.EnabledServices
			tree.Customizations.DisabledServices = options.DisabledServices
			tree.Customizations.MaskedServices = options.MaskedServices
			configured := len(options.EnabledServices)+len(options.DisabledServices)+len(options.MaskedServices) > 0

			pipeline, err := manifest.SerializeWith(tree, manifest.Inputs{})
			require.NoError(t, err)
			if configured {
				last := pipeline.Stages[len(pipeline.Stages)-1]
				assert.Equal(t, "org.osbuild.systemd", last.Type)
				assert.Equal(t, &options, last.Options)
			} else {
				for _, stage := range pipeline.Stages {
					assert.NotEqual(t, "org.osbuild.systemd", stage.Type)
				}
			}

			chains, err := m.GetPackageSetChains()
			require.NoError(t, err)
			require.NotEmpty(t, chains[build.Name()])
			if configured {
				assert.Contains(t, chains[build.Name()][0].Include, "systemd")
			}
		})
	}
}
