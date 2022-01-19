package dnf_json

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
)

type DumpPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	RepoID  string `json:"repo_id"`
	Summary string `json:"summary"`
}

type Repository struct {
	ID      string `json:"id"`
	Baseurl string `json:"baseurl,omitempty"`
}

type Arguments struct {
	Arch             string       `json:"arch"`
	ModulePlatformID string       `json:"module_platform_id"`
	Repos            []Repository `json:"repos"`
}

type DumpRequest struct {
	Command   string    `json:"command"`
	Arguments Arguments `json:"arguments"`
}

type DumpResponse struct {
	Packages []DumpPackage `json:"packages"`
}

// GetRepositoryIDFromURL returns a repository id that is the same as the one created by dnf
func GetRepositoryIDFromURL(rawUrl string) (string, error) {
	repoUrl, err := url.Parse(rawUrl)
	if err != nil {
		return "", err
	}
	repoID := repoUrl.Host + repoUrl.Path
	repoID = strings.Replace(repoID, "/", "_", -1)
	return repoID, nil
}

// CompareVersions return 1 in case version1 > version2,
// 0 in case the two versions are equal
// and -1 in case version1 < version2
// return error in case version1 or version2 does not pass version validation
func CompareVersions(version1 string, version2 string) (int, error) {

	ver1, err := version.NewVersion(version1)
	if err != nil {
		return 0, err
	}

	ver2, err := version.NewVersion(version2)
	if err != nil {
		return 0, err
	}
	return ver1.Compare(ver2), nil
}

func GetClient() *http.Client {

	return &http.Client{Timeout: 120 * time.Second}
}

// Dump return all the packages contained in the supplied repositories.
func Dump(dnfJsonURL string, arch string, modulePlatformID string, repos []Repository) ([]DumpPackage, error) {
	var packages []DumpPackage

	if len(repos) == 0 {
		return packages, nil
	}

	dumpRequest := DumpRequest{
		Command: "dump",
		Arguments: Arguments{
			Arch:             arch,
			ModulePlatformID: modulePlatformID,
			Repos:            repos,
		},
	}

	for ind := 0; ind < len(repos); ind++ {
		if repos[ind].ID == "" {
			repoID, err := GetRepositoryIDFromURL(repos[ind].Baseurl)
			if err != nil {
				return packages, err
			}
			repos[ind].ID = repoID
		}
	}

	body, err := json.Marshal(dumpRequest)
	if err != nil {
		return packages, err
	}

	client := GetClient()

	resp, err := client.Post(dnfJsonURL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return packages, err
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return packages, err
	}

	dumpResponse := DumpResponse{}

	err = json.Unmarshal(body, &dumpResponse)
	if err != nil {
		return packages, err

	}

	packages = dumpResponse.Packages

	return packages, nil
}

// FilterDumpPackages returns the packages which names contain the search string sorted by name.
func FilterDumpPackages(dumpPackages []DumpPackage, search string) []DumpPackage {

	packages := make([]DumpPackage, 0)
	packagesMap := make(map[string]DumpPackage)
	packagesNames := make([]string, 0)

	if search == "" || len(dumpPackages) == 0 {
		return packages
	}

	search = strings.ToLower(search)

	for _, dumpPackage := range dumpPackages {
		if strings.Contains(strings.ToLower(dumpPackage.Name), search) {
			if mapPackage, ok := packagesMap[dumpPackage.Name]; ok {
				if comparison, err := CompareVersions(dumpPackage.Version, mapPackage.Version); err == nil && comparison > 0 {
					packagesMap[mapPackage.Name] = dumpPackage
				}
			} else {
				packagesMap[dumpPackage.Name] = dumpPackage
				packagesNames = append(packagesNames, dumpPackage.Name)
			}
		}
	}

	sort.Strings(packagesNames)

	for _, packageName := range packagesNames {
		packages = append(packages, packagesMap[packageName])
	}

	return packages
}

// Search for packages in the supplied repositories
func Search(dnfJsonURL string, search string, arch string, modulePlatformID string, repos []Repository) ([]DumpPackage, error) {

	var packages []DumpPackage

	if search == "" || len(repos) == 0 {
		return packages, nil
	}

	packages, err := Dump(dnfJsonURL, arch, modulePlatformID, repos)
	if err != nil {
		return packages, err
	}

	return FilterDumpPackages(packages, search), nil
}
