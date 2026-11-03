package oci

import (
	"fmt"

	"github.com/oracle/oci-go-sdk/v54/common"
	"github.com/oracle/oci-go-sdk/v54/core"
	"github.com/oracle/oci-go-sdk/v54/identity"
)

// Client wraps OCI compute and networking APIs used for boot tests.
type Client struct {
	region         string
	tenancyID      string
	compute        core.ComputeClient
	virtualNetwork core.VirtualNetworkClient
	identity       identity.IdentityClient
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
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("oci identity client: %w", err)
	}

	tenancyID, err := configProvider.TenancyOCID()
	if err != nil {
		return nil, fmt.Errorf("oci config tenancy: %w", err)
	}

	apiRegion, err := configProvider.Region()
	if err != nil {
		return nil, fmt.Errorf("oci config region: %w", err)
	}
	if region != "" {
		computeClient.SetRegion(region)
		vnClient.SetRegion(region)
		identityClient.SetRegion(region)
		apiRegion = region
	}

	return &Client{
		region:         apiRegion,
		tenancyID:      tenancyID,
		compute:        computeClient,
		virtualNetwork: vnClient,
		identity:       identityClient,
	}, nil
}

// Region returns the API region configured for this client.
func (c *Client) Region() string {
	return c.region
}
