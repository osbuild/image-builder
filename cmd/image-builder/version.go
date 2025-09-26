package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/osbuild/images/pkg/osbuild"
	"gopkg.in/yaml.v3"
)

// Usually set by whatever is building the binary with a `-X main.version=22`, for example
// in `make build`.
var version = "unknown"

type versionDescription struct {
	ImageBuilder struct {
		Version      string `yaml:"version"`
		Commit       string `yaml:"commit"`
		Dependencies struct {
			Images  string `yaml:"images"`
			OSBuild string `yaml:"osbuild"`
		} `yaml:"dependencies"`
	} `yaml:"image-builder"`
}

func readVersionInfo() *versionDescription {
	vd := &versionDescription{}

	// We'll be getting these values from the build info if they're available, otherwise
	// they will always be set to unknown. Note that `version` is set globally so it can
	// be defined by whatever is building this project.
	vd.ImageBuilder.Commit = "unknown"
	vd.ImageBuilder.Version = version
	vd.ImageBuilder.Dependencies.Images = "unknown"
	vd.ImageBuilder.Dependencies.OSBuild = "unknown"

	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, bs := range bi.Settings {
			switch bs.Key {
			case "vcs.revision":
				vd.ImageBuilder.Commit = bs.Value
			}
		}

		for _, dep := range bi.Deps {
			if dep.Path == "github.com/osbuild/images" {
				vd.ImageBuilder.Dependencies.Images = dep.Version
			}
		}
	}

	osbuildVersion, err := osbuild.OSBuildVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get osbuild version: %v\n", err)
	}
	vd.ImageBuilder.Dependencies.OSBuild = osbuildVersion

	return vd
}

func prettyVersion() string {
	var b strings.Builder

	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)

	enc.Encode(readVersionInfo())

	return b.String()
}
