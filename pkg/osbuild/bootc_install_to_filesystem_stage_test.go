package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
)

func makeFakeContainerInputs() osbuild.ContainerDeployInputs {
	return osbuild.ContainerDeployInputs{
		Images: osbuild.NewContainersInputForSources([]container.Spec{
			{
				ImageID:   "id-0",
				Source:    "registry.example.org/reg/img",
				LocalName: "local-name",
			},
		},
		),
	}
}

func TestBootcInstallToFilesystemStageNewHappy(t *testing.T) {
	devices := makeOsbuildDevices("dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	inputs := makeFakeContainerInputs()
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	expectedStage := &osbuild.Stage{
		Type:    "org.osbuild.bootc.install-to-filesystem",
		Options: (*osbuild.BootcInstallToFilesystemOptions)(nil),
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}
	stage, err := osbuild.NewBootcInstallToFilesystemStage(nil, inputs, devices, mounts, pf)
	require.Nil(t, err)
	assert.Equal(t, stage, expectedStage)
}

func TestBootcInstallToFilesystemStageNewEssentialMountsOnly(t *testing.T) {
	devices := makeOsbuildDevices("dev-for-/", "dev-for-/boot/efi", "dev-for-/var/log")
	mounts := makeOsbuildMounts("/", "/boot/efi", "/var/log")
	inputs := makeFakeContainerInputs()
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	expectedStage := &osbuild.Stage{
		Type:    "org.osbuild.bootc.install-to-filesystem",
		Options: (*osbuild.BootcInstallToFilesystemOptions)(nil),
		Inputs:  inputs,
		Devices: devices,
		Mounts: []osbuild.Mount{
			{Name: "mnt-for-/", Type: "org.osbuild.ext4", Source: "dev-for-/", Target: "/"},
			{Name: "mnt-for-/boot/efi", Type: "org.osbuild.vfat", Source: "dev-for-/boot/efi", Target: "/boot/efi"},
		},
	}
	stage, err := osbuild.NewBootcInstallToFilesystemStage(nil, inputs, devices, mounts, pf)
	require.Nil(t, err)
	assert.Equal(t, expectedStage, stage)
}

func TestBootcInstallToFilesystemStageNewNoContainers(t *testing.T) {
	devices := makeOsbuildDevices("dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	inputs := osbuild.ContainerDeployInputs{}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	_, err := osbuild.NewBootcInstallToFilesystemStage(nil, inputs, devices, mounts, pf)
	assert.EqualError(t, err, "expected exactly one container input but got: 0 (map[])")
}

func TestBootcInstallToFilesystemStageNewTwoContainers(t *testing.T) {
	devices := makeOsbuildDevices("dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	inputs := osbuild.ContainerDeployInputs{
		Images: osbuild.ContainersInput{
			References: map[string]osbuild.ContainersInputSourceRef{
				"1": {},
				"2": {},
			},
		},
	}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	_, err := osbuild.NewBootcInstallToFilesystemStage(nil, inputs, devices, mounts, pf)
	assert.EqualError(t, err, "expected exactly one container input but got: 2 (map[1:{} 2:{}])")
}

func TestBootcInstallToFilesystemStageMissingMounts(t *testing.T) {
	devices := makeOsbuildDevices("dev-for-/")
	mounts := makeOsbuildMounts("/")
	inputs := makeFakeContainerInputs()
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	stage, err := osbuild.NewBootcInstallToFilesystemStage(nil, inputs, devices, mounts, pf)
	// XXX: rename error
	assert.ErrorContains(t, err, "required mounts for bootupd stage [/boot/efi] missing")
	require.Nil(t, stage)
}

func TestBootcInstallToFilesystemStageJsonHappy(t *testing.T) {
	devices := makeOsbuildDevices("disk", "dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	inputs := makeFakeContainerInputs()
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	opts := &osbuild.BootcInstallToFilesystemOptions{
		TargetImgref: "quay.io/centos-bootc/centos-bootc-dev:stream9",
	}
	stage, err := osbuild.NewBootcInstallToFilesystemStage(opts, inputs, devices, mounts, pf)
	require.Nil(t, err)
	stageJson, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(stageJson), `{
  "type": "org.osbuild.bootc.install-to-filesystem",
  "inputs": {
    "images": {
      "type": "org.osbuild.containers",
      "origin": "org.osbuild.source",
      "references": {
        "id-0": {
          "name": "local-name"
        }
      }
    }
  },
  "options": {
    "target-imgref": "quay.io/centos-bootc/centos-bootc-dev:stream9"
  },
  "devices": {
    "dev-for-/": {
      "type": "org.osbuild.loopback"
    },
    "dev-for-/boot": {
      "type": "org.osbuild.loopback"
    },
    "dev-for-/boot/efi": {
      "type": "org.osbuild.loopback"
    },
    "disk": {
      "type": "org.osbuild.loopback"
    }
  },
  "mounts": [
    {
      "name": "mnt-for-/",
      "type": "org.osbuild.ext4",
      "source": "dev-for-/",
      "target": "/"
    },
    {
      "name": "mnt-for-/boot",
      "type": "org.osbuild.ext4",
      "source": "dev-for-/boot",
      "target": "/boot"
    },
    {
      "name": "mnt-for-/boot/efi",
      "type": "org.osbuild.vfat",
      "source": "dev-for-/boot/efi",
      "target": "/boot/efi"
    }
  ]
}`)
}
