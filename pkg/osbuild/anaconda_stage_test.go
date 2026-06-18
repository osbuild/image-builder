package osbuild_test

import (
	"testing"

	"github.com/osbuild/image-builder/pkg/customizations/anaconda"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/stretchr/testify/require"
)

func TestAnacondaStageOptions(t *testing.T) {

	type testCase struct {
		enable   []string
		disable  []string
		expected []string
	}

	testCases := map[string]testCase{
		"empty-args": {
			expected: []string{},
		},
		"enabled-module": {
			enable: []string{
				anaconda.ModuleUsers,
			},
			expected: []string{
				anaconda.ModuleUsers,
			},
		},
		"multi-enabled-module": {
			enable: []string{
				anaconda.ModuleSubscription,
				anaconda.ModuleTimezone,
				anaconda.ModuleUsers,
			},
			expected: []string{
				anaconda.ModuleSubscription,
				anaconda.ModuleTimezone,
				anaconda.ModuleUsers,
			},
		},
		"add-non-constant": {
			enable: []string{
				"org.osbuild.not.anaconda.module",
			},
			expected: []string{
				"org.osbuild.not.anaconda.module",
			},
		},
		"no-op-disable": {
			disable: []string{
				anaconda.ModuleUsers,
			},
			expected: []string{},
		},
		"enable-then-disable": {
			enable: []string{
				anaconda.ModuleServices,
			},
			disable: []string{
				anaconda.ModuleServices,
			},
			expected: []string{},
		},
		"enable-then-disable-nonsense": {
			enable: []string{
				"org.osbuild.not.anaconda.module.2",
			},
			disable: []string{
				"org.osbuild.not.anaconda.module.2",
			},
			expected: []string{},
		},
		"enable-then-disable-multi": {
			enable: []string{
				anaconda.ModuleSubscription,
				anaconda.ModuleTimezone,
				anaconda.ModuleUsers,
			},
			disable: []string{
				anaconda.ModuleSubscription,
				anaconda.ModuleTimezone,
				anaconda.ModuleUsers,
			},
			expected: []string{},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			options := osbuild.NewAnacondaStageOptionsLegacy(tc.enable, tc.disable)

			require.NotNil(options)
			require.ElementsMatch(options.KickstartModules, tc.expected)
		})
	}

}
