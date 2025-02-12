package main

import (
	"fmt"
	"io/fs"
	"net/url"

	"github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/arch"
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

func parseRepoURLs(repoURLs []string, what string) ([]rpmmd.RepoConfig, error) {
	var repoConf []rpmmd.RepoConfig

	for i, repoURL := range repoURLs {
		// We want to eventually support more URIs repos here:
		// - config:/path/to/repo.json
		// - copr:@osbuild/osbuild (with full gpg retrival via the copr API)
		// But for now just default to base-urls

		baseURL, err := url.Parse(repoURL)
		if err != nil {
			return nil, fmt.Errorf("cannot parse extra repo %w", err)
		}
		if baseURL.Scheme == "" {
			return nil, fmt.Errorf(`scheme missing in %q, please prefix with e.g. file:// or https://`, repoURL)
		}

		// TODO: to support gpg checking we will need to add signing keys.
		// We will eventually add support for our own "repo.json" format
		// which is rich enough to contain gpg keys (and more).
		checkGPG := false
		repoConf = append(repoConf, rpmmd.RepoConfig{
			Id:           fmt.Sprintf("%s-repo-%v", what, i),
			Name:         fmt.Sprintf("%s repo#%v %s%s", what, i, baseURL.Host, baseURL.Path),
			BaseURLs:     []string{baseURL.String()},
			CheckGPG:     &checkGPG,
			CheckRepoGPG: &checkGPG,
		})
	}

	return repoConf, nil
}

func newRepoRegistryImpl(dataDir string, extraRepos []string) (*reporegistry.RepoRegistry, error) {
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
	repoConf, err := parseRepoURLs(extraRepos, "extra")
	if err != nil {
		return nil, err
	}
	// Only add extra repos for the host architecture. We do not support
	// cross-building (yet) so this is fine. Once we support cross-building
	// this needs to move (probably into manifestgen) because at this
	// level we we do not know (yet) what manifest we will generate.
	myArch := arch.Current().String()
	for _, repoArchConfigs := range conf {
		archCfg := repoArchConfigs[myArch]
		archCfg = append(archCfg, repoConf...)
		repoArchConfigs[myArch] = archCfg
	}

	return reporegistry.NewFromDistrosRepoConfigs(conf), nil
}

// this is a variable to make it overridable in tests
var newRepoRegistry = newRepoRegistryImpl
