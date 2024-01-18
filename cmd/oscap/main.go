package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/osbuild/image-builder/internal/distribution"
	v1 "github.com/osbuild/image-builder/internal/v1"
)

// Unmarshal the blueprint toml file in a custom data structure
type Packages struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type Filesystem struct {
	Mountpoint string `json:"mountpoint,omitempty"`
	Size       uint64 `json:"size,omitempty"`
}

type Kernel struct {
	Name   *string `json:"name,omitempty"`
	Append *string `json:"append"`
}

type Services struct {
	Disabled *[]string `json:"disabled,omitempty"`
	Enabled  *[]string `json:"enabled,omitempty"`
}

type Customizations struct {
	Filesystem []Filesystem `json:"filesystem,omitempty"`
	Packages   *[]string    `json:"packages,omitempty"`
	Openscap   *OpenSCAP    `json:"openscap,omitempty"`
	Kernel     *Kernel      `json:"kernel,omitempty"`
	Services   *Services    `json:"services,omitempty"`
}

type OpenSCAP struct {
	// add Name & Description to the customizations struct
	// so that these are saved to the json file
	Name        string `json:"profile_name,omitempty"`
	Description string `json:"profile_description,omitempty"`
}

type Blueprint struct {
	Customizations Customizations
	Packages       []Packages
	Description    string // get the description from the blueprint.toml
	Name           string
}

func cleanToml(dir string, datastreamDistro string, profile string) {
	fmt.Printf("        clean blueprint.toml ")
	// delete toml file, there's no need to keep It
	err := os.Remove(path.Join(dir, "blueprint.toml"))
	if err != nil {
		panic(err)
	}
	fmt.Println("✓")
}

func getToml(dir string, datastreamDistro string, profile string) {
	fmt.Printf("        get blueprint.toml ")
	cmd := exec.Command("oscap",
		"xccdf",
		"generate",
		"fix",
		"--profile",
		string(profile),
		"--fix-type",
		"blueprint",
		fmt.Sprintf(
			"/usr/share/xml/scap/ssg/content/ssg-%s-ds.xml",
			datastreamDistro,
		),
	) // #nosec G204 This is a utility program that a dev is gonna start by hand, there's no risk here.
	bpFile, err := os.Create(path.Join(dir, "blueprint.toml")) // #nosec G304
	if err != nil {
		panic(err)
	}
	defer bpFile.Close()
	cmd.Stdout = bpFile
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	err = cmd.Wait()
	if err != nil {
		panic(err)
	}
	fmt.Println("✓")
}

func getDescriptionFromProfileInfo(profileInfo string) string {
	descriptionBlock := strings.Split(profileInfo, "Description: ")
	if len(descriptionBlock) <= 1 {
		return ""
	}
	description := strings.Split(descriptionBlock[1], "\n")
	return description[0] // get rid of new line
}

func getProfileDescription(datastreamDistro string, profile string) string {
	fmt.Printf("        get profile description ")
	cmd := exec.Command("oscap",
		"info",
		"--profile",
		string(profile),
		fmt.Sprintf(
			"/usr/share/xml/scap/ssg/content/ssg-%s-ds.xml",
			datastreamDistro,
		),
	) // #nosec G204 This is a utility program that a dev is gonna start by hand, there's no risk here.
	output, err := cmd.Output()
	if err != nil {
		// we don't want to error out here, so just warn
		// as we still want mountpoint and package info
		msg := fmt.Sprintf("Warning: error getting description for %s profile", profile)
		fmt.Println(msg)
		panic(err)
	}
	fmt.Println("✓")
	description := getDescriptionFromProfileInfo(string(output))
	return description
}

func generateJson(dir, datastreamDistro, profileDescription, profile string) {
	fmt.Printf("        generate customizations.json ")
	bpFile, err := os.Open(path.Join(dir, "blueprint.toml")) // #nosec G304
	if err != nil {
		panic(err)
	}
	defer bpFile.Close()

	bpFileContent, err := io.ReadAll(bpFile)
	if err != nil {
		panic(err)
	}
	var bp Blueprint
	err = toml.Unmarshal(bpFileContent, &bp)
	if err != nil {
		panic(err)
	}

	// Convert the custom data structure into a `Customizations` object.
	// This will be easier to handle in IB's API later on
	customizations := v1.Customizations{}
	var fs []v1.Filesystem
	for _, bpFileSystem := range bp.Customizations.Filesystem {
		fs = append(fs, v1.Filesystem{MinSize: bpFileSystem.Size, Mountpoint: bpFileSystem.Mountpoint})
	}
	if len(fs) > 0 {
		customizations.Filesystem = &fs
	}

	var packages []string
	for _, bpPackage := range bp.Packages {
		packages = append(packages, bpPackage.Name)
	}
	if len(packages) > 0 {
		customizations.Packages = &packages
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
			services.Enabled = s.Enabled
		}
		if s.Disabled != nil {
			services.Disabled = s.Disabled
		}
	}
	if services != nil {
		customizations.Services = services
	}

	openscap := v1.OpenSCAP{}
	openscap.ProfileId = profile
	openscap.ProfileName = &bp.Description // annoyingly the Profile name is saved to the blueprint description
	openscap.ProfileDescription = &profileDescription
	customizations.Openscap = &openscap

	// Write it all down on the fileSystem
	bArray, err := json.Marshal(customizations)
	if err != nil {
		panic(err)
	}
	// hack to add an empty line at the end of the file for nicer diffs
	bArray = append(bArray, '\n')
	err = os.WriteFile(path.Join(dir, "customizations.json"), bArray, os.ModePerm)
	if err != nil {
		panic(err)
	}
	fmt.Println("✓")
}

// This program needs as an argument the directory to the distributions root file
func main() {
	var distributionsFolder = os.Args[1]
	distros, err := distribution.LoadDistroRegistry(distributionsFolder)
	distros.Available(true).List()
	if err != nil {
		panic(err)
	}

	for _, distro := range distros.Available(true).List() {
		oscapDistroName := distro.OscapName
		profiles, _ := v1.OscapProfiles(
			v1.Distributions(distro.Distribution.Name),
		)
		fmt.Printf("Distribution %s:\n", distro.Distribution.Name)
		for _, profile := range profiles {
			fmt.Printf("    %s\n", profile)
			// prepare the directory to store the blueprint.
			// * the path should be $oscapFolder/datastreamDistro/profile/blueprint.toml
			dir := path.Join(
				distributionsFolder,
				distro.Distribution.Name,
				"oscap",
				filepath.Base(string(profile)))
			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				panic(err)
			}
			// toml generation
			getToml(dir, oscapDistroName, string(profile))
			// get profile description
			profileDescription := getProfileDescription(oscapDistroName, string(profile))
			// json generation
			generateJson(dir, oscapDistroName, profileDescription, string(profile))
			// toml is not needed in the repo
			cleanToml(dir, oscapDistroName, string(profile))
		}
	}
}
