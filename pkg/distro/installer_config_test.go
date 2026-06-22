package distro_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/distro"
)

func TestInstallerConfigInheritFrom(t *testing.T) {
	tests := []struct {
		name           string
		parentConfig   *distro.InstallerConfig
		childConfig    *distro.InstallerConfig
		expectedConfig *distro.InstallerConfig
	}{
		{
			name: "inheritance does not override/change child",
			parentConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-parent"},
			},
			childConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-child"},
			},
			expectedConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-child"},
			},
		},
		{
			name: "inheritance add unset parent keys",
			parentConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-parent"},
				AdditionalDrivers:       []string{"parent-drv"},
			},
			childConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-child"},
			},
			expectedConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-child"},
				AdditionalDrivers:       []string{"parent-drv"},
			},
		},
		{
			name: "inheritance fully merges",
			parentConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-parent"},
				AdditionalDrivers:       []string{"parent-drv"},
			},
			childConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-child"},
				EnabledAnacondaModules:  []string{"child-ana"},
			},
			expectedConfig: &distro.InstallerConfig{
				AdditionalDracutModules: []string{"mod-child"},
				AdditionalDrivers:       []string{"parent-drv"},
				EnabledAnacondaModules:  []string{"child-ana"},
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expectedConfig, tt.childConfig.InheritFrom(tt.parentConfig), "test case %q failed (idx %d)", tt.name, idx)
		})
	}
}
