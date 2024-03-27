package recommendations

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type RecommendationsClient struct {
	URL          string
	tokenURL     string
	accessToken  string
	clientId     string
	clientSecret string
	client       *http.Client
	tokenMu      sync.RWMutex
}

type RecommendationsClientConfig struct {
	URL          string
	CA           string
	TokenURL     string
	ClientId     string
	ClientSecret string
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
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
		URL:          conf.URL,
		tokenURL:     conf.TokenURL,
		clientId:     conf.ClientId,
		clientSecret: conf.ClientSecret,
		client:       client,
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

	token := func() string {
		rc.tokenMu.RLock()
		defer rc.tokenMu.RUnlock()
		return rc.accessToken
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token()))

	resp, err := rc.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		err = rc.refreshToken()
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token()))
		_, err = body.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		resp, err = rc.client.Do(req)
	}
	return resp, err
}

func (rc *RecommendationsClient) refreshToken() error {
	rc.tokenMu.Lock()
	defer rc.tokenMu.Unlock()

	data := url.Values{}
	if rc.clientSecret != "" {
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", rc.clientId)
		data.Set("client_secret", rc.clientSecret)
	}

	resp, err := http.PostForm(rc.tokenURL, data)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Errorf("Error closing body after refreshing composer client token: %v", err)
		}
	}()

	var tr tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tr)
	if err != nil {
		return err
	}

	rc.accessToken = tr.AccessToken
	return nil
}

func (rc *RecommendationsClient) RecommendationsPackages(recommendationsPackages RecommendPackageRequest) (*http.Response, error) {
	buf, err := json.Marshal(recommendationsPackages)
	if err != nil {
		return nil, err
	}
	return rc.request("POST", fmt.Sprintf("%s/api/packages/recommendations", rc.URL), contentHeaders, bytes.NewReader(buf))
}
