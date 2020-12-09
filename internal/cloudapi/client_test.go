package cloudapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOsbuildClientWithoutCerts(t *testing.T) {
	client, err := NewOsbuildClient("localhost:8086", nil, nil, nil)

	require.NoError(t, err)
	require.Equal(t, "", client.ca)
	require.Equal(t, "", client.cert)
	require.Equal(t, "", client.key)
}

func TestNewOsbuildClientWithCerts(t *testing.T) {
	myCert := "test-cert"
	myKey := "test-key"
	myCA := "test-ca"

	client, err := NewOsbuildClient("localhost:8086", &myCert, &myKey, &myCA)
	require.NoError(t, err)
	require.NotEqual(t, "", client.ca)
	require.NotEqual(t, "", client.cert)
	require.NotEqual(t, "", client.key)
}

func TestNewOsbuildClientWithCertsAndHttps(t *testing.T) {
	myCert := "test-cert"
	myKey := "test-key"
	myCA := "test-ca"

	_, err := NewOsbuildClient("https://localhost:8086", &myCert, &myKey, &myCA)
	require.Error(t, err)
}

func TestConfigureClientWithValidCertsAndHttps(t *testing.T) {
	myCert := "/etc/osbuild-composer/client-crt.pem"
	myKey := "/etc/osbuild-composer/client-key.pem"
	myCA := "/etc/osbuild-composer-test/ca/ca.cert.pem"

	client, err := NewOsbuildClient("https://localhost:8086/", &myCert, &myKey, &myCA)
	require.NoError(t, err)
	require.NotNil(t, client.client)
	require.IsType(t, &http.Client{}, client.client)
}

func TestComposeStatusWithHTTPServer(t *testing.T) {
	uuid := "1cf31af9-d2be-4b11-bb2d-f2f2d22b0736"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, r.URL.Path, fmt.Sprintf("/compose/%s", uuid))
		require.Equal(t, r.Method, "GET")
		w.Header().Set("Content-Type", "application/json")
		s := ComposeStatus{
			Status: "running",
		}
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer ts.Close()

	client, err := NewOsbuildClient(ts.URL, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client.client)

	resp, err := client.ComposeStatus(uuid)
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusOK)

	var s ComposeStatus
	err = json.NewDecoder(resp.Body).Decode(&s)
	require.NoError(t, err)
	require.Equal(t, s.Status, "running")
}

func TestComposeWithHTTPServer(t *testing.T) {
	uuid := "1cf31af9-d2be-4b11-bb2d-f2f2d22b0736"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, r.URL.Path, "/compose")
		require.Equal(t, r.Method, "POST")
		require.Equal(t, r.Header.Get("Content-Type"), "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		s := ComposeResult{
			Id: uuid,
		}
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer ts.Close()

	client, err := NewOsbuildClient(ts.URL, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client.client)

	resp, err := client.Compose(ComposeRequest{})
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusCreated)

	var s ComposeResult
	err = json.NewDecoder(resp.Body).Decode(&s)
	require.NoError(t, err)
	require.Equal(t, s.Id, uuid)
}
