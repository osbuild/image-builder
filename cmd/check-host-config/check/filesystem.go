package check

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/osbuild/image-builder/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "filesystem",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
		RunOn:                  []string{"!rhel-8.4", "!rhel-8.6", "!rhel-8.8", "!rhel-8.10"},
	}, filesystemCheck)
}

// collectExpectedMountpoints gathers all mountpoints from both
// customizations.filesystem and customizations.disk (plain, LVM, btrfs).
func collectExpectedMountpoints(config *buildconfig.BuildConfig) []string {
	c := config.Blueprint.Customizations
	var mountpoints []string

	for _, fs := range c.Filesystem {
		if fs.Mountpoint != "" {
			mountpoints = append(mountpoints, fs.Mountpoint)
		}
	}

	if c.Disk != nil {
		for _, part := range c.Disk.Partitions {
			switch part.Type {
			case "plain", "":
				if part.FSType == "swap" {
					mountpoints = append(mountpoints, "swap")
				} else if part.Mountpoint != "" {
					mountpoints = append(mountpoints, part.Mountpoint)
				}
			case "lvm":
				for _, lv := range part.LogicalVolumes {
					if lv.FSType == "swap" {
						mountpoints = append(mountpoints, "swap")
					} else if lv.Mountpoint != "" {
						mountpoints = append(mountpoints, lv.Mountpoint)
					}
				}
			case "btrfs":
				for _, subvol := range part.Subvolumes {
					if subvol.Mountpoint != "" {
						mountpoints = append(mountpoints, subvol.Mountpoint)
					}
				}
			}
		}
	}

	return mountpoints
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Mountpoints []*string     `json:"mountpoints"`
	FSType      *string       `json:"fstype"`
	Children    []lsblkDevice `json:"children"`
}

// getActiveMountpoints returns a set of currently mounted paths by parsing
// the JSON output of lsblk. Swap devices are identified by fstype.
func getActiveMountpoints() (map[string]bool, error) {
	stdout, stderr, exitCode, err := Exec("lsblk", "-J", "-o", "MOUNTPOINTS,FSTYPE")
	if err != nil {
		return nil, fmt.Errorf("failed to run lsblk: %w (exit %d, stderr: %s)", err, exitCode, string(stderr))
	}

	var output lsblkOutput
	if err := json.Unmarshal(stdout, &output); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk JSON: %w", err)
	}

	mounts := make(map[string]bool)
	var collect func([]lsblkDevice)
	collect = func(devices []lsblkDevice) {
		for _, dev := range devices {
			if dev.FSType != nil && *dev.FSType == "swap" {
				mounts["swap"] = true
			}
			for _, mp := range dev.Mountpoints {
				if mp != nil && *mp != "" && *mp != "[SWAP]" {
					mounts[*mp] = true
				}
			}
			collect(dev.Children)
		}
	}
	collect(output.BlockDevices)

	return mounts, nil
}

func filesystemCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	expected := collectExpectedMountpoints(config)
	if len(expected) == 0 {
		return Skip("no filesystem or disk customizations")
	}

	active, err := getActiveMountpoints()
	if err != nil {
		return Fail("failed to read active mountpoints:", err)
	}

	var missing []string
	for _, mp := range expected {
		if active[mp] {
			log.Printf("filesystem check: mountpoint %s is present\n", mp)
		} else {
			missing = append(missing, mp)
		}
	}

	if len(missing) > 0 {
		return Fail("expected mountpoints not found:", strings.Join(missing, ", "))
	}

	return Pass()
}
