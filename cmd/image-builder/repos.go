package main

import (
	"github.com/osbuild/images/pkg/reporegistry"
)

// XXX: copied from "composer", should be exported there so
// that we keep this in sync
// XXX2: means we need to depend on osbuild-composer-common or a new rpm
// that provides the relevant packages *or* we use go:embed (cf images#1038)
//
// defaultDataDirs contains the default search paths to look for repository
// data. Note that the repositories are under a repositories/ sub-directory
// and contain a bunch of json files of the form "$distro_$version".json
// (but that is an implementation detail that the "images" library takes
// care of).
var defaultDataDirs = []string{
	"/etc/osbuild-composer",
	"/usr/share/osbuild-composer",
}

var newRepoRegistry = func(dataDir string) (*reporegistry.RepoRegistry, error) {
	var dataDirs []string
	if dataDir != "" {
		dataDirs = []string{dataDir}
	} else {
		dataDirs = defaultDataDirs
	}

	return reporegistry.New(dataDirs)
}
