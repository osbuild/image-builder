package content_sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/redhatinsights/identity"
)

type RepositoryByID map[string]ApiRepositoryResponse

type ContentSourcesClient struct {
	url    *url.URL
	client *http.Client
}

type ContentSourcesClientConfig struct {
	URL string
}

func NewClient(conf ContentSourcesClientConfig) (*ContentSourcesClient, error) {
	csURL, err := url.Parse(conf.URL)
	if err != nil {
		return nil, err
	}
	csc := ContentSourcesClient{
		url:    csURL,
		client: &http.Client{},
	}

	return &csc, nil
}

func (csc *ContentSourcesClient) request(method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	return csc.client.Do(req)
}

func (csc *ContentSourcesClient) fetchRepositories(ctx context.Context, repoURLs []string, repoIDs []string, external bool) (*ApiRepositoryCollectionResponse, error) {
	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("unable to get identity from context")
	}

	csReposURL := csc.url.JoinPath("repositories/")
	queryValues := csReposURL.Query()
	if len(repoURLs) > 0 {
		queryValues.Add("url", strings.Join(repoURLs, ","))
	} else if len(repoIDs) > 0 {
		queryValues.Add("uuid", strings.Join(repoIDs, ","))
	} else {
		return nil, fmt.Errorf("at least one repo url or repo id needs to be given")
	}
	if external {
		queryValues.Add("origin", "external,upload")
	} else {
		queryValues.Add("origin", "red_hat")
	}
	csReposURL.RawQuery = queryValues.Encode()

	resp, err := csc.request("GET", csReposURL.String(), map[string]string{
		"x-rh-identity": id,
	}, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode != http.StatusUnauthorized {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("unable to fetch repositories, got %v response, body: %s", resp.StatusCode, body)
			}
		}
		return nil, fmt.Errorf("unable to fetch repositories, got %v response", resp.StatusCode)
	}

	var repos *ApiRepositoryCollectionResponse
	err = json.NewDecoder(resp.Body).Decode(&repos)
	if err != nil {
		return nil, fmt.Errorf("unable to parse repositories: %v", err)
	}

	return repos, nil
}

func (csc *ContentSourcesClient) GetRepositories(ctx context.Context, repoURLs []string, repoIDs []string, external bool) (RepositoryByID, error) {
	result := make(RepositoryByID, len(repoURLs)+len(repoIDs))

	if len(repoURLs) > 0 {
		repos, err := csc.fetchRepositories(ctx, repoURLs, nil, external)
		if err != nil {
			return nil, err
		}
		for _, repo := range *repos.Data {
			result[*repo.Uuid] = repo
		}
	}

	if len(repoIDs) > 0 {
		repos, err := csc.fetchRepositories(ctx, nil, repoIDs, external)
		if err != nil {
			return nil, err
		}
		for _, repo := range *repos.Data {
			result[*repo.Uuid] = repo
		}
	}

	return result, nil
}

// returns []ApiRepositoryExportResponse
func (csc *ContentSourcesClient) BulkExportRepositories(ctx context.Context, body ApiRepositoryExportRequest) (*http.Response, error) {
	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("unable to get identity from context")
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return csc.request("POST", csc.url.JoinPath("repositories", "bulk_export/").String(), map[string]string{
		"x-rh-identity": id,
		"content-type":  "application/json",
	}, bytes.NewReader(buf))
}

// returns ApiListSnapshotByDateResponse
func (csc *ContentSourcesClient) GetSnapshotsForDate(ctx context.Context, body ApiListSnapshotByDateRequest) (*http.Response, error) {
	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("unable to get identity from context")
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return csc.request("POST", csc.url.JoinPath("snapshots", "for_date/").String(), map[string]string{
		"x-rh-identity": id,
		"content-type":  "application/json",
	}, bytes.NewReader(buf))
}
