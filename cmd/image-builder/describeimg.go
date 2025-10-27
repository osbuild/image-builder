package main

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
)

// Use yaml output by default because it is both nicely human and
// machine readable and parts of our image defintions will be written
// in yaml too.  This means this should be a possible input a
// "flattended" image definiton.
type describeImgYAML struct {
	Distro string `yaml:"distro"`
	Type   string `yaml:"type"`
	Arch   string `yaml:"arch"`

	// XXX: think about ordering (as this is what the user will see)
	OsVersion string `yaml:"os_version"`

	Bootmode        string `yaml:"bootmode"`
	PartitionType   string `yaml:"partition_type"`
	DefaultFilename string `yaml:"default_filename"`

	BuildPipelines   []string                 `yaml:"build_pipelines"`
	PayloadPipelines []string                 `yaml:"payload_pipelines"`
	Packages         map[string]*packagesYAML `yaml:"packages"`

	PartitionTable *disk.PartitionTable `yaml:"partition_table,omitempty"`
}

type packagesYAML struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

func dummyManifestFor(imgType distro.ImageType) (*manifest.Manifest, error) {
	var bp blueprint.Blueprint
	// XXX: '*-simplified-installer' images require the installation device to be specified as a BP customization.
	// Workaround this for now by setting a dummy device. We should ideally have a way to get image type pkg sets
	// without doing this.
	if strings.HasSuffix(imgType.Name(), "-simplified-installer") {
		bp.Customizations = &blueprint.Customizations{
			InstallationDevice: "/dev/dummy",
		}
	}

	var imgOpts distro.ImageOptions
	// Mock ostree options for ostree-based images to make describe work
	if imgType.OSTreeRef() != "" {
		imgOpts.OSTree = &ostree.ImageOptions{
			URL: "http://example.com/repo",
		}
	}

	manifest, _, err := imgType.Manifest(&bp, imgOpts, nil, nil)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func packageSetsFor(imgType distro.ImageType) (map[string]*packagesYAML, error) {
	manifest, err := dummyManifestFor(imgType)
	if err != nil {
		return nil, err
	}

	res := make(map[string]*packagesYAML)

	pkgSetChains, err := manifest.GetPackageSetChains()
	if err != nil {
		return nil, err
	}
	for pipelineName, pkgSets := range pkgSetChains {
		incM := map[string]bool{}
		excM := map[string]bool{}
		for _, pkgSet := range pkgSets {
			for _, s := range pkgSet.Include {
				incM[s] = true
			}
			for _, s := range pkgSet.Exclude {
				excM[s] = true
			}
		}
		inc := make([]string, 0, len(incM))
		exc := make([]string, 0, len(excM))
		for name := range incM {
			inc = append(inc, name)
		}
		for name := range excM {
			exc = append(exc, name)
		}
		slices.Sort(inc)
		slices.Sort(exc)

		res[pipelineName] = &packagesYAML{
			Include: inc,
			Exclude: exc,
		}
	}
	return res, nil
}

// XXX: should this live in images instead?
func describeImage(img *imagefilter.Result, out io.Writer) error {
	// see
	// https://github.com/osbuild/images/pull/1019#discussion_r1832376568
	// for what is available on an image (without depsolve or partitioning)
	pkgSets, err := packageSetsFor(img.ImgType)
	if err != nil {
		return err
	}
	partTable, err := img.ImgType.BasePartitionTable()
	if err != nil && !errors.Is(err, defs.ErrNoPartitionTableForImgType) {
		return err
	}
	m, err := dummyManifestFor(img.ImgType)
	if err != nil {
		return err
	}

	arch := img.ImgType.Arch()
	distro := arch.Distro()
	outYaml := &describeImgYAML{
		Distro:           distro.Name(),
		OsVersion:        distro.OsVersion(),
		Arch:             arch.Name(),
		Type:             img.ImgType.Name(),
		Bootmode:         img.ImgType.BootMode().String(),
		PartitionType:    img.ImgType.PartitionType().String(),
		DefaultFilename:  img.ImgType.Filename(),
		BuildPipelines:   m.BuildPipelines(),
		PayloadPipelines: m.PayloadPipelines(),
		Packages:         pkgSets,
		PartitionTable:   partTable,
	}
	// deliberately break the yaml until the feature is stable
	fmt.Fprint(out, "@WARNING - the output format is not stable yet and may change\n")
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	return enc.Encode(outYaml)
}
