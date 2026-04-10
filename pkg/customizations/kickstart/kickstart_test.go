package kickstart_test

import (
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/stretchr/testify/assert"
)

func TestNewKickstart(t *testing.T) {
	type testCase struct {
		customizations *blueprint.Customizations

		expOptions *kickstart.Options
		expErr     string
	}

	testCases := map[string]testCase{
		"empty": {
			expOptions: &kickstart.Options{
				Users:  nil,
				Groups: nil,
			},
		},

		"users+groups": {
			// we don't need to extensively test the whole blueprint.User ->
			// users.User and blueprint.Group -> users.Group conversion here,
			// so just one test case is enough
			customizations: &blueprint.Customizations{
				User: []blueprint.UserCustomization{
					{
						Name: "alice",
						Key:  common.ToPtr("ssh-ed25519 iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"),
						Home: common.ToPtr("/var/home/alice"),
					},
					{
						Name: "bob",
						Key:  common.ToPtr("ssh-ed25519 eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
						Home: common.ToPtr("/var/home/bob"),
					},
				},
				Group: []blueprint.GroupCustomization{
					{
						Name: "data",
						GID:  common.ToPtr(2010),
					},
					{
						Name: "datarw",
						GID:  common.ToPtr(2020),
					},
				},
			},

			expOptions: &kickstart.Options{
				Users: []users.User{
					{
						Name: "alice",
						Key:  common.ToPtr("ssh-ed25519 iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"),
						Home: common.ToPtr("/var/home/alice"),
					},
					{
						Name: "bob",
						Key:  common.ToPtr("ssh-ed25519 eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
						Home: common.ToPtr("/var/home/bob"),
					},
				},
				Groups: []users.Group{
					{
						Name: "data",
						GID:  common.ToPtr(2010),
					},
					{
						Name: "datarw",
						GID:  common.ToPtr(2020),
					},
				},
			},
		},

		"installer-customizations-kickstart-contents": {
			customizations: &blueprint.Customizations{
				Installer: &blueprint.InstallerCustomization{
					Kickstart: &blueprint.Kickstart{
						Contents: "echo 'Hello!!'",
					},
				},
			},

			expOptions: &kickstart.Options{
				Users:  nil,
				Groups: nil,
				UserFile: &kickstart.File{
					Contents: "echo 'Hello!!'",
				},
			},
		},

		"installer-customizations-unattended": {
			customizations: &blueprint.Customizations{
				Installer: &blueprint.InstallerCustomization{
					Unattended: true,
				},
			},

			expOptions: &kickstart.Options{
				Users:      nil,
				Groups:     nil,
				Unattended: true,
			},
		},

		"error/installer-customizations-kickstart+unattended": {
			customizations: &blueprint.Customizations{
				Installer: &blueprint.InstallerCustomization{
					Kickstart: &blueprint.Kickstart{
						Contents: "echo 'Hello!!'",
					},
					Unattended: true,
				},
			},

			expErr: "installer.unattended is not supported when adding custom kickstart contents",
		},

		"error/unsupported-root-options": {
			customizations: &blueprint.Customizations{
				User: []blueprint.UserCustomization{
					{
						Name:   "root",
						Home:   common.ToPtr("/megaroot"),
						Shell:  common.ToPtr("/bin/zsh"),
						Groups: []string{"cups"},
					},
				},
			},

			expErr: "unsupported kickstart options for user \"root\": groups, home, shell",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			options, err := kickstart.New(tc.customizations)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expOptions, options)
		})
	}
}

func TestIsZeroKickstart(t *testing.T) {
	emptyKickstart, err := kickstart.New(&blueprint.Customizations{})

	assert.NoError(t, err)
	assert.Equal(t, true, emptyKickstart.IsZero())
}
