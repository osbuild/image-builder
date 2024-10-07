package v1

import (
	"errors"
	"os"
	"slices"

	"github.com/BurntSushi/toml"
	"github.com/osbuild/image-builder/internal/oscap"
)

func OscapProfiles(distribution Distributions) (DistributionProfileResponse, error) {
	switch distribution {
	case Rhel8, Rhel84, Rhel85, Rhel86, Rhel87, Rhel88, Rhel89, Rhel810, Rhel8Nightly:
		return DistributionProfileResponse{
			XccdfOrgSsgprojectContentProfileAnssiBp28Enhanced,
			XccdfOrgSsgprojectContentProfileAnssiBp28High,
			XccdfOrgSsgprojectContentProfileAnssiBp28Intermediary,
			XccdfOrgSsgprojectContentProfileAnssiBp28Minimal,
			XccdfOrgSsgprojectContentProfileCis,
			XccdfOrgSsgprojectContentProfileCisServerL1,
			XccdfOrgSsgprojectContentProfileCisWorkstationL1,
			XccdfOrgSsgprojectContentProfileCisWorkstationL2,
			XccdfOrgSsgprojectContentProfileCui,
			XccdfOrgSsgprojectContentProfileE8,
			XccdfOrgSsgprojectContentProfileHipaa,
			XccdfOrgSsgprojectContentProfileIsmO,
			XccdfOrgSsgprojectContentProfileOspp,
			XccdfOrgSsgprojectContentProfilePciDss,
			XccdfOrgSsgprojectContentProfileStig,
			XccdfOrgSsgprojectContentProfileStigGui,
		}, nil
	case Centos9, Rhel9, Rhel91, Rhel92, Rhel93, Rhel94, Rhel9Nightly:
		return DistributionProfileResponse{
			XccdfOrgSsgprojectContentProfileAnssiBp28Enhanced,
			XccdfOrgSsgprojectContentProfileAnssiBp28High,
			XccdfOrgSsgprojectContentProfileAnssiBp28Intermediary,
			XccdfOrgSsgprojectContentProfileAnssiBp28Minimal,
			XccdfOrgSsgprojectContentProfileCcnAdvanced,
			XccdfOrgSsgprojectContentProfileCcnBasic,
			XccdfOrgSsgprojectContentProfileCcnIntermediate,
			XccdfOrgSsgprojectContentProfileCis,
			XccdfOrgSsgprojectContentProfileCisServerL1,
			XccdfOrgSsgprojectContentProfileCisWorkstationL1,
			XccdfOrgSsgprojectContentProfileCisWorkstationL2,
			XccdfOrgSsgprojectContentProfileCui,
			XccdfOrgSsgprojectContentProfileE8,
			XccdfOrgSsgprojectContentProfileHipaa,
			XccdfOrgSsgprojectContentProfileIsmO,
			XccdfOrgSsgprojectContentProfileOspp,
			XccdfOrgSsgprojectContentProfilePciDss,
			XccdfOrgSsgprojectContentProfileStig,
			XccdfOrgSsgprojectContentProfileStigGui,
		}, nil
	case Rhel90:
		fallthrough
	default:
		return nil, errors.New("No profile for the specified distribution")
	}
}

func BlueprintToCustomizations(profile string, description string, bp oscap.Blueprint) (*Customizations, error) {
	// Convert the custom data structure into a `Customizations` object.
	// This will be easier to handle in IB's API later on
	customizations := Customizations{}

	var fs []Filesystem
	for _, bpFileSystem := range bp.Customizations.Filesystem {
		fs = append(fs, Filesystem{
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

	var kernel *Kernel
	if k := bp.Customizations.Kernel; k != nil {
		kernel = &Kernel{}
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

	var services *Services
	if s := bp.Customizations.Services; s != nil {
		services = &Services{}
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

	var openscap OpenSCAP
	err := openscap.FromOpenSCAPProfile(OpenSCAPProfile{
		ProfileId:          profile,
		ProfileName:        &bp.Description, // annoyingly the Profile name is saved to the blueprint description
		ProfileDescription: &profileDescription,
	})
	if err != nil {
		return nil, err
	}
	customizations.Openscap = &openscap

	return &customizations, nil
}

func processRequest(profile string, datastream string, tailoring *string) (*Customizations, error) {
	// the generated blueprint doesn't contain the profile
	// description, so we have to run the oscap tool to get
	// this information
	description := oscap.GetProfileDescription(profile, datastream)

	var file *os.File
	if tailoring != nil {
		file, err := oscap.GenXMLTailoringFile(tailoring, datastream)
		if err != nil {
			return nil, err
		}
		defer os.Remove(file.Name())
	}

	rawBp, err := oscap.GenTOMLBlueprint(profile, datastream, file)
	if err != nil {
		return nil, err
	}

	var bp *oscap.Blueprint
	err = toml.Unmarshal(rawBp, &bp)
	if err != nil {
		return nil, err
	}

	return BlueprintToCustomizations(profile, description, *bp)
}
