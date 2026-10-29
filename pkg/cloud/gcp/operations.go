package gcp

import (
	"context"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/option"
)

// waitGlobalOperation blocks until a global Compute Engine operation completes.
func (g *GCP) waitGlobalOperation(ctx context.Context, project, operationName string) error {
	client, err := compute.NewGlobalOperationsRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("waitGlobalOperation: %w", err)
	}
	defer client.Close()

	op, err := client.Wait(ctx, &computepb.WaitGlobalOperationRequest{
		Project:   project,
		Operation: operationName,
	})
	if err != nil {
		return fmt.Errorf("waitGlobalOperation Wait: %w", err)
	}
	return operationError(op)
}

// waitZoneOperation blocks until a zonal Compute Engine operation completes.
func (g *GCP) waitZoneOperation(ctx context.Context, project, zone, operationName string) error {
	client, err := compute.NewZoneOperationsRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("waitZoneOperation: %w", err)
	}
	defer client.Close()

	op, err := client.Wait(ctx, &computepb.WaitZoneOperationRequest{
		Project:   project,
		Zone:      zone,
		Operation: operationName,
	})
	if err != nil {
		return fmt.Errorf("waitZoneOperation Wait: %w", err)
	}
	return operationError(op)
}

func operationError(op *computepb.Operation) error {
	if op.GetHttpErrorStatusCode() != 0 {
		return fmt.Errorf("operation failed: HTTP %d: %s", op.GetHttpErrorStatusCode(), op.GetHttpErrorMessage())
	}
	if op.GetStatus() == computepb.Operation_DONE {
		return nil
	}
	return fmt.Errorf("operation finished with status %s", op.GetStatus().String())
}

func zoneRegion(zone string) (string, error) {
	idx := strings.LastIndex(zone, "-")
	if idx <= 0 {
		return "", fmt.Errorf("zoneRegion: invalid zone %q", zone)
	}
	return zone[:idx], nil
}
