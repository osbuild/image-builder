package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk/partition"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/generic"
	"github.com/osbuild/images/pkg/ostree"
)

func TestCheckOptions(t *testing.T) {
	type testCase struct {
		arch    string // for when it's relevant; if empty, defaults to x86_64
		distro  string
		it      string
		bp      blueprint.Blueprint
		options distro.ImageOptions
		expErr  string
	}

	testCases := map[string]testCase{
		"f42/ami-ok": {
			distro:  "fedora-42",
			it:      "generic-ami",
			bp:      blueprint.Blueprint{},
			options: distro.ImageOptions{},
			expErr:  "",
		},
		"f42/ami-installer-error": {
			distro: "fedora-42",
			it:     "generic-ami",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			expErr: "blueprint validation failed for image type \"generic-ami\": customizations.installer: not supported",
		},
		"f42/ami-ostree-error": {
			distro: "fedora-42",
			it:     "generic-ami",
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "OSTree is not supported for \"generic-ami\"",
		},
		"f42/ostree-disk-supported": {
			distro: "fedora-42",
			it:     "iot-qcow2",
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					Files: []blueprint.FileCustomization{{
						Path: "/etc/osbuild/stamp",
						Data: "Created by osbuild",
					}},
					Directories: []blueprint.DirectoryCustomization{{
						Path: "/etc/osbuild",
					}},
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
					FIPS: common.ToPtr(true),
				},
			},
		},
		"f42/ostree-disk-not-supported": {
			distro: "fedora-42",
			it:     "iot-qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					Files: []blueprint.FileCustomization{{
						Path: "/etc/osbuild/stamp",
						Data: "Created by osbuild",
					}},
					Directories: []blueprint.DirectoryCustomization{{
						Path: "/etc/osbuild",
					}},
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
					FIPS: common.ToPtr(true),
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-qcow2\": customizations.kernel.name: not supported",
		},
		"f42/iot-simplified-requires-install-device": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"f42/iot-simplified-requires-install-device-error": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": customizations.installation_device: required",
		},
		"f42/iot-simplified-supported-customizations": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure:     "true",
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"f42/iot-simplified-unsupported-customizations": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure:     "true",
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": customizations.services: not supported",
		},
		"f42/iot-simplified-fdo-requires-manufacturing-url": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure: "true",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": customizations.fdo.manufacturing_server_url: required when using fdo",
		},
		"f42/iot-simplified-fdo-requires-a-diun-option": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo",
		},
		"f42/iot-simplified-fdo-requires-exactly-one-diun-option": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						ManufacturingServerURL: "https://example.com/fdo",
						DiunPubKeyInsecure:     "true",
						DiunPubKeyHash:         "ffff",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo",
		},
		"f42/iot-simplified-ignition": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"f42/iot-simplified-ignition-no-provisioning-url": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": customizations.ignition.firstboot requires customizations.ignition.firstboot.provisioning_url",
		},
		"f42/iot-simplified-ignition-option-conflict": {
			distro: "fedora-42",
			it:     "iot-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					Ignition: &blueprint.IgnitionCustomization{
						Embedded: &blueprint.EmbeddedIgnitionCustomization{
							Config: "/ignition.cfg",
						},
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-simplified-installer\": customizations.ignition.embedded cannot be used with customizations.ignition.firstboot",
		},

		"f42/iot-installer-supported-customizations": {
			distro: "fedora-42",
			it:     "iot-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Timezone: &blueprint.TimezoneCustomization{
						Timezone: common.ToPtr("UTC"),
					},
					Locale: &blueprint.LocaleCustomization{
						Languages: []string{"en_GB.UTF-8"},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"f42/iot-installer-unsupported-customizations": {
			distro: "fedora-42",
			it:     "iot-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Timezone: &blueprint.TimezoneCustomization{
						Timezone: common.ToPtr("UTC"),
					},
					Locale: &blueprint.LocaleCustomization{
						Languages: []string{"en_GB.UTF-8"},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-installer\": customizations.kernel: not supported",
		},

		"f42/live-installer-no-installer-customizations": {
			distro: "fedora-42",
			it:     "workstation-live-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
		},
		"f42/live-installer-unsupported-customizations": {
			distro: "fedora-42",
			it:     "workstation-live-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User: []blueprint.UserCustomization{{Name: "root"}},
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			expErr: "blueprint validation failed for image type \"workstation-live-installer\": customizations.user: not supported",
		},

		"f42/ostree-types-no-oscap": {
			distro: "fedora-42",
			it:     "iot-container",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "xccdf_org.ssgproject.content_profile_ospp",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"iot-container\": customizations.openscap: not supported",
		},

		"f42/iot-installer-installer-customizations": {
			distro: "fedora-42",
			it:     "iot-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"f42/iot-installer-bad-combinations": {
			distro: "fedora-42",
			it:     "iot-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User: []blueprint.UserCustomization{{Name: "root"}},
					Installer: &blueprint.InstallerCustomization{
						Kickstart: &blueprint.Kickstart{
							Contents: "echo 'Testing'",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-installer\": customizations.installer.kickstart.contents cannot be used with customizations.user or customizations.group",
		},

		"f42/ostree-disk-unsupported-containers": {
			distro: "fedora-42",
			it:     "iot-qcow2",
			bp: blueprint.Blueprint{
				Containers: []blueprint.Container{
					{
						Source: "example.org/containers/test:42",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"iot-qcow2\": containers: not supported",
		},

		"f42/ostree-commit-unsupported-kernel-append": {
			distro: "fedora-42",
			it:     "iot-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Append: "debug",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"iot-commit\": customizations.kernel.append: not supported",
		},

		"f42/oscap-empty-profile": {
			distro: "fedora-42",
			it:     "vhd",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"generic-vhd\": customizations.openscap.profile_id: required when using customizations.openscap",
		},
		"f42/disk-and-filesystems": {
			distro: "fedora-42",
			it:     "generic-qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Filesystem: []blueprint.FilesystemCustomization{
						{
							MinSize:    1024,
							Mountpoint: "/home",
						},
					},
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									Label:      "root",
									FSType:     "ext4",
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"generic-qcow2\": customizations.disk cannot be used with customizations.filesystem",
		},
		"f42/bad-filesystem-mountpoint": {
			distro: "fedora-42",
			it:     "generic-qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Filesystem: []blueprint.FilesystemCustomization{
						{
							MinSize:    1024,
							Mountpoint: "/etc",
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"generic-qcow2\": The following custom mountpoints are not supported [\"/etc\"]",
		},
		"f42/bad-disk-mountpoint": {
			distro: "fedora-42",
			it:     "generic-qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/etc",
									FSType:     "ext4",
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"generic-qcow2\": The following errors occurred while setting up custom mountpoints:\npath \"/etc\" is not allowed",
		},
		"f42/two-lvm+btrfs": {
			distro: "fedora-42",
			it:     "generic-qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "lvm",
								VGCustomization: blueprint.VGCustomization{
									LogicalVolumes: []blueprint.LVCustomization{
										{
											Name: "lv1",
											FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
												Mountpoint: "/data",
												FSType:     "ext4",
											},
										},
									},
								},
							},
							{
								Type: "btrfs",
								BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
									Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
										{
											Name:       "b1",
											Mountpoint: "/stuff",
										},
									},
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"generic-qcow2\": btrfs and lvm partitioning cannot be combined",
		},

		"r8/ami-ok": {
			distro:  "rhel-8.10",
			it:      "ami",
			bp:      blueprint.Blueprint{},
			options: distro.ImageOptions{},
			expErr:  "",
		},
		"r8/ami-installer-error": {
			distro: "rhel-8.10",
			it:     "ami",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			expErr: "blueprint validation failed for image type \"ami\": customizations.installer: not supported",
		},
		"r8/ami-ostree-error": {
			distro: "rhel-8.10",
			it:     "ami",
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "OSTree is not supported for \"ami\"",
		},
		"r8/ostree-installer-requires-ostree-url": {
			distro: "rhel-8.10",
			it:     "edge-installer",
			expErr: "options validation failed for image type \"edge-installer\": ostree.url: required, there is no default available",
		},
		"r8/ostree-disk-supported": {
			distro: "rhel-8.10",
			it:     "edge-raw-image",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r8/ostree-disk-not-supported": {
			distro: "rhel-8.10",
			it:     "edge-raw-image",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					Files: []blueprint.FileCustomization{{
						Path: "/etc/osbuild/stamp",
						Data: "Created by osbuild",
					}},
					Directories: []blueprint.DirectoryCustomization{{
						Path: "/etc/osbuild",
					}},
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
					FIPS: common.ToPtr(true),
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-raw-image\": customizations.kernel.name: not supported",
		},
		"r8/edge-simplified-requires-install-device": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r8/edge-simplified-requires-install-device-error": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.installation_device: required",
		},
		"r8/edge-simplified-supported-customizations": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure:     "true",
						ManufacturingServerURL: "https://example.com/fdo",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r8/edge-simplified-unsupported-customizations": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure:     "true",
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.services: not supported",
		},
		"r8/edge-simplified-fdo-requires-manufacturing-url": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure: "true",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.fdo.manufacturing_server_url: required when using fdo",
		},
		"r8/edge-simplified-fdo-requires-a-diun-option": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						ManufacturingServerURL: "https://example.com/fdo",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo",
		},
		"r8/edge-simplified-fdo-requires-exactly-one-diun-option": {
			distro: "rhel-8.10",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						ManufacturingServerURL: "https://example.com/fdo",
						DiunPubKeyInsecure:     "true",
						DiunPubKeyHash:         "ffff",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo",
		},

		"r8/edge-installer-supported-customizations": {
			distro: "rhel-8.10",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Timezone: &blueprint.TimezoneCustomization{
						Timezone: common.ToPtr("UTC"),
					},
					Locale: &blueprint.LocaleCustomization{
						Languages: []string{"en_GB.UTF-8"},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r8/edge-installer-unsupported-customizations": {
			distro: "rhel-8.10",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Timezone: &blueprint.TimezoneCustomization{
						Timezone: common.ToPtr("UTC"),
					},
					Locale: &blueprint.LocaleCustomization{
						Languages: []string{"en_GB.UTF-8"},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-installer\": customizations.kernel: not supported",
		},

		"r8/ostree-types-no-oscap": {
			distro: "rhel-8.10",
			it:     "edge-container",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "xccdf_org.ssgproject.content_profile_ospp",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-container\": customizations.openscap: not supported",
		},

		"r8/edge-installer-installer-customizations": {
			distro: "rhel-8.10",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r8/edge-installer-bad-combinations": {
			distro: "rhel-8.10",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User: []blueprint.UserCustomization{{Name: "root"}},
					Installer: &blueprint.InstallerCustomization{
						Kickstart: &blueprint.Kickstart{
							Contents: "echo 'Testing'",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-installer\": customizations.installer.kickstart.contents cannot be used with customizations.user or customizations.group",
		},

		"r8/ostree-disk-requires-ostree-url": {
			distro: "rhel-8.10",
			it:     "edge-raw-image",
			expErr: "options validation failed for image type \"edge-raw-image\": ostree.url: required, there is no default available",
		},

		"r8/ostree-no-containers": {
			distro: "rhel-8.10",
			it:     "edge-raw-image",
			bp: blueprint.Blueprint{
				Containers: []blueprint.Container{
					{
						Source: "example.org/containers/test:42",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-raw-image\": containers: not supported",
		},

		"r8/ostree-commit-unsupported-kernel-append": {
			distro: "rhel-8.10",
			it:     "edge-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Append: "debug",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-commit\": customizations.kernel.append: not supported",
		},

		"r8/ostree-mountpoints-not-supported": {
			distro: "rhel-8.10",
			it:     "edge-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Filesystem: []blueprint.FilesystemCustomization{
						{
							Mountpoint: "/data",
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-commit\": customizations.filesystem: not supported",
		},

		"r8/ostree-partitioning-not-supported": {
			distro: "rhel-8.10",
			it:     "edge-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/data",
									FSType:     "ext4",
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-commit\": customizations.disk: not supported",
		},

		"r8/oscap-empty-profile": {
			distro: "rhel-8.10",
			it:     "vhd",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"vhd\": customizations.openscap.profile_id: required when using customizations.openscap",
		},
		"r8/btrfs-mode-unsupported": {
			distro: "rhel-8.10",
			it:     "edge-raw-image",
			options: distro.ImageOptions{
				PartitioningMode: partition.BtrfsPartitioningMode,
			},
			expErr: "partitioning mode btrfs not supported for \"edge-raw-image\"",
		},
		"r8/aarch-swap-partition-not-supported": {
			distro: "rhel-8.10",
			it:     "qcow2",
			arch:   "aarch64",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									Label:      "root",
									FSType:     "ext4",
								},
							},
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									FSType: "swap",
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"qcow2\": customizations.disk: swap partition creation is not supported on rhel-8.10 aarch64",
		},
		"r8/aarch-swap-lv-not-supported": {
			distro: "rhel-8.10",
			it:     "qcow2",
			arch:   "aarch64",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "lvm",
								VGCustomization: blueprint.VGCustomization{
									LogicalVolumes: []blueprint.LVCustomization{
										{
											FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
												Mountpoint: "/",
												Label:      "root",
												FSType:     "ext4",
											},
										},
										{
											FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
												FSType: "swap",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"qcow2\": customizations.disk: swap logical volume creation is not supported on rhel-8.10 aarch64",
		},
		"r8/oscap-8.6-unsupported": {
			distro: "rhel-8.6",
			it:     "ami",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						// must be a valid ID, otherwise it will return the
						// invalid profile ID error from checkOptionsCommon()
						ProfileID: "xccdf_org.ssgproject.content_profile_stig",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"ami\": customizations.openscap: not supported for distro version: 8.6",
		},

		"r9/ami-ok": {
			distro:  "rhel-9.7",
			it:      "ami",
			bp:      blueprint.Blueprint{},
			options: distro.ImageOptions{},
			expErr:  "",
		},
		"r9/ami-installer-error": {
			distro: "rhel-9.7",
			it:     "ami",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			expErr: "blueprint validation failed for image type \"ami\": customizations.installer: not supported",
		},
		"r9/ami-ostree-error": {
			distro: "rhel-9.7",
			it:     "ami",
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "OSTree is not supported for \"ami\"",
		},
		"r9/ostree-installer-requires-ostree-url": {
			distro: "rhel-9.7",
			it:     "edge-installer",
			expErr: "options validation failed for image type \"edge-installer\": ostree.url: required, there is no default available",
		},
		"r9/ostree-disk-supported": {
			distro: "rhel-9.7",
			it:     "edge-raw-image",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r9/ostree-disk-not-supported": {
			distro: "rhel-9.7",
			it:     "edge-raw-image",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					Files: []blueprint.FileCustomization{{
						Path: "/etc/osbuild/stamp",
						Data: "Created by osbuild",
					}},
					Directories: []blueprint.DirectoryCustomization{{
						Path: "/etc/osbuild",
					}},
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
					FIPS: common.ToPtr(true),
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-raw-image\": customizations.kernel.name: not supported",
		},
		"r9/edge-simplified-requires-install-device": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r9/edge-simplified-requires-install-device-error": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.installation_device: required",
		},
		"r9/edge-simplified-supported-customizations": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure:     "true",
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r9/edge-simplified-unsupported-customizations": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure:     "true",
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Services: &blueprint.ServicesCustomization{
						Disabled: []string{"sshd.service"},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.services: not supported",
		},
		"r9/edge-simplified-fdo-does-not-require-manufacturing-url": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						DiunPubKeyInsecure: "true",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.fdo.manufacturing_server_url: required when using fdo",
		},
		"r9/edge-simplified-fdo-requires-a-diun-option": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						ManufacturingServerURL: "https://example.com/fdo",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo",
		},
		"r9/edge-simplified-fdo-requires-exactly-one-diun-option": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					FDO: &blueprint.FDOCustomization{
						ManufacturingServerURL: "https://example.com/fdo",
						DiunPubKeyInsecure:     "true",
						DiunPubKeyHash:         "ffff",
					},
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-debug",
					},
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo",
		},
		"r9/edge-simplified-ignition": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r9/edge-simplified-ignition-no-provisioning-url": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					Ignition: &blueprint.IgnitionCustomization{
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.ignition.firstboot requires customizations.ignition.firstboot.provisioning_url",
		},
		"r9/edge-simplified-ignition-option-conflict": {
			distro: "rhel-9.7",
			it:     "edge-simplified-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					InstallationDevice: "/dev/null",
					Ignition: &blueprint.IgnitionCustomization{
						Embedded: &blueprint.EmbeddedIgnitionCustomization{
							Config: "/ignition.cfg",
						},
						FirstBoot: &blueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "https://example.com/provision",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-simplified-installer\": customizations.ignition.embedded cannot be used with customizations.ignition.firstboot",
		},

		"r9/edge-installer-supported-customizations": {
			distro: "rhel-9.7",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Timezone: &blueprint.TimezoneCustomization{
						Timezone: common.ToPtr("UTC"),
					},
					Locale: &blueprint.LocaleCustomization{
						Languages: []string{"en_GB.UTF-8"},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r9/edge-installer-unsupported-customizations": {
			distro: "rhel-9.7",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User:  []blueprint.UserCustomization{{Name: "root"}},
					Group: []blueprint.GroupCustomization{{Name: "admins"}},
					FIPS:  common.ToPtr(true),
					Timezone: &blueprint.TimezoneCustomization{
						Timezone: common.ToPtr("UTC"),
					},
					Locale: &blueprint.LocaleCustomization{
						Languages: []string{"en_GB.UTF-8"},
					},
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-installer\": customizations.kernel: not supported",
		},

		"r9/ostree-types-no-oscap": {
			distro: "rhel-9.7",
			it:     "edge-container",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "xccdf_org.ssgproject.content_profile_ospp",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-container\": customizations.openscap: not supported",
		},

		"r9/oscap-empty-profile": {
			distro: "rhel-9.7",
			it:     "vhd",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"vhd\": customizations.openscap.profile_id: required when using customizations.openscap",
		},

		"r9/edge-installer-installer-customizations": {
			distro: "rhel-9.7",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},
		"r9/edge-installer-bad-combinations": {
			distro: "rhel-9.7",
			it:     "edge-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					User: []blueprint.UserCustomization{{Name: "root"}},
					Installer: &blueprint.InstallerCustomization{
						Kickstart: &blueprint.Kickstart{
							Contents: "echo 'Testing'",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-installer\": customizations.installer.kickstart.contents cannot be used with customizations.user or customizations.group",
		},

		"r9/ostree-disk-unsupported-containers": {
			distro: "rhel-9.7",
			it:     "edge-ami",
			bp: blueprint.Blueprint{
				Containers: []blueprint.Container{
					{
						Source: "example.org/containers/test:42",
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "blueprint validation failed for image type \"edge-ami\": containers: not supported",
		},

		"r9/ostree-commit-unsupported-kernel-append": {
			distro: "rhel-9.7",
			it:     "edge-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Append: "debug",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-commit\": customizations.kernel.append: not supported",
		},

		"r9/ostree-disk-requires-ostree-url": {
			distro: "rhel-9.7",
			it:     "edge-vsphere",
			expErr: "options validation failed for image type \"edge-vsphere\": ostree.url: required, there is no default available",
		},
		"r9/ostree-disk2-requires-ostree-url": {
			distro: "rhel-9.7",
			it:     "edge-ami",
			expErr: "options validation failed for image type \"edge-ami\": ostree.url: required, there is no default available",
		},

		"r9/ostree-mountpoints-not-supported": {
			distro: "rhel-9.7",
			it:     "edge-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Filesystem: []blueprint.FilesystemCustomization{
						{
							Mountpoint: "/data",
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-commit\": customizations.filesystem: not supported",
		},

		"r9/ostree-partitioning-not-supported": {
			distro: "rhel-9.7",
			it:     "edge-commit",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/data",
									FSType:     "ext4",
								},
							},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"edge-commit\": customizations.disk: not supported",
		},

		"r9/ostree-disk-mountpoints-supported": {
			distro: "rhel-9.7",
			it:     "edge-vsphere",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Filesystem: []blueprint.FilesystemCustomization{
						{
							Mountpoint: "/data",
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},

		"r9/ostree-disk-partitioning-supported": {
			distro: "rhel-9.7",
			it:     "edge-vsphere",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "plain",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/data",
									FSType:     "ext4",
								},
							},
						},
					},
				},
			},
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
		},

		"r9/cvm-kernel-unsupported": {
			distro: "rhel-9.7",
			it:     "azure-cvm",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"azure-cvm\": customizations.kernel: not supported",
		},

		"r9/bad-ostree-ref": {
			distro: "rhel-9.4",
			it:     "edge-commit",
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					ImageRef: "-bad-ref",
				},
			},
			expErr: "invalid ostree image ref \"-bad-ref\"",
		},

		"r9/oscap-9.0-unsupported": {
			distro: "rhel-9.0",
			it:     "ami",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						// must be a valid ID, otherwise it will return the
						// invalid profile ID error from checkOptionsCommon()
						ProfileID: "xccdf_org.ssgproject.content_profile_stig",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"ami\": customizations.openscap: not supported for distro version: 9.0",
		},

		"r10/ami-ok": {
			distro:  "rhel-10.0",
			it:      "ami",
			bp:      blueprint.Blueprint{},
			options: distro.ImageOptions{},
			expErr:  "",
		},
		"r10/ami-installer-error": {
			distro: "rhel-10.0",
			it:     "ami",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
					},
				},
			},
			expErr: "blueprint validation failed for image type \"ami\": customizations.installer: not supported",
		},
		"r10/ami-ostree-error": {
			distro: "rhel-10.0",
			it:     "ami",
			options: distro.ImageOptions{
				OSTree: &ostree.ImageOptions{
					URL: "https://example.org/repo",
				},
			},
			expErr: "OSTree is not supported for \"ami\"",
		},

		"r10/oscap-empty-profile": {
			distro: "rhel-10.0",
			it:     "vhd",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"vhd\": customizations.openscap.profile_id: required when using customizations.openscap",
		},

		"r10/cvm-kernel-unsupported": {
			distro: "rhel-10.0",
			it:     "azure-cvm",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Name: "kernel-rt",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"azure-cvm\": customizations.kernel: not supported",
		},

		"r10/bad-partitioning": {
			distro: "rhel-10.0",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Disk: &blueprint.DiskCustomization{
						Partitions: []blueprint.PartitionCustomization{
							{
								Type: "wrong",
							},
						},
					},
				},
			},
			expErr: "invalid partitioning customizations:\nunknown partition type: wrong",
		},
		"r10/unsupported-oscap-policy": {
			distro: "rhel-10.1",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "unsupported-profile",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"qcow2\": customizations.openscap.profile_id: unsupported profile unsupported-profile",
		},
		"r10/duplicate-file-customization": {
			distro: "rhel-10.1",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Files: []blueprint.FileCustomization{
						{
							Path: "/file1",
						},
						{
							Path: "/file1",
						},
					},
				},
			},
			expErr: "duplicate files / directory customization paths: [/file1]",
		},
		"r10/bad-path-for-file-customization": {
			distro: "rhel-10.1",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Files: []blueprint.FileCustomization{
						{
							Path: "/bin/bin",
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"qcow2\": the following custom files are not allowed: [\"/bin/bin\"]",
		},
		"r10/bad-path-for-dir-customization": {
			distro: "rhel-10.1",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Directories: []blueprint.DirectoryCustomization{
						{
							Path: "/bin",
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"qcow2\": the following custom directories are not allowed: [\"/bin\"]",
		},
		"r10/bad-repo-customization": {
			distro: "rhel-10.1",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Repositories: []blueprint.RepositoryCustomization{
						{
							// Invalid: requires ID
							BaseURLs: []string{"https://example.org/repo"},
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"qcow2\": Repository ID is required",
		},
		"r10/bad-installer-combinations": {
			distro: "rhel-10.1",
			it:     "image-installer",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Installer: &blueprint.InstallerCustomization{
						Unattended: true,
						Kickstart: &blueprint.Kickstart{
							Contents: "echo 'bork'",
						},
					},
				},
			},
			expErr: "blueprint validation failed for image type \"image-installer\": installer.unattended is not supported when adding custom kickstart contents",
		},

		"r7/ok": {
			distro:  "rhel-7.9",
			it:      "qcow2",
			bp:      blueprint.Blueprint{},
			options: distro.ImageOptions{},
			expErr:  "",
		},

		"r7/no-containers": {
			distro: "rhel-7.9",
			it:     "azure-rhui",
			bp: blueprint.Blueprint{
				Containers: []blueprint.Container{
					{
						Name: "example.org/containers/some-kind-of-image:100",
					},
				},
			},
			expErr: "blueprint validation failed for image type \"azure-rhui\": containers: not supported",
		},

		"r7/oscap-empty-profile": {
			distro: "rhel-7.9",
			it:     "qcow2",
			bp: blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					OpenSCAP: &blueprint.OpenSCAPCustomization{
						ProfileID: "xccdf_org.ssgproject.content_profile_ospp",
					},
				},
			},
			// NOTE (validation-warnings): temporary change in error message due to change from errors to warnings in distro.ValidateConfig()
			expErr: "blueprint validation failed for image type \"qcow2\": customizations.openscap.profile_id: unsupported profile xccdf_org.ssgproject.content_profile_ospp",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			d := generic.DistroFactory(tc.distro)
			archName := tc.arch
			if archName == "" {
				archName = "x86_64"
			}
			arch, err := d.GetArch(archName)
			assert.NoError(err)
			it, err := arch.GetImageType(tc.it)
			assert.NoError(err)

			genit, ok := it.(*generic.ImageType) // checkOptions() function is defined on generic.ImageType
			assert.True(ok, "image type %q for distro %q does not appear to be valid", tc.it, d.Name())
			warnings, err := generic.ImageTypeCheckOptions(genit, &tc.bp, tc.options)
			if tc.expErr == "" {
				assert.NoError(err)
			} else {
				// NOTE (validation-warnings): errors from distro.ValidateConfig() have been temporarily converted to warnings.
				// If we don't get an error, assume the expected error is in the warnings and check for that.
				if err == nil {
					assert.Contains(warnings, tc.expErr)
				} else {
					assert.EqualError(err, tc.expErr)
				}
			}
		})
	}
}
