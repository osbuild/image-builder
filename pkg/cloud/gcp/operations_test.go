package gcp

import (
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/stretchr/testify/require"
)

func TestZoneRegion(t *testing.T) {
	region, err := zoneRegion("us-central1-a")
	require.NoError(t, err)
	require.Equal(t, "us-central1", region)

	_, err = zoneRegion("invalid")
	require.Error(t, err)
}

func TestOperationError(t *testing.T) {
	require.NoError(t, operationError(&computepb.Operation{Status: computepb.Operation_DONE.Enum()}))
	require.ErrorContains(t, operationError(&computepb.Operation{Status: computepb.Operation_RUNNING.Enum()}), "RUNNING")
}
