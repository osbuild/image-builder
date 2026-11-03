package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v54/common"
	"github.com/oracle/oci-go-sdk/v54/core"
	"github.com/oracle/oci-go-sdk/v54/identity"
)

// AvailabilityDomainForShape picks an AD where the shape is offered (composer oci.sh parity).
func (c *Client) AvailabilityDomainForShape(ctx context.Context, compartmentID, shape string) (string, error) {
	adResp, err := c.identity.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(c.tenancyID),
	})
	if err != nil {
		return "", fmt.Errorf("ListAvailabilityDomains: %w", err)
	}
	for _, ad := range adResp.Items {
		if ad.Name == nil {
			continue
		}
		shapesResp, err := c.compute.ListShapes(ctx, core.ListShapesRequest{
			CompartmentId:      common.String(compartmentID),
			AvailabilityDomain: ad.Name,
		})
		if err != nil {
			return "", fmt.Errorf("ListShapes in %s: %w", *ad.Name, err)
		}
		for _, item := range shapesResp.Items {
			if item.Shape != nil && *item.Shape == shape {
				return *ad.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no availability domain offers shape %q in compartment %s", shape, compartmentID)
}
