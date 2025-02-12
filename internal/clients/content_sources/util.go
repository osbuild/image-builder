package content_sources

import (
	"fmt"
)

func GetBaseURL(repo ApiRepositoryResponse) (string, error) {
	if repo.Origin == nil {
		return "", fmt.Errorf("unable to read origin from repository %s", *repo.Uuid)
	}
	switch *repo.Origin {
	case "upload":
		return *repo.LatestSnapshotUrl, nil
	case "external", "red_hat":
		return *repo.Url, nil
	}
	return "", fmt.Errorf("unknown origin on content sources repository %s, origin: %s", *repo.Uuid, *repo.Origin)
}
