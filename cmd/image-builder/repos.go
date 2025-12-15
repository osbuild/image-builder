package main

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	"github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
)

// defaultRepoDirs contains the default search paths to look for
// repository data. They contain a bunch of json files of the form
// "$distro_$version".json (but that is an implementation detail that
// the "images" library takes care of).
var defaultRepoDirs = []string{
	"/etc/image-builder/repositories",
	"/usr/share/image-builder/repositories",
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

func newRepoRegistryImpl(repoDir string, extraRepos []string) (*reporegistry.RepoRegistry, error) {
	var repoDirs []string
	var builtins []fs.FS

	if repoDir != "" {
		withRepoSubdir := filepath.Join(repoDir, "repositories")
		if _, err := os.Stat(withRepoSubdir); err == nil {
			// we don't care about the error case here, we just want to know
			// if it exists; not if we can't read it or other errors
			fmt.Fprintf(os.Stderr, "WARNING: found a `repositories` subdirectory at '%s', in the future `image-builder` will not descend into this subdirectory to look for repository files. Please move any repository files directly into the directory '%s' and remove the `repositories` subdirectory to silence this warning.\n", withRepoSubdir, repoDir)
		}
		repoDirs = []string{withRepoSubdir, repoDir}
	} else {
		repoDirs = defaultRepoDirs
		builtins = []fs.FS{repos.FS}
	}

	conf, err := reporegistry.LoadAllRepositories(repoDirs, builtins)
	if err != nil {
		return nil, err
	}

	// Add extra repos to all architecture. We support
	// cross-building but at this level here we don't know yet
	// what manifests will be generated so we must (for now)
	// rely on the user to DTRT with extraRepos.
	//
	// XXX: this should probably go into manifestgen.Options as a
	// new Options.ExtraRepoConf eventually (just like
	// OverrideRepos)
	repoConf, err := parseRepoURLs(extraRepos, "extra")
	if err != nil {
		return nil, err
	}
	for _, repoArchConfigs := range conf {
		for arch := range repoArchConfigs {
			archCfg := repoArchConfigs[arch]
			archCfg = append(archCfg, repoConf...)
			repoArchConfigs[arch] = archCfg
		}
	}

	return reporegistry.NewFromDistrosRepoConfigs(conf), nil
}

// this is a variable to make it overridable in tests
var newRepoRegistry = newRepoRegistryImpl
