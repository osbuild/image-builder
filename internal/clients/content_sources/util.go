package content_sources

import (
	"fmt"
	"net/url"
)

func GetBaseURL(repo ApiRepositoryResponse, csReposURL *url.URL) (string, error) {
	if repo.Origin == nil {
		return "", fmt.Errorf("unable to read origin from repository %s", *repo.Uuid)
	}
	switch *repo.Origin {
	case "upload":
		if csReposURL == nil {
			return "", fmt.Errorf("upload repositories require a content sources URL")
		}
		// Snapshot URLs need to be replaced with the internal mtls URL
		repoURL, err := url.Parse(*repo.LatestSnapshotUrl)
		if err != nil {
			return "", err
		}
		return csReposURL.JoinPath(repoURL.Path).String(), nil
	case "external", "red_hat":
		return *repo.Url, nil
	}
	return "", fmt.Errorf("unknown origin on content sources repository %s, origin: %s", *repo.Uuid, *repo.Origin)
}
