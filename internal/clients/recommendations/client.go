package recommendations

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder/internal/oauth2"
	"github.com/sirupsen/logrus"
)

type RecommendationsClient struct {
	URL     string
	tokener oauth2.Tokener
	client  *http.Client
}

type RecommendationsClientConfig struct {
	URL     string
	CA      string
	Tokener oauth2.Tokener
}

var contentHeaders = map[string]string{"Content-Type": "application/json"}

func NewClient(conf RecommendationsClientConfig) (*RecommendationsClient, error) {
	if conf.URL == "" {
		logrus.Warn("Recommendation URL not set, client will fail")
	}
	client, err := createClient(conf.URL, conf.CA)
	if err != nil {
		return nil, fmt.Errorf("error creating recommend http client")
	}

	rc := RecommendationsClient{
		URL:     conf.URL,
		tokener: conf.Tokener,
		client:  client,
	}

	return &rc, nil
}

func createClient(recommendationsURL string, ca string) (*http.Client, error) {
	if !strings.HasPrefix(recommendationsURL, "https") || ca == "" {
		return &http.Client{}, nil
	}

	var tlsConfig *tls.Config
	caCert, err := os.ReadFile(filepath.Clean(ca))
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    caCertPool,
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport}, nil
}

func (rc *RecommendationsClient) request(method, url string, headers map[string]string, body io.ReadSeeker) (*http.Response, error) {
	if rc.URL == "" {
		return nil, fmt.Errorf("recommendation client URL was not set")

	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	token, err := rc.tokener.Token(req.Context())
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := rc.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		token, err = rc.tokener.ForceRefresh(req.Context())
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		_, err = body.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		resp, err = rc.client.Do(req)
	}
	return resp, err
}

func (rc *RecommendationsClient) RecommendationsPackages(recommendationsPackages RecommendPackageRequest) (*http.Response, error) {
	buf, err := json.Marshal(recommendationsPackages)
	if err != nil {
		return nil, err
	}
	return rc.request("POST", fmt.Sprintf("%s/packages/recommendations", rc.URL), contentHeaders, bytes.NewReader(buf))
}
