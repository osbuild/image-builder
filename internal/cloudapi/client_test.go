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

func TestComposeStatusWithHTTPServer(t *testing.T) {
	uuid := "1cf31af9-d2be-4b11-bb2d-f2f2d22b0736"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, r.URL.Path, fmt.Sprintf("/api/composer/v1/compose/%s", uuid))
		require.Equal(t, r.Method, "GET")
		w.Header().Set("Content-Type", "application/json")
		s := ComposeStatus{
			ImageStatus: ImageStatus{
				Status: ImageStatusValue("building"),
			},
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
	require.Equal(t, s.ImageStatus.Status, ImageStatusValue("building"))
}

func TestComposeWithHTTPServer(t *testing.T) {
	uuid := "1cf31af9-d2be-4b11-bb2d-f2f2d22b0736"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, r.URL.Path, "/api/composer/v1/compose")
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
