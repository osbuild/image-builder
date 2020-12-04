package cloudapi

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewOsbuildClientWithoutCerts(t *testing.T) {
	client := NewOsbuildClient("localhost:8086", nil, nil, nil)

	require.Equal(t, "", client.ca)
	require.Equal(t, "", client.cert)
	require.Equal(t, "", client.key)
}

func TestNewOsbuildClientWithCerts(t *testing.T) {
	myCert := "test-cert"
	myKey := "test-key"
	myCA := "test-ca"
	client := NewOsbuildClient("localhost:8086", &myCert, &myKey, &myCA)

	require.NotEqual(t, "", client.ca)
	require.NotEqual(t, "", client.cert)
	require.NotEqual(t, "", client.key)
}

func TestOsbuildClientGetNoError(t *testing.T) {
	// we're not using HTTPS URL on purpose
	client := NewOsbuildClient("localhost:8086/", nil, nil, nil)
	new_client, err := client.Get()

	require.NoError(t, err)
	require.NotNil(t, new_client)
	require.Equal(t, "localhost:8086/", new_client.Server)
}

func TestOsbuildClientConfigureClientReturnsNilWhenNotUsingHttps(t *testing.T) {
	// we're not using HTTPS URL on purpose
	osbuild_client := NewOsbuildClient("localhost:8086/", nil, nil, nil)

	openapi_client := Client{
		Server: osbuild_client.osbuildURL,
	}

	result := osbuild_client.ConfigureClient(&openapi_client)
	require.Nil(t, result)
}
