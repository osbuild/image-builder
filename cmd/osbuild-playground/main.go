package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/distro"
	"github.com/osbuild/image-builder/v73/pkg/distrofactory"
	"github.com/osbuild/image-builder/v73/pkg/image"
	"github.com/osbuild/image-builder/v73/pkg/reporegistry"
)

var ImageTypes = make(map[string]image.ImageKind)

func AddImageType(img image.ImageKind) {
	ImageTypes[img.Name()] = img
}

func main() {
	var distroArg string
	flag.StringVar(&distroArg, "distro", "host", "distro to build from")
	var archArg string
	flag.StringVar(&archArg, "arch", arch.Current().String(), "architecture to build for")
	var imageTypeArg string
	flag.StringVar(&imageTypeArg, "type", "my-container", "image type to build")
	flag.Parse()

	// Path to options or '-' for stdin
	optionsArg := flag.Arg(0)

	img := ImageTypes[imageTypeArg]
	if optionsArg != "" {
		var reader io.Reader
		if optionsArg == "-" {
			reader = os.Stdin
		} else {
			var err error
			reader, err = os.Open(optionsArg)
			if err != nil {
				panic("Could not open path to image options: " + err.Error())
			}
		}
		file, err := io.ReadAll(reader)
		if err != nil {
			panic("Could not read image options: " + err.Error())
		}
		err = json.Unmarshal(file, img)
		if err != nil {
			panic("Could not parse image options: " + err.Error())
		}
	}

	distroFac := distrofactory.NewDefault()
	var d distro.Distro
	if distroArg == "host" {
		d = distroFac.FromHost()
		if d == nil {
			panic("host distro not supported")
		}
	} else {
		d = distroFac.GetDistro(distroArg)
		if d == nil {
			panic(fmt.Sprintf("distro '%s' not supported\n", distroArg))
		}
	}

	arch, err := d.GetArch(archArg)
	if err != nil {
		panic(fmt.Sprintf("arch '%s' not supported\n", archArg))
	}

	repos, err := reporegistry.LoadRepositories([]string{"./"}, d.Name())
	if err != nil {
		panic("could not load repositories for distro " + d.Name())
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}

	state_dir := path.Join(home, ".local/share/osbuild-playground/")

	RunPlayground(img, d, arch, repos, state_dir)
}
