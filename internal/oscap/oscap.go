package oscap

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

func GenXMLTailoringFile(tailoring *string, datastream string) (*os.File, error) {
	if tailoring == nil || *tailoring == "" {
		// we don't need to process this any further,
		// since the xml file will just end up blank
		// and would cause issues later down the line
		return nil, nil
	}

	jsonFile, err := os.CreateTemp("", "tailoring.json")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp json file: %w", err)
	}
	defer os.Remove(jsonFile.Name())

	_, err = jsonFile.Write([]byte(*tailoring))
	if err != nil {
		return nil, fmt.Errorf("Error writing json customizations to temp file: %w", err)
	}

	xmlFile, err := os.CreateTemp("", "tailoring.xml")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp xml file: %w", err)
	}

	// TODO: json schema validation
	// we could potentially validate the `json` input
	// here against:
	// https://github.com/ComplianceAsCode/schemas/blob/b91c8e196a8cc515e0cc7f10b2c5a02b4179c0e5/tailoring/schema.json
	// Alternatively, we could just fetch the `xml` blob from the compliance service and
	// skip this step altogether

	// The oscap blueprint generation tool
	// doesn't accept `json` as input, so we
	// need to convert it to `xml`
	cmd := exec.Command(
		"autotailor",
		"-j", jsonFile.Name(),
		"-o", xmlFile.Name(),
		datastream,
	) // #nosec G204

	if err := cmd.Run(); err != nil {
		defer os.Remove(xmlFile.Name())
		return nil, fmt.Errorf("Error executing blueprint generation: %w", err)
	}

	return xmlFile, nil
}

func GenTOMLBlueprint(profile string, datastream string, file *os.File) ([]byte, error) {
	var cmd *exec.Cmd
	if file != nil {
		cmd = exec.Command("oscap",
			"xccdf",
			"generate",
			"fix",
			"--profile",
			string(profile),
			"--tailoring-file",
			file.Name(),
			"--fix-type",
			"blueprint",
			datastream,
		) // #nosec G204
	} else {
		cmd = exec.Command("oscap",
			"xccdf",
			"generate",
			"fix",
			"--profile",
			string(profile),
			"--fix-type",
			"blueprint",
			datastream,
		) // #nosec G204
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Error generating toml blueprint: %w", err)
	}

	return output, nil
}
