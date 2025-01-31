package main

import (
	"fmt"
	"io/fs"
	"net/url"

	"github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
)

// defaultDataDirs contains the default search paths to look for
// repository data. They contain a bunch of json files of the form
// "$distro_$version".json (but that is an implementation detail that
// the "images" library takes care of).
var defaultDataDirs = []string{
	"/etc/image-builder/repositories",
	"/usr/share/image-builder/repositories",
}

type repoConfig struct {
	DataDir    string
	ExtraRepos []string
}

func parseExtraRepo(extraRepo string) ([]rpmmd.RepoConfig, error) {
	// We want to eventually support more URIs repos here:
	// - config:/path/to/repo.json
	// - copr:@osbuild/osbuild (with full gpg retrival via the copr API)
	// But for now just default to base-urls

	baseURL, err := url.Parse(extraRepo)
	if err != nil {
		return nil, fmt.Errorf("cannot parse extra repo %w", err)
	}
	if baseURL.Scheme == "" {
		return nil, fmt.Errorf(`scheme missing in %q, please prefix with e.g. file:`, extraRepo)
	}

	// TODO: to support gpg checking we will need to add signing keys.
	// We will eventually add support for our own "repo.json" format
	// which is rich enough to contain gpg keys (and more).
	checkGPG := false
	return []rpmmd.RepoConfig{
		{
			Id:           baseURL.String(),
			Name:         baseURL.String(),
			BaseURLs:     []string{baseURL.String()},
			CheckGPG:     &checkGPG,
			CheckRepoGPG: &checkGPG,
		},
	}, nil
}

var newRepoRegistry = func(dataDir string, extraRepos []string) (*reporegistry.RepoRegistry, error) {
	var dataDirs []string
	if dataDir != "" {
		dataDirs = []string{dataDir}
	} else {
		dataDirs = defaultDataDirs
	}

	conf, err := reporegistry.LoadAllRepositories(dataDirs, []fs.FS{repos.FS})
	if err != nil {
		return nil, err
	}

	// XXX: this should probably go into manifestgen.Options as
	// a new Options.ExtraRepoConf eventually (just like OverrideRepos)
	for _, repo := range extraRepos {
		// XXX: this loads the extra repo unconditionally to all
		// distro/arch versions. we do not know in advance where
		// it belongs to
		extraRepo, err := parseExtraRepo(repo)
		if err != nil {
			return nil, err
		}
		for _, repoArchConfigs := range conf {
			for arch := range repoArchConfigs {
				archCfg := repoArchConfigs[arch]
				archCfg = append(archCfg, extraRepo...)
				repoArchConfigs[arch] = archCfg
			}
		}
	}

	return reporegistry.NewFromDistrosRepoConfigs(conf), nil
}
