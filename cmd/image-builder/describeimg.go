package main

import (
	"fmt"
	"io"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/imagefilter"
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
}

type packagesYAML struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

func packageSetsFor(imgType distro.ImageType) (map[string]*packagesYAML, error) {
	var bp blueprint.Blueprint
	manifest, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
	if err != nil {
		return nil, err
	}

	res := make(map[string]*packagesYAML)

	for pipelineName, pkgSets := range manifest.GetPackageSetChains() {
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

	outYaml := &describeImgYAML{
		Distro:           img.Distro.Name(),
		OsVersion:        img.Distro.OsVersion(),
		Arch:             img.Arch.Name(),
		Type:             img.ImgType.Name(),
		Bootmode:         img.ImgType.BootMode().String(),
		PartitionType:    img.ImgType.PartitionType().String(),
		DefaultFilename:  img.ImgType.Filename(),
		BuildPipelines:   img.ImgType.BuildPipelines(),
		PayloadPipelines: img.ImgType.PayloadPipelines(),
		Packages:         pkgSets,
	}
	// deliberately break the yaml until the feature is stable
	fmt.Fprint(out, "@WARNING - the output format is not stable yet and may change\n")
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	return enc.Encode(outYaml)
}
