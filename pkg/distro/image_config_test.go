package distro

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func TestImageConfigInheritFrom(t *testing.T) {
	tests := []struct {
		name           string
		distroConfig   *ImageConfig
		imageConfig    *ImageConfig
		expectedConfig *ImageConfig
	}{
		{
			name: "inheritance with overridden values",
			distroConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			imageConfig: &ImageConfig{
				Timezone: common.ToPtr("UTC"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{
						{
							Hostname: "169.254.169.123",
							Prefer:   common.ToPtr(true),
							Iburst:   common.ToPtr(true),
							Minpoll:  common.ToPtr(4),
							Maxpoll:  common.ToPtr(4),
						},
					},
					LeapsecTz: common.ToPtr(""),
				},
			},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("UTC"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{
						{
							Hostname: "169.254.169.123",
							Prefer:   common.ToPtr(true),
							Iburst:   common.ToPtr(true),
							Minpoll:  common.ToPtr(4),
							Maxpoll:  common.ToPtr(4),
						},
					},
					LeapsecTz: common.ToPtr(""),
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		},
		{
			name: "empty image type configuration",
			distroConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			imageConfig: &ImageConfig{},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		},
		{
			name:         "empty distro configuration",
			distroConfig: &ImageConfig{},
			imageConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		},
		{
			name:         "empty distro configuration",
			distroConfig: nil,
			imageConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		}, {
			name: "inheritance with nil imageConfig",
			distroConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
			},
			imageConfig: nil,
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
			},
		}, {
			name:           "inheritance with nil imageConfig and distroConfig",
			distroConfig:   nil,
			imageConfig:    nil,
			expectedConfig: &ImageConfig{},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expectedConfig, tt.imageConfig.InheritFrom(tt.distroConfig), "test case %q failed (idx %d)", tt.name, idx)
		})
	}
}

func TestImageConfigDNFSetReleaseverNotSet(t *testing.T) {
	var expected *osbuild.DNFConfigStageOptions
	cnf := &ImageConfig{}
	options, err := cnf.DNFConfigOptions("9-stream")
	assert.NoError(t, err)
	assert.Equal(t, expected, options)

	cnf.DNFConfig = &DNFConfig{
		SetReleaseverVar: common.ToPtr(false),
	}
	options, err = cnf.DNFConfigOptions("9-stream")
	assert.NoError(t, err)
	assert.Equal(t, expected, options)
}

func TestImageConfigDNFConfigOptions(t *testing.T) {
	type testCase struct {
		config *DNFConfig

		expected *osbuild.DNFConfigStageOptions
		expErr   string
	}

	// the exact value of os release doesn't matter, so we use the same one for all cases
	osRelease := "9-stream"

	testCases := map[string]testCase{
		"nothing": {},
		"ipresolve": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Config: &osbuild.DNFConfig{
						Main: &osbuild.DNFConfigMain{
							IPResolve: "4",
						},
					},
				},
			},
			expected: &osbuild.DNFConfigStageOptions{
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		"ipresolve+noreleasever": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Config: &osbuild.DNFConfig{
						Main: &osbuild.DNFConfigMain{
							IPResolve: "4",
						},
					},
				},
				SetReleaseverVar: common.ToPtr(false),
			},
			expected: &osbuild.DNFConfigStageOptions{
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		"ipresolve+releasever": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Config: &osbuild.DNFConfig{
						Main: &osbuild.DNFConfigMain{
							IPResolve: "4",
						},
					},
				},
				SetReleaseverVar: common.ToPtr(true),
			},
			expected: &osbuild.DNFConfigStageOptions{
				Variables: []osbuild.DNFVariable{
					{
						Name:  "releasever",
						Value: osRelease,
					},
				},
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		"releasever": {
			config: &DNFConfig{
				SetReleaseverVar: common.ToPtr(true),
			},
			expected: &osbuild.DNFConfigStageOptions{
				Variables: []osbuild.DNFVariable{
					{
						Name:  "releasever",
						Value: osRelease,
					},
				},
			},
		},
		"ipresolve+somevar": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Variables: []osbuild.DNFVariable{
						{
							Name:  "custom-var",
							Value: "custom-val",
						},
					},
					Config: &osbuild.DNFConfig{
						Main: &osbuild.DNFConfigMain{
							IPResolve: "4",
						},
					},
				},
			},
			expected: &osbuild.DNFConfigStageOptions{
				Variables: []osbuild.DNFVariable{
					{
						Name:  "custom-var",
						Value: "custom-val",
					},
				},
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		"somevar+releasever": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Variables: []osbuild.DNFVariable{
						{
							Name:  "custom-var",
							Value: "custom-val",
						},
					},
				},
				SetReleaseverVar: common.ToPtr(true),
			},
			expected: &osbuild.DNFConfigStageOptions{
				Variables: []osbuild.DNFVariable{
					{
						Name:  "custom-var",
						Value: "custom-val",
					},
					{
						Name:  "releasever",
						Value: osRelease,
					},
				},
			},
		},
		"ipresolve+somevar+releasever": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Variables: []osbuild.DNFVariable{
						{
							Name:  "custom-var",
							Value: "custom-val",
						},
					},
					Config: &osbuild.DNFConfig{
						Main: &osbuild.DNFConfigMain{
							IPResolve: "4",
						},
					},
				},
				SetReleaseverVar: common.ToPtr(true),
			},
			expected: &osbuild.DNFConfigStageOptions{
				Variables: []osbuild.DNFVariable{
					{
						Name:  "custom-var",
						Value: "custom-val",
					},
					{
						Name:  "releasever",
						Value: osRelease,
					},
				},
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		"variable-conflict": {
			config: &DNFConfig{
				Options: &osbuild.DNFConfigStageOptions{
					Variables: []osbuild.DNFVariable{
						{
							Name:  "custom-var",
							Value: "custom-val",
						},
						{
							Name:  "releasever",
							Value: "100.42",
						},
					},
					Config: &osbuild.DNFConfig{
						Main: &osbuild.DNFConfigMain{
							IPResolve: "4",
						},
					},
				},
				SetReleaseverVar: common.ToPtr(true),
			},
			expErr: "dnf_config.set_releasever_var is enabled and conflicts with the releasever variable set in dnf_config.options.variables with value: 100.42",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ic := &ImageConfig{
				DNFConfig: tc.config,
			}

			options, err := ic.DNFConfigOptions(osRelease)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.Equal(tc.expected, options)
		})
	}
}
