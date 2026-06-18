package osbuild

import (
	"testing"

	"github.com/osbuild/image-builder/pkg/customizations/oscap"
	"github.com/stretchr/testify/assert"
)

func TestNewOscapRemediationStage(t *testing.T) {
	stageOptions := &OscapRemediationStageOptions{DataDir: "/var/tmp", Config: OscapConfig{
		Datastream: "test_stream",
		ProfileID:  "test_profile",
	}}
	expectedStage := &Stage{
		Type:    "org.osbuild.oscap.remediation",
		Options: stageOptions,
	}
	actualStage := NewOscapRemediationStage(stageOptions)
	assert.Equal(t, expectedStage, actualStage)
}

func TestOscapRemediationStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options OscapRemediationStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: OscapRemediationStageOptions{},
			err:     true,
		},
		{
			name: "empty-datastream",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					ProfileID: "test-profile",
				},
			},
			err: true,
		},
		{
			name: "empty-profile-id",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					Datastream: "test-datastream",
				},
			},
			err: true,
		},
		{
			name: "invalid-verbosity-level",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					Datastream:   "test-datastream",
					ProfileID:    "test-profile",
					VerboseLevel: "FAKE",
				},
			},
			err: true,
		},
		{
			name: "valid-data",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					Datastream:   "test-datastream",
					ProfileID:    "test-profile",
					VerboseLevel: "INFO",
					Compression:  true,
				},
			},
			err: false,
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.Config.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewOscapRemediationStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.Config.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewOscapRemediationStage(&tt.options) })
			}
		})
	}
}

func TestInternalConfigToStageOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  *oscap.RemediationConfig
		expected OscapConfig
	}{
		{
			name: "no-tailoring",
			options: &oscap.RemediationConfig{
				ProfileID:  "some-profile",
				Datastream: "some-datastream",
			},
			expected: OscapConfig{
				ProfileID:  "some-profile",
				Datastream: "some-datastream",
			},
		},
		{
			name: "tailoring",
			options: &oscap.RemediationConfig{
				ProfileID:  "some-profile",
				Datastream: "some-datastream",
				TailoringConfig: &oscap.TailoringConfig{
					TailoredProfileID: "some-tailored-profile",
					TailoringPath:     "/some/tailoring/path.xml",
					Selected:          []string{"one", "two", "three"},
				},
			},
			expected: OscapConfig{
				ProfileID:  "some-tailored-profile",
				Datastream: "some-datastream",
				Tailoring:  "/some/tailoring/path.xml",
			},
		},
		{
			name: "json-tailoring",
			options: &oscap.RemediationConfig{
				ProfileID:  "some-profile",
				Datastream: "some-datastream",
				TailoringConfig: &oscap.TailoringConfig{
					TailoredProfileID: "some-tailored-profile",
					TailoringPath:     "/some/tailoring/path.xml",
					JSONFilepath:      "/some/tailoring/path.json",
				},
			},
			expected: OscapConfig{
				ProfileID:  "some-tailored-profile",
				Datastream: "some-datastream",
				Tailoring:  "/some/tailoring/path.xml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stageOptions := NewOscapRemediationStageOptions("data-dir", tt.options)
			assert.Equal(t, tt.expected, stageOptions.Config)
		})
	}
}
