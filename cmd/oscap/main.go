package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"

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

type Customizations struct {
	Filesystem []Filesystem `json:"filesystem,omitempty"`
	Packages   *[]string    `json:"packages,omitempty"`
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

func generateJson(dir string, datastreamDistro string, profile string) {
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
		oscapName := distro.OscapName
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
			getToml(dir, oscapName, string(profile))
			// json generation
			generateJson(dir, oscapName, string(profile))
			// toml is not needed in the repo
			cleanToml(dir, oscapName, string(profile))
		}
	}
}
