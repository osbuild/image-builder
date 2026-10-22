package oci

import (
	"fmt"

	"github.com/oracle/oci-go-sdk/v54/common"
	"github.com/oracle/oci-go-sdk/v54/core"
)

// Client wraps OCI compute and networking APIs used for boot tests.
type Client struct {
	region         string
	compute        core.ComputeClient
	virtualNetwork core.VirtualNetworkClient
}

// NewClient builds a client from ~/.oci/config; non-empty region overrides the API endpoint.
func NewClient(region string) (*Client, error) {
	configProvider := common.DefaultConfigProvider()

	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("oci compute client: %w", err)
	}
	vnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("oci virtual network client: %w", err)
	}

	apiRegion, err := configProvider.Region()
	if err != nil {
		return nil, fmt.Errorf("oci config region: %w", err)
	}
	if region != "" {
		computeClient.SetRegion(region)
		vnClient.SetRegion(region)
		apiRegion = region
	}

	return &Client{
		region:         apiRegion,
		compute:        computeClient,
		virtualNetwork: vnClient,
	}, nil
}

// Region returns the API region configured for this client.
func (c *Client) Region() string {
	return c.region
}
