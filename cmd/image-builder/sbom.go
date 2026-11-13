package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/osbuild/image-builder/pkg/hashutil"
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
