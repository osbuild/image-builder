package osbuild

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/pkg/customizations/subscription"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRhsmStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rhsm",
		Options: &RHSMStageOptions{},
	}
	actualStage := NewRHSMStage(&RHSMStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestRhsmStageJson(t *testing.T) {
	tests := []struct {
		Options    RHSMStageOptions
		JsonString string
	}{
		{
			Options: RHSMStageOptions{
				YumPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: true,
					},
					SubscriptionManager: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			},
			JsonString: `{"yum-plugins":{"product-id":{"enabled":true},"subscription-manager":{"enabled":false}}}`,
		},
		{
			Options: RHSMStageOptions{
				DnfPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: true,
					},
					SubscriptionManager: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			},
			JsonString: `{"dnf-plugins":{"product-id":{"enabled":true},"subscription-manager":{"enabled":false}}}`,
		},
		{
			Options: RHSMStageOptions{
				SubMan: &RHSMStageOptionsSubMan{
					Rhsm:      &SubManConfigRHSMSection{},
					Rhsmcertd: &SubManConfigRHSMCERTDSection{},
				},
			},
			JsonString: `{"subscription-manager":{"rhsm":{},"rhsmcertd":{}}}`,
		},
	}
	for _, test := range tests {
		marshaledJson, err := json.Marshal(test.Options)
		require.NoError(t, err, "failed to marshal JSON")
		require.Equal(t, string(marshaledJson), test.JsonString)

		var jsonOptions RHSMStageOptions
		err = json.Unmarshal([]byte(test.JsonString), &jsonOptions)
		require.NoError(t, err, "failed to parse JSON")
		require.True(t, reflect.DeepEqual(test.Options, jsonOptions))
	}
}

func TestNewRHSMStageOptions(t *testing.T) {
	type testCase struct {
		config       *subscription.RHSMConfig
		stageOptions *RHSMStageOptions
	}

	testCases := []testCase{
		{
			config: &subscription.RHSMConfig{
				DnfPlugins: subscription.SubManDNFPluginsConfig{
					ProductID: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
					SubscriptionManager: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
				YumPlugins: subscription.SubManDNFPluginsConfig{
					ProductID: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: subscription.SubManConfig{
					Rhsm: subscription.SubManRHSMConfig{
						ManageRepos:          common.ToPtr(true),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
					Rhsmcertd: subscription.SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
			stageOptions: &RHSMStageOptions{
				DnfPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
					SubscriptionManager: &RHSMStageOptionsDnfPlugin{
						Enabled: true,
					},
				},
				YumPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: true,
					},
					SubscriptionManager: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
				SubMan: &RHSMStageOptionsSubMan{
					Rhsm: &SubManConfigRHSMSection{
						ManageRepos:          common.ToPtr(true),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
					Rhsmcertd: &SubManConfigRHSMCERTDSection{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		{
			config: &subscription.RHSMConfig{
				DnfPlugins: subscription.SubManDNFPluginsConfig{
					ProductID: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: subscription.SubManConfig{
					Rhsm: subscription.SubManRHSMConfig{
						ManageRepos: common.ToPtr(false),
					},
				},
			},
			stageOptions: &RHSMStageOptions{
				DnfPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
				SubMan: &RHSMStageOptionsSubMan{
					Rhsm: &SubManConfigRHSMSection{
						ManageRepos: common.ToPtr(false),
					},
				},
			},
		},
		{
			config: &subscription.RHSMConfig{
				SubMan: subscription.SubManConfig{
					Rhsm: subscription.SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: subscription.SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
			stageOptions: &RHSMStageOptions{
				SubMan: &RHSMStageOptionsSubMan{
					Rhsm: &SubManConfigRHSMSection{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: &SubManConfigRHSMCERTDSection{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			assert.EqualValues(t, tc.stageOptions, NewRHSMStageOptions(tc.config))
		})
	}
}
