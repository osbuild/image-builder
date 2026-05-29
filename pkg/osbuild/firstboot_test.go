package osbuild

import (
	"io/fs"
	"testing"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/customizations/firstboot"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func firstbootTestDir(path string) *fsnode.Directory {
	d, err := fsnode.NewDirectory(path, common.ToPtr(fs.FileMode(0755)), "root", "root", true)
	if err != nil {
		panic(err)
	}
	return d
}

func firstbootTestFile(path, data string) *fsnode.File {
	f, err := fsnode.NewFile(path, common.ToPtr(fs.FileMode(0770)), "root", "root", []byte(data))
	if err != nil {
		panic(err)
	}
	return f
}

func firstbootTestUnit(filename string, unit *UnitSection, service *ServiceSection) *SystemdUnitCreateStageOptions {
	return &SystemdUnitCreateStageOptions{
		Filename: filename,
		UnitType: SystemUnitType,
		UnitPath: UsrUnitPath,
		Config: SystemdUnit{
			Unit: unit,
			Service: &ServiceSection{
				Type:            OneshotServiceType,
				ExecStart:       service.ExecStart,
				ExecStartPre:    service.ExecStartPre,
				RemainAfterExit: true,
			},
			Install: &InstallSection{
				WantedBy: []string{"basic.target"},
			},
		},
	}
}

func TestGenFirstbootFromOptions(t *testing.T) {
	tests := []struct {
		name      string
		fbo       *firstboot.FirstbootOptions
		wantCerts []string
		wantDirs  []*fsnode.Directory
		wantFiles []*fsnode.File
		wantUnits []*SystemdUnitCreateStageOptions
	}{
		{
			name: "nil",
			fbo:  nil,
		},
		{
			name: "empty-scripts",
			fbo:  &firstboot.FirstbootOptions{},
		},
		{
			name: "single-script",
			fbo: &firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{
						Filename: "osbuild-first-boot-setup",
						Contents: "#!/bin/bash\necho setup\n",
					},
				},
			},
			wantDirs: []*fsnode.Directory{
				firstbootTestDir(firstbootScriptDir),
				firstbootTestDir("/var/local/osbuild-first-boot"),
			},
			wantFiles: []*fsnode.File{
				firstbootTestFile(firstbootMarkerPath, ""),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-setup", "#!/bin/bash\necho setup\n"),
			},
			wantUnits: []*SystemdUnitCreateStageOptions{
				firstbootTestUnit("osbuild-first-boot-setup.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After:               []string{"network-online.target", "osbuild-first-boot.service"},
					},
					&ServiceSection{
						ExecStartPre: []string{"/usr/bin/rm " + firstbootMarkerPath},
						ExecStart:    []string{"/usr/libexec/osbuild-first-boot/osbuild-first-boot-setup"},
					},
				),
			},
		},
		{
			name: "multiple-scripts",
			fbo: &firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{
						Filename:      "osbuild-first-boot-satellite",
						Contents:      "#!/usr/bin/bash\ncurl https://sat.example.com/register",
						IgnoreFailure: true,
						Certs:         []string{"cert1", "cert2"},
					},
					{
						Filename:      "osbuild-first-boot-aap",
						Contents:      "#!/usr/bin/bash\ncurl -i --data 'host_config_key=host-config-key' 'https://aap.example.com/api/v2/job_templates/9/callback/'\n",
						IgnoreFailure: true,
						Certs:         []string{"cert3", "cert4"},
						After:         []string{"sshd.service"},
					},
					{
						Filename: "osbuild-first-boot-custom-1",
						Contents: "echo 'unnamed'",
					},
				},
			},
			wantCerts: []string{"cert1", "cert2", "cert3", "cert4"},
			wantDirs: []*fsnode.Directory{
				firstbootTestDir(firstbootScriptDir),
				firstbootTestDir("/var/local/osbuild-first-boot"),
			},
			wantFiles: []*fsnode.File{
				firstbootTestFile(firstbootMarkerPath, ""),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-satellite", "#!/usr/bin/bash\ncurl https://sat.example.com/register"),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-aap", "#!/usr/bin/bash\ncurl -i --data 'host_config_key=host-config-key' 'https://aap.example.com/api/v2/job_templates/9/callback/'\n"),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-custom-1", "echo 'unnamed'"),
			},
			wantUnits: []*SystemdUnitCreateStageOptions{
				firstbootTestUnit("osbuild-first-boot-satellite.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After:               []string{"network-online.target", "osbuild-first-boot.service"},
					},
					&ServiceSection{
						ExecStart: []string{"-/usr/libexec/osbuild-first-boot/osbuild-first-boot-satellite"},
					},
				),
				firstbootTestUnit("osbuild-first-boot-aap.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After: []string{
							"network-online.target",
							"osbuild-first-boot.service",
							"sshd.service",
							"osbuild-first-boot-satellite.service",
						},
					},
					&ServiceSection{
						ExecStart: []string{"-/usr/libexec/osbuild-first-boot/osbuild-first-boot-aap"},
					},
				),
				firstbootTestUnit("osbuild-first-boot-custom-1.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After: []string{
							"network-online.target",
							"osbuild-first-boot.service",
							"osbuild-first-boot-aap.service",
						},
					},
					&ServiceSection{
						ExecStartPre: []string{"/usr/bin/rm " + firstbootMarkerPath},
						ExecStart:    []string{"/usr/libexec/osbuild-first-boot/osbuild-first-boot-custom-1"},
					},
				),
			},
		},
		{
			name: "ignore-failure-and-before",
			fbo: &firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{
						Filename:      "osbuild-first-boot-ignore-errors",
						Contents:      "echo 'ignore errors'",
						IgnoreFailure: true,
						Before:        []string{"postgresql.service"},
					},
				},
			},
			wantDirs: []*fsnode.Directory{
				firstbootTestDir(firstbootScriptDir),
				firstbootTestDir("/var/local/osbuild-first-boot"),
			},
			wantFiles: []*fsnode.File{
				firstbootTestFile(firstbootMarkerPath, ""),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-ignore-errors", "echo 'ignore errors'"),
			},
			wantUnits: []*SystemdUnitCreateStageOptions{
				firstbootTestUnit("osbuild-first-boot-ignore-errors.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After:               []string{"network-online.target", "osbuild-first-boot.service"},
						Before:              []string{"postgresql.service"},
					},
					&ServiceSection{
						ExecStartPre: []string{"/usr/bin/rm " + firstbootMarkerPath},
						ExecStart:    []string{"-/usr/libexec/osbuild-first-boot/osbuild-first-boot-ignore-errors"},
					},
				),
			},
		},
		{
			name: "two-scripts-in-order",
			fbo: &firstboot.FirstbootOptions{
				Scripts: []firstboot.Script{
					{
						Filename: "osbuild-first-boot-one",
						Contents: "echo one",
					},
					{
						Filename: "osbuild-first-boot-two",
						Contents: "echo two",
					},
				},
			},
			wantDirs: []*fsnode.Directory{
				firstbootTestDir(firstbootScriptDir),
				firstbootTestDir("/var/local/osbuild-first-boot"),
			},
			wantFiles: []*fsnode.File{
				firstbootTestFile(firstbootMarkerPath, ""),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-one", "echo one"),
				firstbootTestFile("/usr/libexec/osbuild-first-boot/osbuild-first-boot-two", "echo two"),
			},
			wantUnits: []*SystemdUnitCreateStageOptions{
				firstbootTestUnit("osbuild-first-boot-one.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After:               []string{"network-online.target", "osbuild-first-boot.service"},
					},
					&ServiceSection{
						ExecStart: []string{"/usr/libexec/osbuild-first-boot/osbuild-first-boot-one"},
					},
				),
				firstbootTestUnit("osbuild-first-boot-two.service",
					&UnitSection{
						ConditionPathExists: []string{firstbootMarkerPath},
						Wants:               []string{"network-online.target"},
						After: []string{
							"network-online.target",
							"osbuild-first-boot.service",
							"osbuild-first-boot-one.service",
						},
					},
					&ServiceSection{
						ExecStartPre: []string{"/usr/bin/rm " + firstbootMarkerPath},
						ExecStart:    []string{"/usr/libexec/osbuild-first-boot/osbuild-first-boot-two"},
					},
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, dirs, files, units, err := GenFirstbootFromOptions(tt.fbo)
			require.NoError(t, err)

			if tt.fbo == nil {
				assert.Nil(t, certs)
				assert.Nil(t, dirs)
				assert.Nil(t, files)
				assert.Nil(t, units)
				return
			}

			assert.Equal(t, tt.wantCerts, certs)
			assert.Equal(t, tt.wantUnits, units)
			require.Len(t, dirs, len(tt.wantDirs))
			for i := range tt.wantDirs {
				assert.Equal(t, tt.wantDirs[i].Path(), dirs[i].Path())
			}
			require.Len(t, files, len(tt.wantFiles))
			for i := range tt.wantFiles {
				assert.Equal(t, tt.wantFiles[i].Path(), files[i].Path())
				assert.Equal(t, tt.wantFiles[i].Data(), files[i].Data())
			}
		})
	}
}

func TestFirstbootMarkerPath(t *testing.T) {
	assert.Equal(t, "/var/local/osbuild-first-boot/.run", firstbootMarkerPath)
}
