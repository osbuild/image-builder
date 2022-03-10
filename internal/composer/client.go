//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=composer --generate types -o openapi.v2.gen.go openapi.v2.yml

package composer

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

type AccessToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type ComposerClient struct {
	composerURL string

	tokenURL     string
	offlineToken string
	accessToken  AccessToken
	tokenMu      sync.RWMutex

	client *http.Client
}

func NewClient(composerURL, tokenURL, offlineToken string, ca string) (*ComposerClient, error) {
	if tokenURL == "" {
		return nil, fmt.Errorf("Client needs token endpoint")
	}
	if offlineToken == "" {
		return nil, fmt.Errorf("Client needs offline token")
	}

	client, err := createClient(composerURL, ca)
	if err != nil {
		return nil, fmt.Errorf("Error creating compose http client")
	}

	cc := ComposerClient{
		composerURL:  fmt.Sprintf("%s/api/image-builder-composer/v2", composerURL),
		tokenURL:     tokenURL,
		offlineToken: offlineToken,
		client:       client,
	}

	return &cc, nil
}

func createClient(composerURL string, ca string) (*http.Client, error) {
	if !strings.HasPrefix(composerURL, "https") || ca == "" {
		return &http.Client{}, nil
	}

	var tlsConfig *tls.Config
	caCert, err := ioutil.ReadFile(filepath.Clean(ca))
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    caCertPool,
	}

	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport}, nil
}

func (cc *ComposerClient) request(method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	token := func() string {
		cc.tokenMu.RLock()
		defer cc.tokenMu.RUnlock()
		return cc.accessToken.AccessToken
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token()))

	resp, err := cc.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = cc.refreshToken()
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token()))
		resp, err = cc.client.Do(req)
	}

	return resp, err
}

func (cc *ComposerClient) refreshToken() error {
	cc.tokenMu.Lock()
	defer cc.tokenMu.Unlock()

	if cc.offlineToken == "" || cc.tokenURL == "" {
		return fmt.Errorf("No offline token or oauth url available")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", "rhsm-api")
	data.Set("refresh_token", cc.offlineToken)

	resp, err := http.PostForm(cc.tokenURL, data)
	if err != nil {
		return err
	}

	var at AccessToken
	err = json.NewDecoder(resp.Body).Decode(&at)
	if err != nil {
		return err
	}

	cc.accessToken = at
	return nil
}

func (cc *ComposerClient) ComposeStatus(id string) (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/composes/%s", cc.composerURL, id), nil, nil)
}

func (cc *ComposerClient) ComposeMetadata(id string) (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/composes/%s/metadata", cc.composerURL, id), nil, nil)
}

func (cc *ComposerClient) Compose(compose ComposeRequest) (*http.Response, error) {
	buf, err := json.Marshal(compose)
	if err != nil {
		return nil, err
	}

	return cc.request("POST", fmt.Sprintf("%s/compose", cc.composerURL), map[string]string{"Content-Type": "application/json"}, bytes.NewReader(buf))
}

func (cc *ComposerClient) OpenAPI() (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/openapi", cc.composerURL), nil, nil)
}
