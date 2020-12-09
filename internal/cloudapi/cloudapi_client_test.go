package cloudapi

import (
	"errors"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"strings"
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

func TestNewComposeRequestWithBody(t *testing.T) {
	request, err := NewComposeRequestWithBody(
		"example.com",
		"text/plain",
		strings.NewReader("body contents"),
	)
	require.NotNil(t, request)
	require.NoError(t, err)
	require.Equal(t, "POST", request.Method)
	require.Equal(t, "/compose", request.URL.Path)
	require.Equal(t, request.Header.Get("Content-Type"), "text/plain")

	body, err := ioutil.ReadAll(request.Body)
	require.NoError(t, err)
	require.Equal(t, "body contents", string(body))
}

func TestNewComposeRequest(t *testing.T) {
	request, err := NewComposeRequest(
		"example.com",
		ComposeJSONRequestBody{
			Distribution:   "fedora-33",
			Customizations: nil,
			ImageRequests:  []ImageRequest{},
		},
	)
	require.NotNil(t, request)
	require.NoError(t, err)
	require.Equal(t, "POST", request.Method)
	require.Equal(t, "/compose", request.URL.Path)
	require.Equal(t, request.Header.Get("Content-Type"), "application/json")

	body, err := ioutil.ReadAll(request.Body)
	require.NoError(t, err)
	require.Equal(t, `{"distribution":"fedora-33","image_requests":[]}`, string(body))
}
