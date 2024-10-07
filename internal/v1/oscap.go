package v1

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
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

func loadOscapCustomizations(distributionDir string, distribution Distributions, profile DistributionProfileItem) (*Customizations, error) {
	//Load the json file with the customizations
	//Ignore the warning from gosec, as this function is only used internally. oscapDir comes from the server
	//configuration and Base path is gotten from the other params, so everything is fine security wise.
	jsonFile, err := os.Open(path.Join(
		distributionDir,
		string(distribution),
		"oscap",
		filepath.Base(string(profile)),
		"customizations.json")) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	bytes, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	// The customizations json file already contains a valid Customizations object to be returned as is.
	var customizations Customizations
	err = json.Unmarshal(bytes, &customizations)
	if err != nil {
		return nil, err
	}

	if customizations.Openscap == nil {
		// set the profile id in the customizations object
		return nil, errors.New("Customizations file is missing OpenSCAP section")
	}

	return &customizations, nil
}
