// Standalone executable that lists all supported combinations of distribution,
// architecture, and image type. Flags can be specified to filter the list.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gobwas/glob"
	"github.com/osbuild/image-builder/v73/pkg/distrofactory"
	testrepos "github.com/osbuild/image-builder/v73/test/data/repositories"
)

type multiValue []string

func (mv *multiValue) String() string {
	return strings.Join(*mv, ", ")
}

func (mv *multiValue) Set(v string) error {
	split := strings.Split(v, ",")
	*mv = split
	return nil
}

// resolveArgValues returns a list of valid values from the list of values on the
// command line. Invalid values are returned separately. Globs are expanded.
// If the args are empty, the valueList is returned as is.
func resolveArgValues(args multiValue, valueList []string) ([]string, []string) {
	if len(args) == 0 {
		return valueList, nil
	}
	selection := make([]string, 0, len(args))
	invalid := make([]string, 0, len(args))
	for _, arg := range args {
		g := glob.MustCompile(arg)
		match := false
		for _, v := range valueList {
			if g.Match(v) {
				selection = append(selection, v)
				match = true
			}
		}
		if !match {
			invalid = append(invalid, arg)
		}
	}
	return selection, invalid
}

type config struct {
	Distro    string `json:"distro"`
	Arch      string `json:"arch"`
	ImageType string `json:"image-type"`
}

func jsonPrint(configs []config) {
	out, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal configs to json")
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func main() {
	var arches, distros, imgTypes multiValue
	var json bool
	flag.Var(&arches, "arches", "comma-separated list of architectures (globs supported)")
	flag.Var(&distros, "distros", "comma-separated list of distributions (globs supported)")
	flag.Var(&imgTypes, "types", "comma-separated list of image types (globs supported)")
	flag.BoolVar(&json, "json", false, "print configs as json")
	flag.Parse()

	testedRepoRegistry, err := testrepos.New()
	if err != nil {
		panic(fmt.Sprintf("failed to create repo registry with tested distros: %v", err))
	}
	distroFac := distrofactory.NewDefault()
	distros, invalidDistros := resolveArgValues(distros, testedRepoRegistry.ListDistros())
	if len(invalidDistros) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid distro names: [%s]\n", strings.Join(invalidDistros, ","))
	}

	configs := make([]config, 0)
	for _, distroName := range distros {
		distribution := distroFac.GetDistro(distroName)
		if distribution == nil {
			fmt.Fprintf(os.Stderr, "WARNING: invalid distro name %q", distroName)
			continue
		}

		distroArches, invalidArches := resolveArgValues(arches, distribution.ListArches())
		if len(invalidArches) > 0 {
			fmt.Fprintf(os.Stderr, "WARNING: invalid arch names [%s] for distro %q\n", strings.Join(invalidArches, ","), distroName)
		}
		for _, archName := range distroArches {
			arch, err := distribution.GetArch(archName)
			if err != nil {
				// resolveArgValues should prevent this
				panic(fmt.Sprintf("invalid arch name %q for distro %q: %s\n", archName, distroName, err.Error()))
			}

			daImgTypes, invalidImageTypes := resolveArgValues(imgTypes, arch.ListImageTypes())
			if len(invalidImageTypes) > 0 {
				fmt.Fprintf(os.Stderr, "WARNING: invalid image type names [%s] for distro %q and arch %q\n", strings.Join(invalidImageTypes, ","), distroName, archName)
			}
			for _, imgTypeName := range daImgTypes {
				imgType, err := arch.GetImageType(imgTypeName)
				if err != nil {
					// resolveArgValues should prevent this
					panic(fmt.Sprintf("invalid image type %q for distro %q and arch %q: %s\n", imgTypeName, distroName, archName, err.Error()))
				}

				c := config{
					Distro:    distroName,
					Arch:      archName,
					ImageType: imgType.Name(),
				}

				configs = append(configs, c)

			}
		}
	}

	if json {
		jsonPrint(configs)
	} else {
		for _, c := range configs {
			fmt.Printf("%s %s %s\n", c.Distro, c.Arch, c.ImageType)
		}
	}
}
