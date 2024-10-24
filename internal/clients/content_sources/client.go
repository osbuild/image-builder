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

// returns ApiRepositoryCollectionResponse
func (csc *ContentSourcesClient) GetRepositories(ctx context.Context, repoURLs []string, external bool) (*http.Response, error) {
	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("Unable to get identity from context")
	}

	csReposURL := csc.url.JoinPath("repositories/")
	queryValues := csReposURL.Query()
	queryValues.Add("url", strings.Join(repoURLs, ","))
	if external {
		queryValues.Add("origin", "external")
	} else {
		queryValues.Add("origin", "red_hat")
	}
	csReposURL.RawQuery = queryValues.Encode()

	return csc.request("GET", csReposURL.String(), map[string]string{
		"x-rh-identity": id,
	}, nil)
}

// returns []ApiRepositoryExportResponse
func (csc *ContentSourcesClient) BulkExportRepositories(ctx context.Context, body ApiRepositoryExportRequest) (*http.Response, error) {
	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("Unable to get identity from context")
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
		return nil, fmt.Errorf("Unable to get identity from context")
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
