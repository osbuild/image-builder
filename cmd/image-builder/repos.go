package main

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/reporegistry"
)

// defaultDataDirs contains the default search paths to look for repository
// data. Note that the repositories are under a repositories/ sub-directory
// and contain a bunch of json files of the form "$distro_$version".json
// (but that is an implementation detail that the "images" library takes
// care of).
var defaultDataDirs = []string{
	"/etc/image-builder",
	"/usr/share/image-builder",
}

var newRepoRegistry = func(dataDir string) (*reporegistry.RepoRegistry, error) {
	var dataDirs []string
	if dataDir != "" {
		dataDirs = []string{dataDir}
	} else {
		dataDirs = defaultDataDirs
	}

	// XXX: think about sharing this with reporegistry?
	var fses []fs.FS
	for _, d := range dataDirs {
		fses = append(fses, os.DirFS(filepath.Join(d, "repositories")))
	}
	fses = append(fses, repos.FS)

	// XXX: should we support disabling the build-ins somehow?
	conf, err := reporegistry.LoadAllRepositoriesFromFS(fses)
	if err != nil {
		return nil, err
	}
	return reporegistry.NewFromDistrosRepoConfigs(conf), nil
}
