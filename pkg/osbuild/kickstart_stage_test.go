package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/pkg/customizations/users"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func TestKickstartStageJsonHappy(t *testing.T) {
	opts := &osbuild.KickstartStageOptions{
		Path: "/osbuild.ks",
		Bootloader: &osbuild.BootloaderOptions{
			Append: "karg1 karg2=0",
		},
	}
	stage := osbuild.NewKickstartStage(opts)
	require.NotNil(t, stage)
	stageJson, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(stageJson), `{
  "type": "org.osbuild.kickstart",
  "options": {
    "path": "/osbuild.ks",
    "bootloader": {
      "append": "karg1 karg2=0"
    }
  }
}`)
}

func TestKickstartStageUsers(t *testing.T) {
	type testCase struct {
		users    []users.User
		expected *osbuild.KickstartStageOptions
		expErr   string
	}

	testCases := map[string]testCase{
		"empty": {
			users:    nil,
			expected: &osbuild.KickstartStageOptions{},
			expErr:   "",
		},
		"1-user": {
			users: []users.User{
				{
					Name:               "user",
					Description:        common.ToPtr("I am user"),
					Password:           common.ToPtr("$6$fakesalt$fakehashedpassword"),
					Key:                common.ToPtr("ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
					Home:               common.ToPtr("/var/home/user"),
					Shell:              common.ToPtr("/usr/bin/fish"),
					Groups:             []string{"grp1", "wheel"},
					UID:                common.ToPtr(1010),
					GID:                common.ToPtr(1020),
					ExpireDate:         common.ToPtr(1756486205),
					ForcePasswordReset: common.ToPtr(false),
				},
			},
			expected: &osbuild.KickstartStageOptions{
				Users: map[string]osbuild.UsersStageOptionsUser{
					"user": {
						UID:                common.ToPtr(1010),
						GID:                common.ToPtr(1020),
						Groups:             []string{"grp1", "wheel"},
						Description:        common.ToPtr("I am user"),
						Home:               common.ToPtr("/var/home/user"),
						Shell:              common.ToPtr("/usr/bin/fish"),
						Password:           common.ToPtr("$6$fakesalt$fakehashedpassword"),
						Key:                common.ToPtr("ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
						ExpireDate:         common.ToPtr(1756486205),
						ForcePasswordReset: common.ToPtr(false),
					},
				},
			},
			expErr: "",
		},
		"2-user+root": {
			users: []users.User{
				{
					Name:               "user",
					Description:        common.ToPtr("I am user"),
					Password:           common.ToPtr("$6$fakesalt$fakehashedpassword"),
					Key:                common.ToPtr("ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
					Home:               common.ToPtr("/var/home/user"),
					Shell:              common.ToPtr("/usr/bin/fish"),
					Groups:             []string{"grp1", "wheel"},
					UID:                common.ToPtr(1010),
					GID:                common.ToPtr(1020),
					ExpireDate:         common.ToPtr(1756486205),
					ForcePasswordReset: common.ToPtr(false),
				},
				{
					Name:     "root",
					Password: common.ToPtr("$6$fakesaltroot$fakehashedpasswordroot"),
					Key:      common.ToPtr("ssh-ed25519 BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
				},
			},
			expected: &osbuild.KickstartStageOptions{
				Users: map[string]osbuild.UsersStageOptionsUser{
					"user": {
						Description:        common.ToPtr("I am user"),
						Password:           common.ToPtr("$6$fakesalt$fakehashedpassword"),
						Key:                common.ToPtr("ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
						Home:               common.ToPtr("/var/home/user"),
						Shell:              common.ToPtr("/usr/bin/fish"),
						Groups:             []string{"grp1", "wheel"},
						UID:                common.ToPtr(1010),
						GID:                common.ToPtr(1020),
						ExpireDate:         common.ToPtr(1756486205),
						ForcePasswordReset: common.ToPtr(false),
					},
					"root": {
						Key: common.ToPtr("ssh-ed25519 BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
					},
				},
				RootPassword: &osbuild.RootPasswordOptions{
					Lock:      false,
					PlainText: false,
					IsCrypted: true,
					AllowSSH:  false,
					Password:  "$6$fakesaltroot$fakehashedpasswordroot",
				},
			},
		},
		"2-user+root-error": {
			users: []users.User{
				{
					Name:               "user",
					Description:        common.ToPtr("I am user"),
					Password:           common.ToPtr("$6$fakesalt$fakehashedpassword"),
					Key:                common.ToPtr("ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
					Home:               common.ToPtr("/var/home/user"),
					Shell:              common.ToPtr("/usr/bin/fish"),
					Groups:             []string{"grp1", "wheel"},
					UID:                common.ToPtr(1010),
					GID:                common.ToPtr(1020),
					ExpireDate:         common.ToPtr(1756486205),
					ForcePasswordReset: common.ToPtr(false),
				},
				{
					Name:               "root",
					Description:        common.ToPtr("super!"),
					Password:           common.ToPtr("$6$fakesaltroot$fakehashedpasswordroot"),
					Key:                common.ToPtr("ssh-ed25519 BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
					Home:               common.ToPtr("/rooot"),
					Shell:              common.ToPtr("/usr/bin/zsh"),
					Groups:             []string{"wheel?"},
					UID:                common.ToPtr(10),
					GID:                common.ToPtr(20),
					ExpireDate:         common.ToPtr(1756486205),
					ForcePasswordReset: common.ToPtr(false),
				},
			},
			expected: &osbuild.KickstartStageOptions{
				Users: map[string]osbuild.UsersStageOptionsUser{
					"user": {
						Description:        common.ToPtr("I am user"),
						Password:           common.ToPtr("$6$fakesalt$fakehashedpassword"),
						Key:                common.ToPtr("ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
						Home:               common.ToPtr("/var/home/user"),
						Shell:              common.ToPtr("/usr/bin/fish"),
						Groups:             []string{"grp1", "wheel"},
						UID:                common.ToPtr(1010),
						GID:                common.ToPtr(1020),
						ExpireDate:         common.ToPtr(1756486205),
						ForcePasswordReset: common.ToPtr(false),
					},
					"root": {
						Description:        common.ToPtr("super!"),
						Password:           common.ToPtr("$6$fakesaltroot$fakehashedpasswordroot"),
						Key:                common.ToPtr("ssh-ed25519 BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
						Home:               common.ToPtr("/rooot"),
						Shell:              common.ToPtr("/usr/bin/zsh"),
						Groups:             []string{"wheel?"},
						UID:                common.ToPtr(10),
						GID:                common.ToPtr(20),
						ExpireDate:         common.ToPtr(1756486205),
						ForcePasswordReset: common.ToPtr(false),
					},
				},
			},
			expErr: "org.osbuild.kickstart: unsupported options for user \"root\": expiredate, force_password_reset, gid, groups, home, shell, uid",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ksOpts, err := osbuild.NewKickstartStageOptions("/test.ks", tc.users, nil)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			exp := tc.expected
			exp.Path = "/test.ks"
			assert.Equal(tc.expected, ksOpts)
		})
	}

}

func TestNewKickstartOptionsPlain(t *testing.T) {
	type testCase struct {
		path                string
		userCustomizations  []users.User
		groupCustomizations []users.Group

		expOptions *osbuild.KickstartStageOptions
		expErr     string
	}

	testCases := map[string]testCase{
		"empty": {
			expErr: "org.osbuild.kickstart: kickstart path \"\" is invalid",
		},
		"bad-path": {
			path:   `C:\Wrong\Operating\System`,
			expErr: `org.osbuild.kickstart: kickstart path "C:\\Wrong\\Operating\\System" is invalid`,
		},
		"path-only": {
			path: "/osbuild-test.ks",
			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
			},
		},
		"user": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
			},
		},
		"user+group": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			groupCustomizations: []users.Group{
				{
					Name: "staff",
					GID:  common.ToPtr(2000),
				},
			},
			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
				Groups: map[string]osbuild.GroupsStageOptionsGroup{
					"staff": {
						GID: common.ToPtr(2000),
					},
				},
			},
		},
		"good-root-options": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:     "root",
					Password: common.ToPtr("$6$BhyxFBgrEFh0VrPJ$MllG8auiU26x2pmzL4.1maHzPHrA.4gTdCvlATFp8HJU9UPee4zCS9BVl2HOzKaUYD/zEm8r/OF05F2icWB0K/"),
				},
			},
			expOptions: &osbuild.KickstartStageOptions{
				Path:  "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{}, // non-nil empty map is returned
				RootPassword: &osbuild.RootPasswordOptions{
					IsCrypted: true,
					Password:  "$6$BhyxFBgrEFh0VrPJ$MllG8auiU26x2pmzL4.1maHzPHrA.4gTdCvlATFp8HJU9UPee4zCS9BVl2HOzKaUYD/zEm8r/OF05F2icWB0K/",
				},
			},
		},
		"bad-root-options": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:   "root",
					Groups: []string{"wheel"},
				},
			},
			expErr: "org.osbuild.kickstart: unsupported options for user \"root\": groups",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ksOptions, err := osbuild.NewKickstartStageOptions(
				tc.path,
				tc.userCustomizations,
				tc.groupCustomizations,
			)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expOptions, ksOptions) // new file path must be the original kickstart path
		})
	}
}

func TestNewKicstartStageOptionsWithOSTreeCommit(t *testing.T) {
	type testCase struct {
		path               string
		userCustomizations []users.User
		ostreeURL          string
		ostreeRef          string
		ostreeRemote       string
		osName             string

		expOptions *osbuild.KickstartStageOptions
		expErr     string
	}

	testCases := map[string]testCase{
		"empty": {
			expErr: "org.osbuild.kickstart: kickstart path \"\" is invalid",
		},
		"user": {

			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
			},
		},
		"ostree": {
			path:         "/osbuild-test.ks",
			ostreeURL:    "https://example.org/internal/ostree/repo",
			ostreeRef:    "example/aarch64/1",
			ostreeRemote: "https://example.org/prod/ostree/repo",
			osName:       "guess-what-its-example",

			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				OSTreeCommit: &osbuild.OSTreeCommitOptions{
					OSName: "guess-what-its-example",
					Remote: "https://example.org/prod/ostree/repo",
					URL:    "https://example.org/internal/ostree/repo",
					Ref:    "example/aarch64/1",
					GPG:    false,
				},
			},
		},
		"user+ostree": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			ostreeURL: "https://example.org/ostree/repo",
			ostreeRef: "example/aarch64/1",

			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
				OSTreeCommit: &osbuild.OSTreeCommitOptions{
					Ref: "example/aarch64/1",
					URL: "https://example.org/ostree/repo",
				},
			},
		},
		"internal-error": {
			path: "/osbuild-test.ks",
			// only need to check that an error from NewKickstartStageOptions
			// is propagated
			userCustomizations: []users.User{
				{
					Name:  "root",
					Shell: common.ToPtr("/bin/tsh"),
				},
			},
			expErr: "org.osbuild.kickstart: unsupported options for user \"root\": shell",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ksOptions, err := osbuild.NewKickstartStageOptionsWithOSTreeCommit(
				tc.path,
				tc.userCustomizations,
				nil,
				tc.ostreeURL,
				tc.ostreeRef,
				tc.ostreeRemote,
				tc.osName,
			)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expOptions, ksOptions) // new file path must be the original kickstart path
		})
	}
}

func TestNewKicstartStageOptionsWithOSTreeContainer(t *testing.T) {
	type testCase struct {
		path               string
		userCustomizations []users.User
		containerURL       string
		containerTransport string
		containerRemote    string
		containerStateRoot string

		expOptions *osbuild.KickstartStageOptions
		expErr     string
	}

	testCases := map[string]testCase{
		"empty": {
			expErr: "org.osbuild.kickstart: kickstart path \"\" is invalid",
		},
		"user": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
			},
		},
		"container": {
			path:               "/osbuild-test.ks",
			containerURL:       "https://example.org/internal/some-kind-of-container",
			containerTransport: "docker",
			containerRemote:    "origin",
			containerStateRoot: "default",

			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				OSTreeContainer: &osbuild.OSTreeContainerOptions{
					StateRoot: "default",
					URL:       "https://example.org/internal/some-kind-of-container",
					Transport: "docker",
					Remote:    "origin",
				},
			},
		},
		"user+container": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			containerURL:       "https://example.org/internal/some-kind-of-container",
			containerTransport: "docker",
			containerRemote:    "origin",
			containerStateRoot: "default",

			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
				OSTreeContainer: &osbuild.OSTreeContainerOptions{
					StateRoot: "default",
					URL:       "https://example.org/internal/some-kind-of-container",
					Transport: "docker",
					Remote:    "origin",
				},
			},
		},
		"internal-error": {
			path: "/osbuild-test.ks",
			// only need to check that an error from NewKickstartStageOptions
			// is propagated
			userCustomizations: []users.User{
				{
					Name:  "root",
					Shell: common.ToPtr("/bin/tsh"),
				},
			},
			expErr: "org.osbuild.kickstart: unsupported options for user \"root\": shell",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ksOptions, err := osbuild.NewKickstartStageOptionsWithOSTreeContainer(
				tc.path,
				tc.userCustomizations,
				nil,
				tc.containerURL,
				tc.containerTransport,
				tc.containerRemote,
				tc.containerStateRoot,
			)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expOptions, ksOptions) // new file path must be the original kickstart path
		})
	}
}

func TestNewKicstartStageOptionsWithLiveIMG(t *testing.T) {
	type testCase struct {
		path               string
		userCustomizations []users.User
		imgURL             string

		expOptions *osbuild.KickstartStageOptions
		expErr     string
	}

	testCases := map[string]testCase{
		"empty": {
			expErr: "org.osbuild.kickstart: kickstart path \"\" is invalid",
		},
		"user": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
			},
		},
		"img": {
			path:   "/osbuild-test.ks",
			imgURL: "/path/to/tarball/image.tar",

			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				LiveIMG: &osbuild.LiveIMGOptions{
					URL: "/path/to/tarball/image.tar",
				},
			},
		},
		"user+img": {
			path: "/osbuild-test.ks",
			userCustomizations: []users.User{
				{
					Name:  "fisher",
					Shell: common.ToPtr("/bin/fish"),
				},
			},
			imgURL: "/path/to/tarball/image.tar",

			expOptions: &osbuild.KickstartStageOptions{
				Path: "/osbuild-test.ks",
				Users: map[string]osbuild.UsersStageOptionsUser{
					"fisher": {
						Shell: common.ToPtr("/bin/fish"),
					},
				},
				LiveIMG: &osbuild.LiveIMGOptions{
					URL: "/path/to/tarball/image.tar",
				},
			},
		},
		"internal-error": {
			path: "/osbuild-test.ks",
			// only need to check that an error from NewKickstartStageOptions
			// is propagated
			userCustomizations: []users.User{
				{
					Name:  "root",
					Shell: common.ToPtr("/bin/tsh"),
				},
			},
			expErr: "org.osbuild.kickstart: unsupported options for user \"root\": shell",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ksOptions, err := osbuild.NewKickstartStageOptionsWithLiveIMG(
				tc.path,
				tc.userCustomizations,
				nil,
				tc.imgURL,
			)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expOptions, ksOptions) // new file path must be the original kickstart path
		})
	}
}

func TestIncludeRaw(t *testing.T) {
	type testCase struct {
		path string
		raw  string

		expPath string
		expData string
		expErr  string
	}

	testCases := map[string]testCase{
		"ok-empty": {
			path: "/t.ks",
			raw:  "",

			expPath: "/t-base.ks",
			expData: "%include /run/install/repo/t-base.ks\n",
		},
		"empty-path": {
			expErr: "path must not be empty",
		},
		"rel-path": {
			path:   "42.ps",
			expErr: "path must be absolute",
		},

		"kickstart-command": {
			path: "/my.ks",
			raw:  "bootc switch --mutate-in-plase --transport registry registry.example.org/my-weird-stuff:v42",

			expPath: "/my-base.ks",
			expData: "%include /run/install/repo/my-base.ks\nbootc switch --mutate-in-plase --transport registry registry.example.org/my-weird-stuff:v42",
		},
		"kickstart-commands": {
			path: "/your.ks",
			raw:  "autopart --nohome --type=btrfs\neula --agreed",

			expPath: "/your-base.ks",
			expData: "%include /run/install/repo/your-base.ks\nautopart --nohome --type=btrfs\neula --agreed",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// only the path is relevant
			ksOptions := &osbuild.KickstartStageOptions{
				Path: tc.path,
			}
			file, err := ksOptions.IncludeRaw(tc.raw)
			if tc.expErr != "" {
				assert.EqualError(err, tc.expErr)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expPath, ksOptions.Path)
			assert.Equal(tc.expData, string(file.Data()))
			assert.Equal(tc.path, string(file.Path())) // new file path must be the original kickstart path
		})
	}
}
