package subscription

import (
	"fmt"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/stretchr/testify/assert"
)

// rhsmConfigNotSamePointers is a helper function to check that the pointers in the
// two RHSMConfig objects are not the same.
func rhsmConfigNotSamePointers(t *testing.T, c1, c2 *RHSMConfig) {
	if c1 != nil && c2 != nil {
		assert.NotSame(t, c1, c2)

		if c1.DnfPlugins.ProductID.Enabled != nil && c2.DnfPlugins.ProductID.Enabled != nil {
			assert.NotSame(t, c1.DnfPlugins.ProductID.Enabled, c2.DnfPlugins.ProductID.Enabled)
		}
		if c1.DnfPlugins.SubscriptionManager.Enabled != nil && c2.DnfPlugins.SubscriptionManager.Enabled != nil {
			assert.NotSame(t, c1.DnfPlugins.SubscriptionManager.Enabled, c2.DnfPlugins.SubscriptionManager.Enabled)
		}

		if c1.YumPlugins.ProductID.Enabled != nil && c2.YumPlugins.ProductID.Enabled != nil {
			assert.NotSame(t, c1.YumPlugins.ProductID.Enabled, c2.YumPlugins.ProductID.Enabled)
		}
		if c1.YumPlugins.SubscriptionManager.Enabled != nil && c2.YumPlugins.SubscriptionManager.Enabled != nil {
			assert.NotSame(t, c1.YumPlugins.SubscriptionManager.Enabled, c2.YumPlugins.SubscriptionManager.Enabled)
		}

		if c1.SubMan.Rhsm.ManageRepos != nil {
			if c2.SubMan.Rhsm.ManageRepos != nil {
				assert.NotSame(t, c1.SubMan.Rhsm.ManageRepos, c2.SubMan.Rhsm.ManageRepos)
			}
			if c2.SubMan.Rhsm.AutoEnableYumPlugins != nil {
				assert.NotSame(t, c1.SubMan.Rhsm.AutoEnableYumPlugins, c2.SubMan.Rhsm.AutoEnableYumPlugins)
			}
		}
		if c1.SubMan.Rhsmcertd.AutoRegistration != nil && c2.SubMan.Rhsmcertd.AutoRegistration != nil {
			assert.NotSame(t, c1.SubMan.Rhsmcertd.AutoRegistration, c2.SubMan.Rhsmcertd.AutoRegistration)
		}

	}
}

func TestRHSMConfigClone(t *testing.T) {
	type testCase struct {
		config *RHSMConfig
	}

	testCases := []testCase{
		{
			config: nil,
		},
		{
			config: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
			},
		},
		{
			config: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
			},
		},
		{
			config: &RHSMConfig{
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
				},
			},
		},
		{
			config: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				YumPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos:          common.ToPtr(true),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			clone := tc.config.Clone()
			rhsmConfigNotSamePointers(t, tc.config, clone)
			assert.EqualValues(t, tc.config, clone)
		})
	}

}

func TestRHSMConfigUpdate(t *testing.T) {
	type testCase struct {
		old      *RHSMConfig
		new      *RHSMConfig
		expected *RHSMConfig
	}

	testCases := []testCase{
		{
			old: nil,
			new: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				YumPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				YumPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		{
			old: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				YumPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
			new: nil,
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				YumPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		{
			old: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
			new: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(false),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(true),
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(false),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(true),
					},
				},
			},
		},
		{
			old: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos:          common.ToPtr(false),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
				},
			},
			new: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						AutoEnableYumPlugins: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(true),
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos:          common.ToPtr(false),
						AutoEnableYumPlugins: common.ToPtr(true),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(true),
					},
				},
			},
		},
		{
			old: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
			},
			new: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos:          common.ToPtr(false),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos:          common.ToPtr(false),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
				},
			},
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			updated := tc.old.Update(tc.new)
			rhsmConfigNotSamePointers(t, tc.old, updated)
			rhsmConfigNotSamePointers(t, tc.new, updated)
			assert.EqualValues(t, tc.expected, updated)
		})
	}
}

func TestRHSMConfigFromBP(t *testing.T) {
	type testCase struct {
		bp       *blueprint.RHSMCustomization
		expected *RHSMConfig
	}

	testCases := []testCase{
		{
			bp:       nil,
			expected: nil,
		},
		{
			bp:       &blueprint.RHSMCustomization{},
			expected: nil,
		},
		{
			bp: &blueprint.RHSMCustomization{
				Config: &blueprint.RHSMConfig{},
			},
			expected: &RHSMConfig{},
		},
		{
			bp: &blueprint.RHSMCustomization{
				Config: &blueprint.RHSMConfig{
					DNFPlugins: &blueprint.SubManDNFPluginsConfig{
						ProductID: &blueprint.DNFPluginConfig{
							Enabled: common.ToPtr(true),
						},
						SubscriptionManager: &blueprint.DNFPluginConfig{
							Enabled: common.ToPtr(false),
						},
					},
					SubscriptionManager: &blueprint.SubManConfig{
						RHSMConfig: &blueprint.SubManRHSMConfig{
							ManageRepos:          common.ToPtr(true),
							AutoEnableYumPlugins: common.ToPtr(false),
						},
						RHSMCertdConfig: &blueprint.SubManRHSMCertdConfig{
							AutoRegistration: common.ToPtr(false),
						},
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos:          common.ToPtr(true),
						AutoEnableYumPlugins: common.ToPtr(false),
					},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		{
			bp: &blueprint.RHSMCustomization{
				Config: &blueprint.RHSMConfig{
					DNFPlugins: &blueprint.SubManDNFPluginsConfig{
						ProductID: &blueprint.DNFPluginConfig{
							Enabled: common.ToPtr(true),
						},
						SubscriptionManager: &blueprint.DNFPluginConfig{},
					},
					SubscriptionManager: &blueprint.SubManConfig{
						RHSMConfig: &blueprint.SubManRHSMConfig{},
						RHSMCertdConfig: &blueprint.SubManRHSMCertdConfig{
							AutoRegistration: common.ToPtr(false),
						},
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: DNFPluginConfig{},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{},
					Rhsmcertd: SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		{
			bp: &blueprint.RHSMCustomization{
				Config: &blueprint.RHSMConfig{
					DNFPlugins: &blueprint.SubManDNFPluginsConfig{
						ProductID: &blueprint.DNFPluginConfig{
							Enabled: common.ToPtr(true),
						},
					},
					SubscriptionManager: &blueprint.SubManConfig{
						RHSMConfig: &blueprint.SubManRHSMConfig{
							ManageRepos: common.ToPtr(true),
						},
					},
				},
			},
			expected: &RHSMConfig{
				DnfPlugins: SubManDNFPluginsConfig{
					ProductID: DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
				},
				SubMan: SubManConfig{
					Rhsm: SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
				},
			},
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			rhsmConfig := RHSMConfigFromBP(tc.bp)
			assert.EqualValues(t, tc.expected, rhsmConfig)
		})
	}
}
