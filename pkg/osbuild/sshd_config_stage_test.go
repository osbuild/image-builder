package osbuild

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/internal/common"
)

func TestNewSshdConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.sshd.config",
		Options: &SshdConfigStageOptions{},
	}
	actualStage := NewSshdConfigStage(&SshdConfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestJsonYamlSshdConfigStage(t *testing.T) {
	// First test that the JSON can be parsed into the expected structure.
	expectedOptions := SshdConfigStageOptions{
		Config: SshdConfigConfig{
			PasswordAuthentication:          common.ToPtr(false),
			ChallengeResponseAuthentication: common.ToPtr(false),
			ClientAliveInterval:             common.ToPtr(180),
			PermitRootLogin:                 PermitRootLoginValueProhibitPassword,
		},
	}
	inputStringJSON := `{
		"config": {
		  "PasswordAuthentication": false,
		  "ChallengeResponseAuthentication": false,
		  "ClientAliveInterval": 180,
		  "PermitRootLogin": "prohibit-password"
		}
	  }`
	inputStringYAML := `
config:
  PasswordAuthentication: false
  ChallengeResponseAuthentication: false
  ClientAliveInterval: 180
  PermitRootLogin: "prohibit-password"
`
	var inputOptions SshdConfigStageOptions
	err := json.Unmarshal([]byte(inputStringJSON), &inputOptions)
	assert.NoError(t, err, "failed to parse JSON into sshd config")
	assert.True(t, reflect.DeepEqual(expectedOptions, inputOptions))
	// ensure YAML is decoded the same way
	err = yaml.Unmarshal([]byte(inputStringYAML), &inputOptions)
	assert.NoError(t, err)
	assert.Equal(t, expectedOptions, inputOptions)

	// Second try the other way around with stress on missing values
	// for those parameters that the user didn't specify.
	inputOptions = SshdConfigStageOptions{
		Config: SshdConfigConfig{
			PasswordAuthentication: common.ToPtr(true),
		},
	}
	expectedString := `{"config":{"PasswordAuthentication":true}}`
	inputBytes, err := json.Marshal(inputOptions)
	assert.NoError(t, err, "failed to marshal sshd config into JSON")
	assert.Equal(t, expectedString, string(inputBytes))
}

func TestSshdConfigStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options SshdConfigStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: SshdConfigStageOptions{},
			err:     false,
		},
		{
			name: "invalid-permit-root-login-str-value",
			options: SshdConfigStageOptions{
				Config: SshdConfigConfig{
					PermitRootLogin: PermitRootLoginValueStr("invalid"),
				},
			},
			err: true,
		},
		{
			name: "valid-permit-root-login-str-value-1",
			options: SshdConfigStageOptions{
				Config: SshdConfigConfig{
					PermitRootLogin: PermitRootLoginValueForcedCommandsOnly,
				},
			},
			err: false,
		},
		{
			name: "valid-permit-root-login-str-value-1",
			options: SshdConfigStageOptions{
				Config: SshdConfigConfig{
					PermitRootLogin: PermitRootLoginValueProhibitPassword,
				},
			},
			err: false,
		},
	}

	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewSshdConfigStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewSshdConfigStage(&tt.options) })
			}
		})
	}
}
