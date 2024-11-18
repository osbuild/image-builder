package main

import (
	"github.com/osbuild/images/pkg/reporegistry"
)

// XXX: copied from "composer", should be exported there so
// that we keep this in sync
// XXX2: means we need to depend on osbuild-composer-common or a new rpm
// that provides the relevant packages *or* we use go:embed (cf images#1038)
var repositoryConfigs = []string{
	"/etc/osbuild-composer",
	"/usr/share/osbuild-composer",
}

var newRepoRegistry = func() (*reporegistry.RepoRegistry, error) {
	// TODO: add a extraReposPaths here so that users can do
	// "ibuilder --repositories ..." to add a custom path(s)

	return reporegistry.New(repositoryConfigs)
}
