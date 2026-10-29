package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZoneRegion(t *testing.T) {
	region, err := zoneRegion("us-central1-a")
	require.NoError(t, err)
	require.Equal(t, "us-central1", region)

	_, err = zoneRegion("invalid")
	require.Error(t, err)
}
