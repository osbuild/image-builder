//go:generate go run -mod=mod github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=composer --generate types -o openapi.v2.gen.go openapi.v2.yml

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

type ComposerClient struct {
	composerURL string

	tokenURL     string
	offlineToken string
	accessToken  string
	clientId     string
	clientSecret string
	tokenMu      sync.RWMutex

	client *http.Client
}

type ComposerClientConfig struct {
	ComposerURL  string
	CA           string
	TokenURL     string
	ClientId     string
	OfflineToken string
	ClientSecret string
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

func NewClient(conf ComposerClientConfig) (*ComposerClient, error) {
	if conf.TokenURL == "" {
		return nil, fmt.Errorf("Client needs token endpoint")
	}
	if conf.ClientId == "" {
		return nil, fmt.Errorf("Client needs clientId")
	}
	if conf.OfflineToken == "" && conf.ClientSecret == "" {
		return nil, fmt.Errorf("Client needs offline token, or client secret")
	}

	client, err := createClient(conf.ComposerURL, conf.CA)
	if err != nil {
		return nil, fmt.Errorf("Error creating compose http client")
	}

	cc := ComposerClient{
		composerURL:  fmt.Sprintf("%s/api/image-builder-composer/v2", conf.ComposerURL),
		tokenURL:     conf.TokenURL,
		clientId:     conf.ClientId,
		offlineToken: conf.OfflineToken,
		clientSecret: conf.ClientSecret,
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
		return cc.accessToken
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

	data := url.Values{}
	if cc.offlineToken != "" {
		data.Set("grant_type", "refresh_token")
		data.Set("client_id", cc.clientId)
		data.Set("refresh_token", cc.offlineToken)
	}
	if cc.clientSecret != "" {
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", cc.clientId)
		data.Set("client_secret", cc.clientSecret)
	}

	resp, err := http.PostForm(cc.tokenURL, data)
	if err != nil {
		return err
	}

	var tr tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tr)
	if err != nil {
		return err
	}

	cc.accessToken = tr.AccessToken
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
