package oscap

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"

	v1 "github.com/osbuild/image-builder/internal/v1"
)

type Packages struct {
	Name    string `json:"name,omitempty" toml:"name,omitempty"`
	Version string `json:"version,omitempty" toml:"version,omitempty"`
}

type Filesystem struct {
	Mountpoint string `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	Size       uint64 `json:"size,omitempty" toml:"size,omitempty"`
}

type Kernel struct {
	Name   *string `json:"name,omitempty" toml:"name,omitempty"`
	Append *string `json:"append" toml:"append"`
}

type Services struct {
	Disabled *[]string `json:"disabled,omitempty" toml:"disabled,omitempty"`
	Enabled  *[]string `json:"enabled,omitempty" toml:"enabled,omitempty"`
	Masked   *[]string `json:"masked,omitempty" toml:"masked,omitempty"`
}

type Customizations struct {
	Filesystem []Filesystem `json:"filesystem,omitempty" toml:"filesystem,omitempty"`
	Packages   *[]string    `json:"packages,omitempty" toml:"packages,omitempty"`
	Openscap   *OpenSCAP    `json:"openscap,omitempty" toml:"openscap,omitempty"`
	Kernel     *Kernel      `json:"kernel,omitempty" toml:"kernel,omitempty"`
	Services   *Services    `json:"services,omitempty" toml:"services,omitempty"`
}

type OpenSCAP struct {
	// add Name & Description to the customizations struct
	// so that these are saved to the json file
	Name        string `json:"profile_name,omitempty" toml:"profile_name,omitempty"`
	Description string `json:"profile_description,omitempty" toml:"profile_description,omitempty"`
}

type Blueprint struct {
	Customizations Customizations
	Packages       []Packages
	Description    string // get the description from the blueprint.toml
	Name           string
}

func GetProfileDescription(profile string, datastream string) string {
	cmd := exec.Command("oscap",
		"info",
		"--profile",
		string(profile),
		datastream,
	) // #nosec G204

	output, err := cmd.Output()
	if err != nil {
		// we don't want to error out here, so just warn
		// as we still want mountpoint and package info
		msg := fmt.Sprintf("Warning: error getting description for %s profile", profile)
		fmt.Println(msg)
		return ""
	}

	descriptionBlock := strings.Split(string(output), "Description: ")
	if len(descriptionBlock) <= 1 {
		return ""
	}

	description := strings.Split(descriptionBlock[1], "\n")
	return description[0] // get rid of new line
}

func BlueprintToCustomizations(profile string, description string, bp Blueprint) (v1.Customizations, error) {
	// Convert the custom data structure into a `Customizations` object.
	// This will be easier to handle in IB's API later on
	customizations := v1.Customizations{}

	var fs []v1.Filesystem
	for _, bpFileSystem := range bp.Customizations.Filesystem {
		fs = append(fs, v1.Filesystem{
			MinSize:    bpFileSystem.Size,
			Mountpoint: bpFileSystem.Mountpoint,
		})
	}
	if len(fs) > 0 {
		customizations.Filesystem = &fs
	}

	var packages []string
	for _, bpPackage := range bp.Packages {
		packages = append(packages, bpPackage.Name)
	}

	var kernel *v1.Kernel
	if k := bp.Customizations.Kernel; k != nil {
		kernel = &v1.Kernel{}
		if k.Name != nil {
			kernel.Name = k.Name
		}
		if k.Append != nil {
			kernel.Append = k.Append
		}
	}
	if kernel != nil {
		customizations.Kernel = kernel
	}

	var services *v1.Services
	if s := bp.Customizations.Services; s != nil {
		services = &v1.Services{}
		if s.Enabled != nil {
			firewalldPkg := "firewalld"
			if slices.Contains(*s.Enabled, firewalldPkg) && !slices.Contains(packages, firewalldPkg) {
				packages = append(packages, firewalldPkg)
			}
			services.Enabled = s.Enabled
		}
		var maskedAndDisabled []string
		if s.Disabled != nil {
			maskedAndDisabled = append(maskedAndDisabled, *s.Disabled...)
		}
		if s.Masked != nil {
			maskedAndDisabled = append(maskedAndDisabled, *s.Masked...)
		}
		// we need to collect both disabled and masked services and
		// assign them to the masked customization, since disabled services
		// that aren't installed on the image will break the image build.
		if maskedAndDisabled != nil {
			services.Masked = &maskedAndDisabled
		}
	}
	if services != nil {
		customizations.Services = services
	}

	if len(packages) > 0 {
		customizations.Packages = &packages
	}

	profileDescription := bp.Description
	if description != "" {
		profileDescription = description
	}

	var openscap v1.OpenSCAP
	err := openscap.FromOpenSCAPProfile(v1.OpenSCAPProfile{
		ProfileId:          profile,
		ProfileName:        &bp.Description, // annoyingly the Profile name is saved to the blueprint description
		ProfileDescription: &profileDescription,
	})
	if err != nil {
		return v1.Customizations{}, err
	}
	customizations.Openscap = &openscap

	return customizations, nil
}
