package cloudapi

import (
	"errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestNewClientWithoutOptions(t *testing.T) {
	// note: URL is without trailling slash on purpose
	result, err := NewClient("example.com")

	require.NoError(t, err)
	require.NotNil(t, result.Client)
	require.Equal(t, http.DefaultClient, result.Client)
	// verify server URL always has a trailing slash
	require.Equal(t, "example.com/", result.Server)
}

func TestNewClientErrorHandlingFromClientOptions(t *testing.T) {
	configureClientWithErrors := func(client *Client) error {
		return errors.New("Expected during testing")
	}
	result, err := NewClient("example.com/", configureClientWithErrors)

	require.Nil(t, result)
	require.EqualError(t, err, "Expected during testing")
}

func TestNewComposeStatusRequest(t *testing.T) {
	request, err := NewComposeStatusRequest("example.com", "dummy-compose-id")
	require.NotNil(t, request)
	require.NoError(t, err)
	require.Equal(t, "GET", request.Method)
	require.Equal(t, "/compose/dummy-compose-id", request.URL.Path)
}
