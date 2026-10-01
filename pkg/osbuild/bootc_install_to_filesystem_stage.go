package osbuild

import (
	"fmt"

	"slices"
	"strings"

	"github.com/osbuild/image-builder/pkg/platform"
)

type BootcInstallToFilesystemOptions struct {
	// options for --root-ssh-authorized-keys
	RootSSHAuthorizedKeys []string `json:"root-ssh-authorized-keys,omitempty"`
	// options for --karg
	Kargs []string `json:"kernel-args,omitempty"`

	// option for --target-imgref
	TargetImgref string `json:"target-imgref"`

	Bootloader string `json:"bootloader,omitempty"`

	ComposeFS *bool `json:"composefs,omitempty"`
}

func (BootcInstallToFilesystemOptions) isStageOptions() {}

// NewBootcInstallToFilesystem creates a new stage for the
// org.osbuild.bootc.install-to-filesystem stage.
//
// It requires a mount setup so that bootupd can be run by bootc. I.e
// "/", "/boot" and "/boot/efi" need to be set up so that
// bootc/bootupd find and install all required bootloader bits.
// With varMounts, filesystems for /var and below are mounted as well, so that
// bootc initializes them from the image. Only pass it for a bootc that
// advertises this (see bootc.Info.InstallVarMounts).
//
// The mounts input should be generated with GenBootupdDevicesMounts.
func NewBootcInstallToFilesystemStage(options *BootcInstallToFilesystemOptions, inputs ContainerDeployInputs, devices map[string]Device, mounts []Mount, pltf platform.Platform, varMounts bool) (*Stage, error) {
	if err := validateBootupdMounts(mounts, pltf); err != nil {
		return nil, err
	}

	if len(inputs.Images.References) != 1 {
		return nil, fmt.Errorf("expected exactly one container input but got: %v (%v)", len(inputs.Images.References), inputs.Images.References)
	}

	// Don't mount other custom mountpoints, bootc requires an otherwise empty
	// target. Older bootc versions reject /var mountpoints, or leave those
	// filesystems empty so that they hide the image's /var content at boot.
	requiredMountpoints := []string{"/", "/boot", "/boot/efi"}
	reqMounts := make([]Mount, 0, len(mounts))
	for _, mount := range mounts {
		if slices.Contains(requiredMountpoints, mount.Target) || (varMounts && IsVarMountpoint(mount.Target)) {
			reqMounts = append(reqMounts, mount)
		}
	}

	return &Stage{
		Type:    "org.osbuild.bootc.install-to-filesystem",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  reqMounts,
	}, nil
}

// IsVarMountpoint returns true for /var and mountpoints below it.
func IsVarMountpoint(target string) bool {
	return target == "/var" || strings.HasPrefix(target, "/var/")
}
