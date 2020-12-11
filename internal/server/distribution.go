package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder/internal/cloudapi"
)

type Distribution struct {
	Distribution DistributionItem `json:"distribution"`
	ArchX86      *X86_64          `json:"x86_64,omitempty"`
}

type X86_64 struct {
	ImageTypes   []string              `json:"image_types"`
	Repositories []cloudapi.Repository `json:"repositories"`
}

func ReadDistributions(distro string) ([]Distribution, error) {
	// note: last value is because tests' pwd is not the repository root !!!
	confPaths := [4]string{"/usr/share/image-builder/distributions", "/app/distributions", "./distributions", "../../distributions"}
	var distributions []Distribution

	var err error
	for _, confPath := range confPaths {
		err = filepath.Walk(confPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if filepath.Ext(path) != ".json" {
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
			var d Distribution
			err = json.NewDecoder(f).Decode(&d)
			if err != nil {
				return err
			}
			distributions = append(distributions, d)
			return nil
		})
		// If the *distributions directory wasn't found, continue to the next one
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	if len(distributions) == 0 {
		return nil, fmt.Errorf("No distributions found, is %v populated with json files?", confPaths[0])
	}

	return distributions, nil
}

func RepositoriesForImage(distro string, arch string) ([]cloudapi.Repository, error) {
	distributions, err := ReadDistributions(distro)
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

func AvailableDistributions() (Distributions, error) {
	distributions, err := ReadDistributions("")
	if err != nil {
		return nil, err
	}

	var availableDistributions Distributions
	for _, distro := range distributions {
		availableDistributions = append(availableDistributions, distro.Distribution)
	}
	return availableDistributions, nil
}

func ArchitecturesForImage(distro string) (Architectures, error) {
	distributions, err := ReadDistributions(distro)
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
