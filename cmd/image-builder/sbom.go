package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/osbuild/image-builder/pkg/hashutil"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/sbom"
)

func writeImageSBOM(outputDir, basename string, outputPaths []string, packages []sbom.Package) error {
	if len(outputPaths) != 1 {
		return fmt.Errorf("SBOM generation requires exactly one image artifact, got %d", len(outputPaths))
	}
	imagePath := outputPaths[0]
	path := filepath.Join(outputDir, basename+".spdx.json")
	sum, err := hashutil.Sha256sum(imagePath)
	if err != nil {
		return fmt.Errorf("hashing built image for SBOM: %w", err)
	}
	doc, err := sbom.Generate(sbom.Inputs{
		Creator: sbom.Creator{
			Tool: sbom.ToolInfo{
				Name:    "image-builder",
				Version: version,
			},
		},
		Image: sbom.Image{
			Name:     basename,
			FileName: filepath.Base(imagePath),
			SHA256:   sum,
		},
		Packages: packages,
	})
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf, sbom.FormatSPDX23JSON); err != nil {
		return err
	}
	// #nosec: G306
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

const taskSBOMContentsName = "sbom-contents.json"

type taskSBOMPackage struct {
	Name          string `json:"name"`
	Epoch         uint   `json:"epoch"`
	Version       string `json:"version"`
	Release       string `json:"release"`
	Arch          string `json:"arch"`
	Vendor        string `json:"vendor"`
	URL           string `json:"url"`
	ChecksumType  string `json:"checksum_type"`
	ChecksumValue string `json:"checksum_value"`
}

func writeTaskSBOMContents(workdir string, packages []sbom.Package) error {
	encoded := make([]taskSBOMPackage, len(packages))
	for i, pkg := range packages {
		encoded[i] = taskSBOMPackage{
			Name:          pkg.Name,
			Epoch:         pkg.Epoch,
			Version:       pkg.Version,
			Release:       pkg.Release,
			Arch:          pkg.Arch,
			Vendor:        pkg.Vendor,
			URL:           pkg.URL,
			ChecksumType:  pkg.Checksum.Type,
			ChecksumValue: pkg.Checksum.Value,
		}
	}
	data, err := json.Marshal(encoded)
	if err != nil {
		return fmt.Errorf("encoding SBOM contents: %w", err)
	}
	path := filepath.Join(workdir, taskSBOMContentsName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing SBOM contents %s: %w", path, err)
	}
	return nil
}

func readTaskSBOMContents(workdir string) ([]sbom.Package, error) {
	path := filepath.Join(workdir, taskSBOMContentsName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading SBOM contents %s: %w", path, err)
	}
	var encoded []taskSBOMPackage
	if err := json.Unmarshal(data, &encoded); err != nil {
		return nil, fmt.Errorf("decoding SBOM contents %s: %w", path, err)
	}
	packages := make([]sbom.Package, len(encoded))
	for i, pkg := range encoded {
		packages[i] = sbom.Package{
			Name:    pkg.Name,
			Epoch:   pkg.Epoch,
			Version: pkg.Version,
			Release: pkg.Release,
			Arch:    pkg.Arch,
			Vendor:  pkg.Vendor,
			URL:     pkg.URL,
			Checksum: rpmmd.Checksum{
				Type:  pkg.ChecksumType,
				Value: pkg.ChecksumValue,
			},
		}
	}
	return packages, nil
}
