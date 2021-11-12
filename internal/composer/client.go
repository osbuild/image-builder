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
	"strings"
	"sync"
	"time"
)

type bearerToken struct {
	AccessToken     string `json:"access_token"`
	ValidForSeconds int    `json:"expires_in"`
}

type ComposerClient struct {
	composerURL string

	tokenURL         string
	offlineToken     string
	lastTokenRefresh *time.Time
	bearerToken      *bearerToken
	tokenMu          sync.Mutex

	client *http.Client
}

func NewClient(composerURL, tokenURL, offlineToken string, ca *string) (*ComposerClient, error) {
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

func createClient(composerURL string, ca *string) (*http.Client, error) {
	if !strings.HasPrefix(composerURL, "https") || ca == nil {
		return &http.Client{}, nil
	}

	var tlsConfig *tls.Config
	caCert, err := ioutil.ReadFile(*ca)
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

func (cc *ComposerClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	var d time.Duration
	cc.tokenMu.Lock()
	defer cc.tokenMu.Unlock()
	if cc.lastTokenRefresh != nil {
		d = time.Since(*cc.lastTokenRefresh)
	}
	if cc.bearerToken == nil || d.Seconds() >= (float64(cc.bearerToken.ValidForSeconds)*0.8) {
		err = cc.refreshBearerToken()
		if err != nil {
			return nil, err
		}
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", cc.bearerToken.AccessToken))
	return req, nil
}

// Note: Only call this function with Client.tokenMu locked!
func (cc *ComposerClient) refreshBearerToken() error {
	if cc.offlineToken == "" || cc.tokenURL == "" {
		return fmt.Errorf("No offline token or oauth url available")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", "rhsm-api")
	data.Set("refresh_token", cc.offlineToken)

	t := time.Now()
	resp, err := http.PostForm(cc.tokenURL, data)
	if err != nil {
		return err
	}

	var bt bearerToken
	err = json.NewDecoder(resp.Body).Decode(&bt)
	if err != nil {
		return err
	}

	cc.bearerToken = &bt
	cc.lastTokenRefresh = &t
	return nil
}

func (cc *ComposerClient) ComposeStatus(id string) (*http.Response, error) {
	req, err := cc.newRequest("GET", fmt.Sprintf("%s/composes/%s", cc.composerURL, id), nil)
	if err != nil {
		return nil, err
	}

	return cc.client.Do(req)
}

func (cc *ComposerClient) ComposeMetadata(id string) (*http.Response, error) {
	req, err := cc.newRequest("GET", fmt.Sprintf("%s/composes/%s/metadata", cc.composerURL, id), nil)
	if err != nil {
		return nil, err
	}

	return cc.client.Do(req)
}

func (cc *ComposerClient) Compose(compose ComposeRequest) (*http.Response, error) {
	buf, err := json.Marshal(compose)
	if err != nil {
		return nil, err
	}

	req, err := cc.newRequest("POST", fmt.Sprintf("%s/compose", cc.composerURL), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	return cc.client.Do(req)
}

func (cc *ComposerClient) OpenAPI() (*http.Response, error) {
	req, err := cc.newRequest("GET", fmt.Sprintf("%s/openapi", cc.composerURL), nil)
	if err != nil {
		return nil, err
	}

	return cc.client.Do(req)
}
