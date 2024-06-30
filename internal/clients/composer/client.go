package composer

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

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ComposerClient struct {
	composerURL string
	tokener     oauth2.Tokener
	client      *http.Client
}

type ComposerClientConfig struct {
	URL     string
	CA      string
	Tokener oauth2.Tokener
}

var contentHeaders = map[string]string{"Content-Type": "application/json"}

func NewClient(conf ComposerClientConfig) (*ComposerClient, error) {
	if conf.URL == "" {
		logrus.Warn("Composer URL not set, client will fail")
	}
	client, err := createClient(conf.URL, conf.CA)
	if err != nil {
		return nil, fmt.Errorf("Error creating compose http client")
	}

	cc := ComposerClient{
		composerURL: fmt.Sprintf("%s/api/image-builder-composer/v2", conf.URL),
		tokener:     conf.Tokener,
		client:      client,
	}

	return &cc, nil
}

func createClient(composerURL string, ca string) (*http.Client, error) {
	if !strings.HasPrefix(composerURL, "https") || ca == "" {
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

func (cc *ComposerClient) request(method, url string, headers map[string]string, body io.ReadSeeker) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	token, err := cc.tokener.Token(req.Context())
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := cc.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		token, err = cc.tokener.ForceRefresh(req.Context())
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		if body != nil {
			_, err = body.Seek(0, io.SeekStart)
			if err != nil {
				return nil, err
			}
		}
		resp, err = cc.client.Do(req)
	}

	return resp, err
}

func (cc *ComposerClient) ComposeStatus(id uuid.UUID) (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/composes/%s", cc.composerURL, id), nil, nil)
}

func (cc *ComposerClient) ComposeMetadata(id uuid.UUID) (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/composes/%s/metadata", cc.composerURL, id), nil, nil)
}

func (cc *ComposerClient) Compose(compose ComposeRequest) (*http.Response, error) {
	buf, err := json.Marshal(compose)
	if err != nil {
		return nil, err
	}

	return cc.request("POST", fmt.Sprintf("%s/compose", cc.composerURL), contentHeaders, bytes.NewReader(buf))
}

func (cc *ComposerClient) OpenAPI() (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/openapi", cc.composerURL), nil, nil)
}

func (cc *ComposerClient) CloneCompose(id uuid.UUID, clone CloneComposeBody) (*http.Response, error) {
	buf, err := json.Marshal(clone)
	if err != nil {
		return nil, err
	}
	return cc.request("POST", fmt.Sprintf("%s/composes/%s/clone", cc.composerURL, id), contentHeaders, bytes.NewReader(buf))
}

func (cc *ComposerClient) CloneStatus(id uuid.UUID) (*http.Response, error) {
	return cc.request("GET", fmt.Sprintf("%s/clones/%s", cc.composerURL, id), nil, nil)
}
