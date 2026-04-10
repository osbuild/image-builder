package distro_test

import (
	"reflect"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/stretchr/testify/assert"
)

type TestImageType struct {
	name             string
	supportedOptions []string
	requiredOptions  []string
}

func (t *TestImageType) SupportedBlueprintOptions() []string {
	return t.supportedOptions
}

func (t *TestImageType) RequiredBlueprintOptions() []string {
	return t.requiredOptions
}

func fullBlueprint() blueprint.Blueprint {
	return blueprint.Blueprint{
		Packages: []blueprint.Package{
			{
				Name: "package",
			},
		},
		Modules: []blueprint.Package{
			{
				Name: "module",
			},
		},
		EnabledModules: []blueprint.EnabledModule{
			{
				Name:   "real_module",
				Stream: "10",
			},
		},
		Groups: []blueprint.Group{
			{
				Name: "group",
			},
		},
		Containers: []blueprint.Container{
			{
				Source: "example.com/containers/test",
			},
		},
		Customizations: &blueprint.Customizations{
			Hostname: common.ToPtr("myhost"),
			Kernel: &blueprint.KernelCustomization{
				Name:   "mykernel",
				Append: "option=value",
			},
			SSHKey: []blueprint.SSHKeyCustomization{
				{
					User: "root",
					Key:  "a-very-good-ssh-key",
				},
			},
			User: []blueprint.UserCustomization{
				{
					Name:        "petris",
					Description: common.ToPtr("I am Petris"),
					Password:    common.ToPtr("terrible password"),
					Key:         common.ToPtr("ssh-key"),
					Home:        common.ToPtr("/home/petros"),
					Shell:       common.ToPtr("/bin/ksh"),
					Groups: []string{
						"wheelie",
					},
					UID: common.ToPtr(1042),
					GID: common.ToPtr(1013),
				},
			},
			Group: []blueprint.GroupCustomization{
				{
					Name: "wheelie",
					GID:  common.ToPtr(9901),
				},
			},
			Timezone: &blueprint.TimezoneCustomization{
				Timezone: common.ToPtr("Australia/Adelaide"),
				NTPServers: []string{
					"ntp.example.com",
				},
			},
			Locale: &blueprint.LocaleCustomization{
				Languages: []string{
					"en_GB.UTF-8",
					"el_CY.UTF-8",
				},
				Keyboard: common.ToPtr("uk"),
			},
			Firewall: &blueprint.FirewallCustomization{
				Ports: []string{
					"1337:tcp",
					"1337:udp",
				},
				Services: &blueprint.FirewallServicesCustomization{
					Enabled: []string{
						"leet.service",
					},
					Disabled: []string{
						"noob.service",
					},
				},
				Zones: []blueprint.FirewallZoneCustomization{
					{
						Name: common.ToPtr("new-zone"),
						Sources: []string{
							"192.0.42.0/8",
						},
					},
				},
			},
			Services: &blueprint.ServicesCustomization{
				Enabled: []string{
					"leet.service",
				},
				Disabled: []string{
					"noob.service",
					"bad.service",
				},
				Masked: []string{
					"never.service",
				},
			},
			Filesystem: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/mnt/stuff",
					MinSize:    100,
				},
			},
			Disk: &blueprint.DiskCustomization{
				Type:    "dos",
				MinSize: 100,
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:      "everything",
						MinSize:   42,
						PartType:  "ff",
						PartLabel: "label",
						PartUUID:  "a-uuid",
						BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "subvol",
									Mountpoint: "/subvols/1",
								},
							},
						},
						VGCustomization: blueprint.VGCustomization{
							Name: "vg01",
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name:    "lv01",
									MinSize: 90,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/mnt",
										Label:      "mnt",
										FSType:     "ext4",
									},
								},
							},
						},
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "xfs",
						},
					},
				},
			},
			InstallationDevice: "/dev/full",
			PartitioningMode:   "auto-lvm",
			FDO: &blueprint.FDOCustomization{
				ManufacturingServerURL:  "fdo.example.com",
				DiunPubKeyInsecure:      "insecure",
				DiunPubKeyHash:          "ffffaaaa123",
				DiunPubKeyRootCerts:     "root-cert-key",
				DiMfgStringTypeMacIface: "--",
			},
			OpenSCAP: &blueprint.OpenSCAPCustomization{
				DataStream: "/usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml",
				ProfileID:  "pci-dss",
				Tailoring: &blueprint.OpenSCAPTailoringCustomizations{
					Selected: []string{
						"bind_crypto_policy",
					},
					Unselected: []string{
						"rpm_verify_permissions",
					},
				},
			},
			Ignition: &blueprint.IgnitionCustomization{
				Embedded: &blueprint.EmbeddedIgnitionCustomization{
					Config: "c29tZSBraW5kIG9mIGNvbmZpZwo=",
				},
				FirstBoot: &blueprint.FirstBootIgnitionCustomization{
					ProvisioningURL: "ignition.example.org",
				},
			},
			Directories: []blueprint.DirectoryCustomization{
				{
					Path:          "/etc/path/to/mydir",
					User:          1000,
					Group:         1001,
					Mode:          "700",
					EnsureParents: true,
				},
			},
			Files: []blueprint.FileCustomization{
				{
					Path:  "/etc/path/to/mydir",
					User:  1000,
					Group: 1001,
					Mode:  "700",
					Data:  "SEVMUCEgIEknbSB0cmFwcGVkIGluIGEgdGVzdCEhCg==",
				},
			},
			Repositories: []blueprint.RepositoryCustomization{
				{
					Id: "baseappstream",
					BaseURLs: []string{
						"https://base.repo.example.org",
					},
					GPGKeys: []string{
						"KEY!!!",
					},
					Metalink:       "https://meta.repo.example.org",
					Mirrorlist:     "https://mirrors.repo.example.org",
					Name:           "baseappstream",
					Priority:       common.ToPtr(3),
					Enabled:        common.ToPtr(true),
					GPGCheck:       common.ToPtr(true),
					RepoGPGCheck:   common.ToPtr(true),
					SSLVerify:      common.ToPtr(true),
					ModuleHotfixes: common.ToPtr(false),
					Filename:       "baseappstream.repo",
				},
			},
			FIPS: common.ToPtr(false),
			Installer: &blueprint.InstallerCustomization{
				Unattended: true,
				SudoNopasswd: []string{
					"%wheelie",
				},
			},
			RPM: &blueprint.RPMCustomization{
				ImportKeys: &blueprint.RPMImportKeys{
					Files: []string{
						"/etc/pki/rpm-gpg/RPM-GPG-KEY-fedora-18-primary",
					},
				},
			},
			RHSM: &blueprint.RHSMCustomization{
				Config: &blueprint.RHSMConfig{
					DNFPlugins: &blueprint.SubManDNFPluginsConfig{
						ProductID: &blueprint.DNFPluginConfig{
							Enabled: common.ToPtr(false),
						},
						SubscriptionManager: &blueprint.DNFPluginConfig{
							Enabled: common.ToPtr(true),
						},
					},
					SubscriptionManager: &blueprint.SubManConfig{
						RHSMConfig: &blueprint.SubManRHSMConfig{
							ManageRepos:          common.ToPtr(true),
							AutoEnableYumPlugins: common.ToPtr(true),
						},
						RHSMCertdConfig: &blueprint.SubManRHSMCertdConfig{
							AutoRegistration: common.ToPtr(true),
						},
					},
				},
			},
			CACerts: &blueprint.CACustomization{
				PEMCerts: []string{
					"-----BEGIN CERTIFICATE-----...-----END CERTIFICATE-----",
				},
			},
			ContainersStorage: &blueprint.ContainerStorageCustomization{
				StoragePath: common.ToPtr("/usr/share/my-containers"),
			},
		},
		Distro: "fedora-99",
	}
}

func allOptionStrings() []string {
	return []string{
		"containers",
		"customizations.cacerts.pem_certs",
		"customizations.containers-storage.destination-path",
		"customizations.directories.ensure_parents",
		"customizations.directories.group",
		"customizations.directories.mode",
		"customizations.directories.path",
		"customizations.directories.user",
		"customizations.disk.minsize",
		"customizations.disk.partitions.fs_type",
		"customizations.disk.partitions.label",
		"customizations.disk.partitions.logical_volumes.fs_type",
		"customizations.disk.partitions.logical_volumes.label",
		"customizations.disk.partitions.logical_volumes.minsize",
		"customizations.disk.partitions.logical_volumes.mountpoint",
		"customizations.disk.partitions.logical_volumes.name",
		"customizations.disk.partitions.minsize",
		"customizations.disk.partitions.mountpoint",
		"customizations.disk.partitions.name",
		"customizations.disk.partitions.part_label",
		"customizations.disk.partitions.part_type",
		"customizations.disk.partitions.part_uuid",
		"customizations.disk.partitions.subvolumes.mountpoint",
		"customizations.disk.partitions.subvolumes.name",
		"customizations.disk.partitions.type",
		"customizations.disk.type",
		"customizations.fdo.di_mfg_string_type_mac_iface",
		"customizations.fdo.diun_pub_key_hash",
		"customizations.fdo.diun_pub_key_insecure",
		"customizations.fdo.diun_pub_key_root_certs",
		"customizations.fdo.manufacturing_server_url",
		"customizations.files.data",
		"customizations.files.group",
		"customizations.files.mode",
		"customizations.files.path",
		"customizations.files.user",
		"customizations.filesystem.minsize",
		"customizations.filesystem.mountpoint",
		"customizations.fips",
		"customizations.firewall.ports",
		"customizations.firewall.services",
		"customizations.firewall.zones",
		"customizations.group.gid",
		"customizations.group.name",
		"customizations.hostname",
		"customizations.ignition.embedded.config",
		"customizations.ignition.firstboot.url",
		"customizations.installation_device",
		"customizations.installer.sudo-nopasswd",
		"customizations.installer.unattended",
		"customizations.kernel.append",
		"customizations.kernel.name",
		"customizations.locale.keyboard",
		"customizations.locale.languages",
		"customizations.openscap.datastream",
		"customizations.openscap.profile_id",
		"customizations.openscap.tailoring.selected",
		"customizations.openscap.tailoring.unselected",
		"customizations.partitioning_mode",
		"customizations.repositories",
		"customizations.repositories.baseurls",
		"customizations.repositories.enabled",
		"customizations.repositories.filename",
		"customizations.repositories.gpgcheck",
		"customizations.repositories.gpgkeys",
		"customizations.repositories.id",
		"customizations.repositories.metalink",
		"customizations.repositories.mirrorlist",
		"customizations.repositories.module_hotfixes",
		"customizations.repositories.name",
		"customizations.repositories.priority",
		"customizations.repositories.repo_gpgcheck",
		"customizations.repositories.sslverify",
		"customizations.rhsm.config.dnf_plugins.product_id.enabled",
		"customizations.rhsm.config.dnf_plugins.subscription_manager.enabled",
		"customizations.rhsm.config.subscription_manager.rhsm.auto_enable_yum_plugins",
		"customizations.rhsm.config.subscription_manager.rhsmcertd.auto_registration",
		"customizations.rhsm.config.subscription_manager.rhsm.manage_repos",
		"customizations.rpm.import_keys.files",
		"customizations.services.disabled",
		"customizations.services.enabled",
		"customizations.services.masked",
		"customizations.sshkey",
		"customizations.timezone",
		"customizations.user.description",
		"customizations.user.gid",
		"customizations.user.groups",
		"customizations.user.home",
		"customizations.user.key",
		"customizations.user.name",
		"customizations.user.password",
		"customizations.user.shell",
		"customizations.user.uid",
		"distro",
		"enabled_modules",
		"groups",
		"modules",
		"packages",
	}
}

func TestValidateConfig(t *testing.T) {
	type testCase struct {
		supported []string
		required  []string
		bp        blueprint.Blueprint
		err       string
	}

	testCases := map[string]testCase{
		"simple": {
			// Support some options and set them
			supported: []string{
				"packages",
				"customizations",
				"customizations.kernel",
				"customizations.timezone",
				"customizations.openscap.profile_id",
				"customizations.locale.keyboard",
			},
			required: []string{"packages"},
			bp: blueprint.Blueprint{
				Packages: []blueprint.Package{
					{Name: "vim"},
				},
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Name: "kernol",
					},
					Locale: &blueprint.LocaleCustomization{
						Keyboard: common.ToPtr("us"),
					},
				},
			},
		},
		"full-array-supported": {
			supported: []string{
				"customizations.user",
			},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User: []blueprint.UserCustomization{
						{
							Name: "mario",
							Key:  common.ToPtr("ssh-key"),
						},
						{
							Name: "green-mario",
							Key:  common.ToPtr("ssh-key"),
						},
					},
				},
			},
		},
		"nothing-supported": {
			// Don't support anything and add Packages
			bp: blueprint.Blueprint{
				Packages: []blueprint.Package{
					{Name: "vim"},
				},
			},
			err: `packages: not supported`,
		},
		"category-not-supported": {
			// Support just the Locale under customizations and select Kernel
			supported: []string{
				"customizations.locale",
			},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Name: "linux",
					},
				},
			},
			err: `customizations.kernel: not supported`,
		},
		"leaf-not-supported": {
			// Support only Enabled under Services and select Disabled as well
			supported: []string{
				"customizations.services.enabled",
			},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Services: &blueprint.ServicesCustomization{
						Enabled:  []string{"good.service"},
						Disabled: []string{"bad.service"},
					},
				},
			},
			err: `customizations.services.disabled: not supported`,
		},
		"leaf-array-not-supported": {
			// Support only Mountpoint under Filesystem (an array) and select MinSize as well
			supported: []string{
				"customizations.filesystem.mountpoint",
			},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Filesystem: []blueprint.FilesystemCustomization{
						{
							Mountpoint: "/mnt/stuff",
							MinSize:    1,
						},
					},
				},
			},
			err: `customizations.filesystem[0].minsize: not supported`,
		},
		"everything-toplevel": {
			// Support all options and customizations at the top level.
			supported: []string{
				"containers",
				"customizations",
				"distro",
				"enabled_modules",
				"groups",
				"modules",
				"packages",
			},
			required: []string{},
			bp:       fullBlueprint(),
		},
		"everything-supported": {
			// Explicitly support all customizations down to each individual value.
			// Normally these can be enabled by simply enabling all the top
			// level categories, but testing the whole thing is good to make
			// sure all elements are visited in the validator.
			supported: allOptionStrings(),
			bp:        fullBlueprint(),
		},
		"missing-customizations-required": {
			supported: []string{"customizations.user"},
			// Require User and don't set anything.
			required: []string{"customizations.user"},
			err:      `customizations: required`,
		},
		"missing-users-required": {
			// Require User and set a Customization but not User.
			supported: []string{"customizations"},
			required:  []string{"customizations.user"},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Hostname: common.ToPtr("fail"),
				},
			},
			err: `customizations.user: required`,
		},
		"required-slice-leaf": {
			// Require the Name under User and set it only for one of the two.
			supported: []string{"customizations.user"},
			required:  []string{"customizations.user.name"},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User: []blueprint.UserCustomization{
						{
							Name: "user-with-name",
							Key:  common.ToPtr("ssh-key"),
						},
						{
							Key: common.ToPtr("ssh-key"),
						},
					},
				},
			},
			err: `customizations.user[1].name: required`,
		},
		"empty-slices-are-unset": {
			bp: blueprint.Blueprint{
				Packages: []blueprint.Package{},
			},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			testImage := &TestImageType{
				name:             name,
				supportedOptions: tc.supported,
				requiredOptions:  tc.required,
			}

			err := distro.ValidateConfig(testImage, tc.bp)
			if tc.err == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.err)
			}
		},
		)
	}
}

func TestValidateSupportedConfig(t *testing.T) {
	type testCase struct {
		supported []string
		config    any
		expErr    string
	}

	type pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	type user struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}

	type systemd struct {
		Enable  []string `json:"enable"`
		Disable []string `json:"disable"`
	}

	type name struct {
		Name string `json:"name"`
	}

	type custWithEmbed struct {
		Type string `json:"type"`
		name
	}

	type customizations struct {
		Users   []user        `json:"users"`
		Systemd *systemd      `json:"systemd"`
		Embed   custWithEmbed `json:"embed"`
	}

	type testConfigType struct {
		Name              string         `json:"name"`
		Enable            *bool          `json:"enable"`
		Packages          []pkg          `json:"packages"`
		InstallerPackages []pkg          `json:"installer_packages"`
		Customizations    customizations `json:"customizations"`
	}

	testCases := map[string]testCase{
		"empty": {
			supported: nil,
			config:    struct{}{},
			expErr:    "",
		},
		"simple": {
			supported: []string{
				"name",
				"packages",
				"customizations",
			},
			config: testConfigType{
				Name: "test_1",
				Packages: []pkg{
					{
						Name: "osbuild-composer",
					},
				},
			},
			expErr: "",
		},
		"nested": {
			supported: []string{
				"name",
				"packages",
				"installer_packages.name",
				"customizations.systemd",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "btrfs-tools",
					},
				},
				Customizations: customizations{
					Systemd: &systemd{
						Enable:  []string{"sshd.service", "cockpit.socket"},
						Disable: []string{"firewalld.service"},
					},
				},
			},
			expErr: "",
		},
		"nested-with-pointer": {
			supported: []string{
				"name",
				"packages",
				"installer_packages.name",
				"customizations.systemd.enable",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "btrfs-tools",
					},
				},
				Customizations: customizations{
					Systemd: &systemd{
						Enable: []string{"sshd.service", "cockpit.socket"},
					},
				},
			},
			expErr: "",
		},
		"installer-not-allowed": {
			supported: []string{
				"name",
				"packages",
				"customizations",
			},
			config: testConfigType{
				Name: "test_1",
				Packages: []pkg{
					{
						Name: "osbuild-composer",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "lvm2",
					},
				},
			},
			expErr: "installer_packages: not supported",
		},
		"enable-not-allowed": {
			supported: []string{
				"name",
				"packages",
				"customizations",
			},
			config: testConfigType{
				Name:   "test_1",
				Enable: common.ToPtr(false),
				Packages: []pkg{
					{
						Name: "osbuild-composer",
					},
				},
			},
			expErr: "enable: not supported",
		},
		"installer.version-not-allowed": {
			supported: []string{
				"name",
				"packages",
				"installer_packages.name",
				"customizations.systemd",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "btrfs-tools",
					},
					{
						Name:    "lvm2",
						Version: "22",
					},
				},
				Customizations: customizations{
					Systemd: &systemd{
						Enable:  []string{"sshd.service", "cockpit.socket"},
						Disable: []string{"firewalld.service"},
					},
				},
			},
			expErr: "installer_packages[1].version: not supported",
		},
		"customizations.user-not-supported": {
			supported: []string{
				"name",
				"packages",
				"installer_packages.name",
				"customizations.systemd",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "btrfs-tools",
					},
				},
				Customizations: customizations{
					Systemd: &systemd{
						Enable:  []string{"sshd.service", "cockpit.socket"},
						Disable: []string{"firewalld.service"},
					},
					Users: []user{
						{
							Name: "Bob",
						},
					},
				},
			},
			expErr: "customizations.users: not supported",
		},
		"customizations.embed-supported": {
			supported: []string{
				"name",
				"customizations.embed",
			},
			config: testConfigType{
				Customizations: customizations{
					Embed: custWithEmbed{
						name: name{
							Name: "embedded structure",
						},
					},
				},
			},
		},
		"customizations.embed-child-not-supported": {
			supported: []string{
				"name",
				"customizations.embed.type",
			},
			config: testConfigType{
				Customizations: customizations{
					Embed: custWithEmbed{
						name: name{
							Name: "embedded structure",
						},
					},
				},
			},
			expErr: "customizations.embed.name: not supported",
		},
		"empty-slices-are-unset": {
			config: testConfigType{
				Packages: []pkg{},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(tc.config)
			err := distro.ValidateSupportedConfig(tc.supported, v)
			if tc.expErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expErr)
			}
		})
	}
}

func TestValidateSupportedConfigTypeError(t *testing.T) {
	type testCase struct {
		supported []string
		config    any
		expErr    string
	}

	type customizations struct {
		Map map[string]string `json:"map"`
		Int int               `json:"int"`
	}

	type testConfigType struct {
		Customizations customizations `json:"customizations"`
	}

	testCases := map[string]testCase{
		"map-key-supported": {
			supported: []string{"customizations.map.somekey"},
			config: testConfigType{
				Customizations: customizations{
					Map: map[string]string{"key": "value"},
				},
			},
			expErr: "customizations.map: internal error: unexpected field type: map (map[key:value])",
		},
		"int-has-child": {
			supported: []string{"customizations.int.whatever"},
			config: testConfigType{
				Customizations: customizations{
					Int: 12,
				},
			},
			expErr: "customizations.int: internal error: supported list specifies child element of non-container type int: 12",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(tc.config)
			err := distro.ValidateSupportedConfig(tc.supported, v)
			if tc.expErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expErr)
			}
		})
	}
}

func TestValidateRequiredConfig(t *testing.T) {
	type testCase struct {
		required []string
		config   any
		expErr   string
	}

	type pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	type user struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}

	type systemd struct {
		Enable  []string `json:"enable"`
		Disable []string `json:"disable"`
	}

	type name struct {
		Name string `json:"name"`
	}

	type custWithEmbed struct {
		Type string `json:"type"`
		name
	}

	type customizations struct {
		Users   []user        `json:"users"`
		Systemd *systemd      `json:"systemd"`
		Embed   custWithEmbed `json:"embed"`
	}

	type testConfigType struct {
		Name              string         `json:"name"`
		Enable            *bool          `json:"enable"`
		Packages          []pkg          `json:"packages"`
		InstallerPackages []pkg          `json:"installer_packages"`
		Customizations    customizations `json:"customizations"`
	}

	testCases := map[string]testCase{
		"empty": {
			required: nil,
			config:   struct{}{},
			expErr:   "",
		},
		"simple": {
			required: []string{
				"name",
				"packages",
			},
			config: testConfigType{
				Name: "test_1",
				Packages: []pkg{
					{
						Name: "osbuild-composer",
					},
				},
			},
			expErr: "",
		},
		"nested": {
			required: []string{
				"name",
				"packages",
				"installer_packages.name",
				"customizations.systemd",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "btrfs-tools",
					},
				},
				Customizations: customizations{
					Systemd: &systemd{
						Enable:  []string{"sshd.service", "cockpit.socket"},
						Disable: []string{"firewalld.service"},
					},
				},
			},
			expErr: "",
		},
		"nested-pointer": {
			required: []string{
				"name",
				"packages",
				"installer_packages.name",
				"customizations.systemd.enable",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "btrfs-tools",
					},
				},
				Customizations: customizations{
					Systemd: &systemd{
						Enable:  []string{"sshd.service", "cockpit.socket"},
						Disable: []string{"firewalld.service"},
					},
				},
			},
			expErr: "",
		},
		"customizations-required": {
			required: []string{
				"name",
				"packages",
				"customizations",
			},
			config: testConfigType{
				Name: "test_1",
				Packages: []pkg{
					{
						Name: "osbuild-composer",
					},
				},
				InstallerPackages: []pkg{
					{
						Name: "lvm2",
					},
				},
			},
			expErr: "customizations: required",
		},
		"name-required": {
			required: []string{
				"name",
				"packages",
			},
			config: testConfigType{
				Packages: []pkg{
					{
						Name: "osbuild-composer",
					},
				},
			},
			expErr: "name: required",
		},
		"user.name-required": {
			required: []string{
				"name",
				"packages",
				"customizations.users.name",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				Customizations: customizations{
					Users: []user{
						{
							Name:     "me",
							Password: "moi",
						},
						{
							Password: "I have no name but I must pass",
						},
					},
				},
			},
			expErr: "customizations.users[1].name: required",
		},
		"customizations.systemd-required": {
			required: []string{
				"name",
				"packages",
				"customizations.systemd",
			},
			config: testConfigType{
				Name: "test_2",
				Packages: []pkg{
					{
						Name:    "osbuild",
						Version: "100",
					},
				},
				Customizations: customizations{
					Users: []user{
						{
							Name: "Bob",
						},
					},
				},
			},
			expErr: "customizations.systemd: required",
		},
		"required-does-not-exist": {
			required: []string{
				"customizations.does-not-exist",
			},
			config: testConfigType{
				Customizations: customizations{
					Users: []user{
						{
							Name: "Bob",
						},
					},
				},
			},
			expErr: "customizations.does-not-exist: customizations does not have a field with JSON tag \"does-not-exist\"",
		},
		"customizations.embed-required-ok": {
			required: []string{
				"customizations.embed",
			},
			config: testConfigType{
				Customizations: customizations{
					Embed: custWithEmbed{
						name: name{
							Name: "embedded structure",
						},
					},
				},
			},
		},
		"customizations.embed-required-error": {
			required: []string{
				"customizations.embed.name",
			},
			config: testConfigType{
				Customizations: customizations{
					Embed: custWithEmbed{
						Type: "t",
					},
				},
			},
			expErr: "customizations.embed.name: required",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(tc.config)
			err := distro.ValidateRequiredConfig(tc.required, v)
			if tc.expErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expErr)
			}
		})
	}
}

func TestValidateRequiredConfigTypeError(t *testing.T) {
	type testCase struct {
		required []string
		config   any
		expErr   string
	}

	type customizations struct {
		Map map[string]string `json:"map"`
		Str string            `json:"str"`
	}

	type testConfigType struct {
		Customizations customizations `json:"customizations"`
	}

	testCases := map[string]testCase{
		"map-key-required": {
			required: []string{"customizations.map.somekey"},
			config: testConfigType{
				Customizations: customizations{
					Map: map[string]string{"key": "value"},
				},
			},
			expErr: "customizations.map: field of type map cannot be marked required",
		},
		"string-has-child": {
			required: []string{"customizations.str.whatever"},
			config: testConfigType{
				Customizations: customizations{
					Str: "hello",
				},
			},
			expErr: "customizations.str: internal error: required list specifies child element of non-container type string: hello",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(tc.config)
			err := distro.ValidateRequiredConfig(tc.required, v)
			if tc.expErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expErr)
			}
		})
	}
}
