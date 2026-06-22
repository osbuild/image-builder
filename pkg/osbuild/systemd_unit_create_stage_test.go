package osbuild

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/internal/testdisk"
	"github.com/osbuild/image-builder/v73/pkg/disk"
	"github.com/stretchr/testify/assert"
)

func createSystemdUnit() SystemdUnit {

	var unit = UnitSection{
		Description:              "Create directory and files",
		DefaultDependencies:      common.ToPtr(true),
		ConditionPathExists:      []string{"!/etc/myfile"},
		ConditionPathIsDirectory: []string{"!/etc/mydir"},
		Requires:                 []string{"dbus.service", "libvirtd.service"},
		Wants:                    []string{"local-fs.target"},
	}
	var service = ServiceSection{
		Type:            OneshotServiceType,
		RemainAfterExit: true,
		ExecStartPre:    []string{"echo creating_files"},
		ExecStopPost:    []string{"echo done_creating_files"},
		ExecStart:       []string{"mkdir -p /etc/mydir", "touch /etc/myfiles"},
	}

	var install = InstallSection{
		RequiredBy: []string{"multi-user.target", "boot-complete.target"},
		WantedBy:   []string{"sshd.service"},
	}

	var systemdUnit = SystemdUnit{
		Unit:    &unit,
		Service: &service,
		Install: &install,
	}

	return systemdUnit
}

func TestNewSystemdUnitCreateStage(t *testing.T) {
	systemdServiceConfig := createSystemdUnit()
	var options = SystemdUnitCreateStageOptions{
		Filename: "create-dir-files.service",
		Config:   systemdServiceConfig,
	}
	expectedStage := &Stage{
		Type:    "org.osbuild.systemd.unit.create",
		Options: &options,
	}

	actualStage := NewSystemdUnitCreateStage(&options)
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewSystemdUnitCreateStageInEtc(t *testing.T) {
	systemdServiceConfig := createSystemdUnit()
	var options = SystemdUnitCreateStageOptions{
		Filename: "create-dir-files.service",
		Config:   systemdServiceConfig,
		UnitPath: EtcUnitPath,
		UnitType: GlobalUnitType,
	}
	expectedStage := &Stage{
		Type:    "org.osbuild.systemd.unit.create",
		Options: &options,
	}

	actualStage := NewSystemdUnitCreateStage(&options)
	assert.Equal(t, expectedStage, actualStage)
}

func TestSystemdUnitStageOptionsValidation(t *testing.T) {
	unitSection := &UnitSection{
		Description:         "test-mount",
		DefaultDependencies: common.ToPtr(true),
	}
	mountSection := &MountSection{
		What:    "/dev/test",
		Where:   "/test",
		Type:    "ext4",
		Options: "defaults",
	}
	installSection := &InstallSection{
		WantedBy: []string{"multi-user.target"},
	}
	serviceSection := &ServiceSection{
		Type:            "oneshot",
		RemainAfterExit: true,
		ExecStart:       []string{"true"},
	}
	socketSection := &SocketSection{
		ListenStream: "/run/test/api.socket",
		SocketGroup:  "testgroup",
		SocketMode:   "660",
	}

	type testCase struct {
		options  SystemdUnitCreateStageOptions
		expected error
	}

	testCases := map[string]testCase{
		// OK
		"service-ok": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Service: serviceSection,
					Install: installSection,
				},
			},
			expected: nil,
		},
		"mount-ok": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.mount",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Mount:   mountSection,
					Install: installSection,
				},
			},
			expected: nil,
		},
		"socket-ok": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.socket",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Install: installSection,
					Socket:  socketSection,
				},
			},
			expected: nil,
		},
		"standard-output-ok-j+c": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Service: &ServiceSection{
						ExecStart:      []string{"true"},
						StandardOutput: "journal+console",
					},
					Install: installSection,
				},
			},
			expected: nil,
		},
		"standard-output-ok-append": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Service: &ServiceSection{
						ExecStart:      []string{"true"},
						StandardOutput: "append:/var/log/test.log",
					},
					Install: installSection,
				},
			},
			expected: nil,
		},

		// missing required section
		"service-no-Service": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd service unit "test.service" requires a Service section`),
		},
		"service-no-Install": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Service: serviceSection,
				},
			},
			expected: fmt.Errorf(`systemd service unit "test.service" requires an Install section`),
		},
		"mount-no-Mount": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.mount",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd mount unit "test.mount" requires a Mount section`),
		},
		"socket-no-Socket": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.socket",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd socket unit "test.socket" requires a Socket section`),
		},

		// incorrect section for type
		"service-with-mount": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Service: serviceSection,
					Mount:   mountSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd service unit "test.service" contains invalid section Mount`),
		},
		"service-with-socket": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Service: serviceSection,
					Socket:  socketSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd service unit "test.service" contains invalid section Socket`),
		},
		"mount-with-service": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.mount",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Mount:   mountSection,
					Service: serviceSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd mount unit "test.mount" contains invalid section Service`),
		},
		"mount-with-socket": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.mount",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Mount:   mountSection,
					Socket:  socketSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`systemd mount unit "test.mount" contains invalid section Socket`),
		},
		"socket-with-Service": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.socket",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Install: installSection,
					Socket:  socketSection,
					Service: serviceSection,
				},
			},
			expected: fmt.Errorf(`systemd socket unit "test.socket" contains invalid section Service`),
		},
		"socket-with-Mount": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.socket",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Install: installSection,
					Socket:  socketSection,
					Mount:   mountSection,
				},
			},
			expected: fmt.Errorf(`systemd socket unit "test.socket" contains invalid section Mount`),
		},

		// bad filename
		"bad-filename": {
			options: SystemdUnitCreateStageOptions{
				Filename: "//not-a-good-path//",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit:    unitSection,
					Service: serviceSection,
					Install: installSection,
				},
			},
			expected: fmt.Errorf("invalid filename \"//not-a-good-path//\" for systemd unit: does not conform to schema (%s)", unitFilenameRegex),
		},

		// bad extension
		"bad-extension": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.whatever",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
			},
			expected: fmt.Errorf("invalid filename \"test.whatever\" for systemd unit: does not conform to schema (%s)", unitFilenameRegex),
		},

		// missing required options
		"mount-no-what": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.mount",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Mount: &MountSection{
						Where:   "/test",
						Type:    "ext4",
						Options: "defaults",
					},
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`What option for Mount section of systemd unit "test.mount" is required`),
		},
		"mount-no-where": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.mount",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Mount: &MountSection{
						What:    "/dev/test",
						Type:    "ext4",
						Options: "defaults",
					},
					Install: installSection,
				},
			},
			expected: fmt.Errorf(`Where option for Mount section of systemd unit "test.mount" is required`),
		},

		// invalid values
		"service-bad-env-vars": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Service: &ServiceSection{

						Type:            "oneshot",
						RemainAfterExit: true,
						ExecStart:       []string{"true"},
						Environment: []EnvironmentVariable{
							{
								Key:   ":bad_var/",
								Value: "can-be-whatever",
							},
						},
					},
					Install: installSection,
				},
			},
			expected: fmt.Errorf("variable name \":bad_var/\" does not conform to schema (%s)", envVarRegex),
		},
		"invalid-standard-output-1": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Service: &ServiceSection{
						ExecStart:      []string{"true"},
						StandardOutput: "very-invalid",
					},
					Install: installSection,
				},
			},
			expected: fmt.Errorf("StandardOutput value \"very-invalid\" does not conform to schema (%s)", systemdStandardOutputRegex),
		},
		"invalid-standard-output-2": {
			options: SystemdUnitCreateStageOptions{
				Filename: "test.service",
				UnitType: GlobalUnitType,
				UnitPath: EtcUnitPath,
				Config: SystemdUnit{
					Unit: unitSection,
					Service: &ServiceSection{
						ExecStart:      []string{"true"},
						StandardOutput: "file:",
					},
					Install: installSection,
				},
			},
			expected: fmt.Errorf("StandardOutput value \"file:\" does not conform to schema (%s)", systemdStandardOutputRegex),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.options.validate()
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestGenSystemdMountStages(t *testing.T) {
	type testCase struct {
		ptname         string // name of partition table from internal/testdisk/partition.go
		unitPath       SystemdUnitPath
		expectedStages []*Stage
	}

	// common install section
	installSection := &InstallSection{
		WantedBy: []string{"multi-user.target"},
	}

	testCases := []testCase{
		{
			ptname:   "plain",
			unitPath: EtcUnitPath,
			expectedStages: []*Stage{
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "-.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/3112efb3-3b6f-4fad-bdeb-445e54d8cac4",
								Where:   "/",
								Type:    "xfs",
								Options: "defaults",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "boot-efi.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    fmt.Sprintf("/dev/disk/by-uuid/%s", disk.EFIFilesystemUUID),
								Where:   "/boot/efi",
								Type:    "vfat",
								Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "boot.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/1407bf28-e80a-4bf0-8cf7-57812428b076",
								Where:   "/boot",
								Type:    "xfs",
								Options: "defaults",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd",
					Options: &SystemdStageOptions{
						EnabledServices: []string{
							"-.mount",
							"boot-efi.mount",
							"boot.mount",
						},
					},
				},
			},
		},
		{
			ptname:   "plain-swap",
			unitPath: EtcUnitPath,
			expectedStages: []*Stage{
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "-.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/061761a9-7a43-441f-a538-5d9754040908",
								Where:   "/",
								Type:    "xfs",
								Options: "defaults",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "boot-efi.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    fmt.Sprintf("/dev/disk/by-uuid/%s", disk.EFIFilesystemUUID),
								Where:   "/boot/efi",
								Type:    "vfat",
								Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "boot.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/1407bf28-e80a-4bf0-8cf7-57812428b076",
								Where:   "/boot",
								Type:    "xfs",
								Options: "defaults",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: `dev-disk-by\x2duuid-3112efb3\x2d3b6f\x2d4fad\x2dbdeb\x2d445e54d8cac4.swap`,
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Swap: &SwapSection{
								What:    "/dev/disk/by-uuid/3112efb3-3b6f-4fad-bdeb-445e54d8cac4",
								Options: "defaults",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd",
					Options: &SystemdStageOptions{
						EnabledServices: []string{
							"-.mount",
							"boot-efi.mount",
							"boot.mount",
							`dev-disk-by\x2duuid-3112efb3\x2d3b6f\x2d4fad\x2dbdeb\x2d445e54d8cac4.swap`,
						},
					},
				},
			},
		},
		{
			ptname:   "btrfs",
			unitPath: EtcUnitPath,
			expectedStages: []*Stage{
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "-.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/3112efb3-3b6f-4fad-bdeb-445e54d8cac4",
								Where:   "/",
								Type:    "btrfs",
								Options: "subvol=root",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "boot-efi.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    fmt.Sprintf("/dev/disk/by-uuid/%s", disk.EFIFilesystemUUID),
								Where:   "/boot/efi",
								Type:    "vfat",
								Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "boot.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/1407bf28-e80a-4bf0-8cf7-57812428b076",
								Where:   "/boot",
								Type:    "xfs",
								Options: "defaults",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd.unit.create",
					Options: &SystemdUnitCreateStageOptions{
						Filename: "var.mount",
						UnitPath: EtcUnitPath,
						Config: SystemdUnit{
							Unit: &UnitSection{
								DefaultDependencies: common.ToPtr(true),
							},
							Mount: &MountSection{
								What:    "/dev/disk/by-uuid/3112efb3-3b6f-4fad-bdeb-445e54d8cac4",
								Where:   "/var",
								Type:    "btrfs",
								Options: "subvol=var",
							},
							Install: installSection,
						},
					},
				},
				{
					Type: "org.osbuild.systemd",
					Options: &SystemdStageOptions{
						EnabledServices: []string{
							"-.mount",
							"boot-efi.mount",
							"boot.mount",
							"var.mount",
						},
					},
				},
			},
		},
	}

	partitionTables := testdisk.TestPartitionTables()
	for _, tc := range testCases {
		t.Run(tc.ptname, func(t *testing.T) {
			pt := partitionTables[tc.ptname]
			pt.GenerateUUIDs(rand.New(rand.NewSource(13))) // #nosec G404

			assert := assert.New(t)
			assert.NotNil(pt)

			stages, err := GenSystemdMountStages(&pt)
			assert.NoError(err)

			assert.Equal(tc.expectedStages, stages)
		})
	}
}
