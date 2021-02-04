package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/osbuild/image-builder/internal/cloudapi"
)

type DistributionFile struct {
	ModulePlatformID string           `json:"module_platform_id"`
	Distribution     DistributionItem `json:"distribution"`
	ArchX86          *X86_64          `json:"x86_64,omitempty"`
}

type X86_64 struct {
	ImageTypes   []string              `json:"image_types"`
	Repositories []cloudapi.Repository `json:"repositories"`
}

func ReadDistributions(distsDir, distro string) ([]DistributionFile, error) {
	// note: last value is because tests' pwd is not the repository root !!!
	var distributions []DistributionFile
	err := filepath.Walk(distsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Ignore non-json and the packages files in the distribution dir
		if filepath.Ext(path) != ".json" || strings.HasSuffix(path, "packages.json") {
			return nil
		}
		if distro != "" && strings.TrimSuffix(info.Name(), ".json") != distro {
			return nil
		}

		f, err := os.Open(path) // #nosec G304
		if err != nil {
			return err
		}
		defer f.Close() // #nosec G307
		var d DistributionFile
		err = json.NewDecoder(f).Decode(&d)
		if err != nil {
			return err
		}
		distributions = append(distributions, d)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(distributions) == 0 {
		return nil, fmt.Errorf("No distributions found, is %v populated with json files?", distsDir)
	}

	return distributions, nil
}

func RepositoriesForImage(distsDir, distro, arch string) ([]cloudapi.Repository, error) {
	distributions, err := ReadDistributions(distsDir, distro)
	if err != nil {
		return nil, err
	}

	switch arch {
	case "x86_64":
		return distributions[0].ArchX86.Repositories, nil
	default:
		return nil, fmt.Errorf("Architecture not supported")
	}
}

func AvailableDistributions(distsDir string) (Distributions, error) {
	distributions, err := ReadDistributions(distsDir, "")
	if err != nil {
		return nil, err
	}

	var availableDistributions Distributions
	for _, distro := range distributions {
		availableDistributions = append(availableDistributions, distro.Distribution)
	}
	return availableDistributions, nil
}

func ArchitecturesForImage(distsDir, distro string) (Architectures, error) {
	distributions, err := ReadDistributions(distsDir, distro)
	if err != nil {
		return nil, err
	}
	d := distributions[0]

	var archs Architectures
	if d.ArchX86 != nil {
		archs = append(archs, ArchitectureItem{
			Arch:       "x86_64",
			ImageTypes: d.ArchX86.ImageTypes,
		})
	}
	return archs, nil
}

type PackagesFile struct {
	Data []Package `json:"data"`
}

func FindPackages(distsDir, distro, arch, search string) ([]Package, error) {
	f, err := os.Open(path.Join(distsDir, fmt.Sprintf("%v-%v-packages.json", distro, arch))) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer f.Close() // #nosec G307

	var p PackagesFile
	err = json.NewDecoder(f).Decode(&p)
	if err != nil {
		return nil, err
	}

	sort.Slice(p.Data, func(i, j int) bool {
		return strings.ToLower(p.Data[i].Name) < strings.ToLower(p.Data[j].Name)
	})

	var filtPkgs []Package
	for _, p := range p.Data {
		if strings.Contains(p.Name, search) {
			filtPkgs = append(filtPkgs, p)
		}
	}
	return filtPkgs, nil
}
