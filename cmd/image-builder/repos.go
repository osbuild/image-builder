package main

import (
	"io/fs"

	"github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/reporegistry"
)

// defaultDataDirs contains the default search paths to look for
// repository data. They contain a bunch of json files of the form
// "$distro_$version".json (but that is an implementation detail that
// the "images" library takes care of).
var defaultDataDirs = []string{
	"/etc/image-builder/repositories",
	"/usr/share/image-builder/repositories",
}

var newRepoRegistry = func(dataDir string) (*reporegistry.RepoRegistry, error) {
	var dataDirs []string
	if dataDir != "" {
		dataDirs = []string{dataDir}
	} else {
		dataDirs = defaultDataDirs
	}

	return reporegistry.New(dataDirs, []fs.FS{repos.FS})
}
