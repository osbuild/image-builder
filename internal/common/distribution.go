package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/osbuild/image-builder/internal/cloudapi"
)

type DistributionItem struct {
	Description string `json:"description"`
	Name        string `json:"name"`
}

type Distributions []DistributionItem

type ArchitectureItem struct {
	Arch       string   `json:"arch"`
	ImageTypes []string `json:"image_types"`
}

type Architectures []ArchitectureItem

type DistributionFile struct {
	ModulePlatformID string           `json:"module_platform_id"`
	Distribution     DistributionItem `json:"distribution"`
	ArchX86          *X86_64          `json:"x86_64,omitempty"`
}

type X86_64 struct {
	ImageTypes   []string              `json:"image_types"`
	Repositories []cloudapi.Repository `json:"repositories"`
}

type Package struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Version string `json:"version"`
}

type PackagesFile struct {
	Data []Package `json:"data"`
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

		cleanPath := filepath.Clean(path)
		f, err := os.Open(cleanPath)
		if err != nil {
			return err
		}
		defer func() {
			err := f.Close()
			if err != nil {
				fmt.Printf("Error closing file: %v", err)
			}
		}()
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
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Architecture not supported")
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

func FindPackages(distsDir, distro, arch, search string) ([]Package, error) {
	cleanPath := filepath.Clean(path.Join(distsDir, fmt.Sprintf("%v-%v-packages.json", distro, arch)))
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("Error closing file: %v", err)
		}
	}()
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
