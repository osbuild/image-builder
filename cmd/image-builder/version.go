package main

import (
	"runtime/debug"
	"strings"

	"gopkg.in/yaml.v3"
)

// Usually set by whatever is building the binary with a `-x main.version=22`, for example
// in `make build`.
var version = "unknown"

type versionDescription struct {
	ImageBuilder struct {
		Version      string `yaml:"version"`
		Commit       string `yaml:"commit"`
		Dependencies struct {
			Images string `yaml:"images"`
		} `yaml:"dependencies"`
	} `yaml:"image-builder"`
}

func readVersionInfo() *versionDescription {
	// We'll be getting these values from the build info if they're available, otherwise
	// they will always be set to unknown. Note that `version` is set globally so it can
	// be defined by whatever is building this project.
	commit := "unknown"
	images := "unknown"

	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, bs := range bi.Settings {
			switch bs.Key {
			case "vcs.revision":
				commit = bs.Value
			}
		}

		for _, dep := range bi.Deps {
			if dep.Path == "github.com/osbuild/images" {
				images = dep.Version
			}
		}
	}

	vd := &versionDescription{}

	vd.ImageBuilder.Version = version
	vd.ImageBuilder.Commit = commit

	vd.ImageBuilder.Dependencies.Images = images

	return vd
}

func prettyVersion() string {
	var b strings.Builder

	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)

	enc.Encode(readVersionInfo())

	return b.String()
}
