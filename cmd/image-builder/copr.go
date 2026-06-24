package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/osbuild/image-builder/pkg/rpmmd"
)

var coprBaseURL = "https://copr.fedorainfracloud.org"

type coprProject struct {
	FullName    string
	Owner       string
	Name        string
	ChrootRepos map[string]string
	GPGKeyURL   string
}

type coprAPIResponse struct {
	FullName    string            `json:"full_name"`
	Ownername   string            `json:"ownername"`
	Name        string            `json:"name"`
	ChrootRepos map[string]string `json:"chroot_repos"`
}

var coprHTTPClient = http.DefaultClient

// parseCoprURL extracts the owner and project from a parsed
// copr:// URL. The expected form is copr://owner/project.
func parseCoprURL(u *url.URL) (owner, project string, err error) {
	// TODO: do we want to require hostnames or something? This
	// TODO: all seems to be a bit iffy as we're working around the
	// TODO: url.URL kinda?

	// url.Parse("copr://@osbuild/osbuild") gives:
	//   Opaque: "//@osbuild/osbuild"
	// url.Parse("copr://user/project") gives:
	//   Host: "user", Path: "/project"

	// Rebuild the path from the raw form to handle both uniformly.
	raw := strings.TrimPrefix(u.String(), "copr://")

	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid copr URL %q, expected copr://owner/project (e.g. copr://@osbuild/osbuild)", u)
	}

	return parts[0], parts[1], nil
}

func fetchCoprProject(owner, project string) (*coprProject, error) {
	apiURL := fmt.Sprintf("%s/api_3/project?ownername=%s&projectname=%s",
		coprBaseURL,
		url.QueryEscape(owner),
		url.QueryEscape(project),
	)

	resp, err := coprHTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch copr project %s/%s: %w", owner, project, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("copr project %s/%s not found", owner, project)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copr API returned status %d for project %s/%s", resp.StatusCode, owner, project)
	}

	var apiResp coprAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("cannot decode copr API response for %s/%s: %w", owner, project, err)
	}

	if len(apiResp.ChrootRepos) == 0 {
		return nil, fmt.Errorf("copr project %s/%s has no chroot repos", owner, project)
	}

	gpgKeyURL := fmt.Sprintf("https://download.copr.fedorainfracloud.org/results/%s/%s/pubkey.gpg", owner, project)

	return &coprProject{
		FullName:    apiResp.FullName,
		Owner:       owner,
		Name:        project,
		ChrootRepos: apiResp.ChrootRepos,
		GPGKeyURL:   gpgKeyURL,
	}, nil
}

func (cp *coprProject) repoConfig(chroot, what string, idx int) *rpmmd.RepoConfig {
	baseURL, ok := cp.ChrootRepos[chroot]
	if !ok {
		return nil
	}

	checkGPG := true
	checkRepoGPG := false
	return &rpmmd.RepoConfig{
		Id:           fmt.Sprintf("%s-copr-%v", what, idx),
		Name:         fmt.Sprintf("Copr repo for %s owned by %s", cp.Name, cp.Owner),
		BaseURLs:     []string{baseURL},
		GPGKeys:      []string{cp.GPGKeyURL},
		CheckGPG:     &checkGPG,
		CheckRepoGPG: &checkRepoGPG,
	}
}

// distroToCoprChroots returns candidate COPR chroot prefixes for a
// given distro name. The arch suffix is not included, callers append
// it themselves (see resolveCoprRepo).
func distroToCoprChroots(distroName string) []string {
	// centos-10 → centos-stream-10
	if strings.HasPrefix(distroName, "centos-") {
		ver := strings.TrimPrefix(distroName, "centos-")
		return []string{
			"centos-stream-" + ver,
			distroName,
		}
	}

	// rhel-10.2, rhel-10, then epel-10.2, epel-10 is our order
	// of preference. in the future we might allow for explicit
	// chroot selection through the URL
	if strings.HasPrefix(distroName, "rhel-") {
		ver := strings.TrimPrefix(distroName, "rhel-")
		major, _, hasDot := strings.Cut(ver, ".")
		candidates := []string{distroName}
		if hasDot {
			candidates = append(candidates, "rhel-"+major)
		}
		candidates = append(candidates, "epel-"+ver)
		if hasDot {
			candidates = append(candidates, "epel-"+major)
		}
		return candidates
	}

	return []string{distroName}
}

// resolveCoprRepo fetches the COPR project and returns a RepoConfig
// for the given distro/arch combination. nil if
// the project has no matching chroot.
func resolveCoprRepo(u *url.URL, what string, idx int, distroName, archName string) (*rpmmd.RepoConfig, error) {
	owner, project, err := parseCoprURL(u)
	if err != nil {
		return nil, err
	}

	cp, err := fetchCoprProject(owner, project)
	if err != nil {
		return nil, err
	}

	for _, prefix := range distroToCoprChroots(distroName) {
		chroot := prefix + "-" + archName
		if rc := cp.repoConfig(chroot, what, idx); rc != nil {
			return rc, nil
		}
	}

	return nil, nil
}
