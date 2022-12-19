//go:generate go run -mod=mod github.com/deepmap/oapi-codegen/cmd/oapi-codegen --config client.cfg.yml provisioning.v1.yml

package provisioning

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/redhatinsights/identity"
)

type ProvisioningClient struct {
	url    string
	client *http.Client
}

type ProvisioningClientConfig struct {
	URL string
}

func NewClient(conf ProvisioningClientConfig) (*ProvisioningClient, error) {
	pc := ProvisioningClient{
		url:    conf.URL,
		client: &http.Client{},
	}

	return &pc, nil
}

func (pc *ProvisioningClient) request(method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	return pc.client.Do(req)
}

func (pc *ProvisioningClient) GetAccountID(ctx context.Context, sourceID string) (*http.Response, error) {
	id, ok := identity.GetIdentityHeader(ctx)
	if !ok {
		return nil, fmt.Errorf("Unable to get identity from context")
	}

	return pc.request("GET", fmt.Sprintf("%s/sources/%s/account_identity", pc.url, sourceID), map[string]string{
		"x-rh-identity": id,
	}, nil)
}
