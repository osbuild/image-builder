package imagefilter

import (
	"errors"
	"fmt"

	"github.com/osbuild/image-builder/v73/pkg/distro"
	"github.com/osbuild/image-builder/v73/pkg/distrofactory"
	"github.com/osbuild/image-builder/v73/pkg/distrosort"
	"github.com/osbuild/image-builder/v73/pkg/reporegistry"
	"github.com/osbuild/image-builder/v73/pkg/rpmmd"
)

type MinimalRepoRegistry interface {
	ListDistros() []string
	ReposByImageTypeName(distro, arch, imageType string) ([]rpmmd.RepoConfig, error)
}

// Result contains a result from a imagefilter.Filter run
type Result struct {
	ImgType distro.ImageType
	Repos   []rpmmd.RepoConfig
}

// ImageFilter is an a flexible way to filter the available images.
type ImageFilter struct {
	fac   *distrofactory.Factory
	repos MinimalRepoRegistry
}

// New creates a new ImageFilter that can be used to filter the list
// of available images
func New(fac *distrofactory.Factory, repos MinimalRepoRegistry) (*ImageFilter, error) {
	if fac == nil {
		return nil, fmt.Errorf("cannot create ImageFilter without a valid distrofactory")
	}
	if repos == nil {
		return nil, fmt.Errorf("cannot create ImageFilter without a valid reporegistry")
	}

	return &ImageFilter{fac: fac, repos: repos}, nil
}

// Filter filters the available images for the given
// distrofactory/reporegistry based on the given filter terms. Glob
// like patterns (?, *) are supported, see fnmatch(3).
//
// Without a prefix in the filter term a simple name filtering is performed.
// With a prefix the specified property is filtered, e.g. "arch:i386". Adding
// filtering will narrow down the filtering (terms are combined via AND).
//
// The following prefixes are supported:
// "distro:" - the distro name, e.g. rhel-9, or fedora*
// "arch:" - the architecture, e.g. x86_64
// "type": - the image type, e.g. ami, or qcow?
// "bootmode": - the bootmode, e.g. "legacy", "uefi", "hybrid"
func (i *ImageFilter) Filter(searchTerms ...string) ([]Result, error) {
	var res []Result

	distroNames := i.repos.ListDistros()
	filter, err := newFilter(searchTerms...)
	if err != nil {
		return nil, err
	}

	if err := distrosort.Names(distroNames); err != nil {
		return nil, err
	}
	for _, distroName := range distroNames {
		distro := i.fac.GetDistro(distroName)
		if distro == nil {
			// XXX: log here?
			continue
		}
		for _, archName := range distro.ListArches() {
			a, err := distro.GetArch(archName)
			if err != nil {
				return nil, err
			}
			for _, imgTypeName := range a.ListImageTypes() {
				imgType, err := a.GetImageType(imgTypeName)
				if err != nil {
					return nil, err
				}
				if filter.Matches(distro, a, imgType) {
					repos, err := i.repos.ReposByImageTypeName(distroName, archName, imgTypeName)
					if errors.Is(err, reporegistry.ErrNoRepoFound) {
						// skip the image if no repositories are found, we cannot build it without repos (except bootc images but those do not use ImageFilter)
						continue
					}
					if err != nil {
						return nil, err
					}
					res = append(res, Result{imgType, repos})
				}
			}
		}
	}

	return res, nil
}
