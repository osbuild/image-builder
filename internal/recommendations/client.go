//go:generate go run -mod=mod github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config client.cfg.yml recommendations.v3.yml

package recommendations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type RecommendationsClient struct {
	recommendationsURL string
	clientId           string
	clientSecret       string
	client             *http.Client
}

type RecommendationsClientConfig struct {
	RecommendationsURL string
	ClientId           string
	ClientSecret       string
}

var contentHeaders = map[string]string{"Content-Type": "application/json"}

func NewClient(conf RecommendationsClientConfig) (*RecommendationsClient, error) {
	if conf.ClientId == "" {
		return nil, fmt.Errorf("client needs clientId")
	}

	rc := RecommendationsClient{
		recommendationsURL: conf.RecommendationsURL,
		clientId:           conf.ClientId,
		clientSecret:       conf.ClientSecret,
		client:             &http.Client{},
	}

	return &rc, nil
}

func (rc *RecommendationsClient) request(method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	return rc.client.Do(req)
}

func (rc *RecommendationsClient) RecommendationsPackages(recommendationsPackages UploadPackageRequest) (*http.Response, error) {
	buf, err := json.Marshal(recommendationsPackages)
	if err != nil {
		return nil, err
	}

	return rc.request("POST", fmt.Sprintf("%s/api/packages/recommendations", rc.recommendationsURL), contentHeaders, bytes.NewReader(buf))
}
