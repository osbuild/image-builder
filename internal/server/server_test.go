package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/logger"
)

func getResponseBody(t *testing.T, path string, auth bool) (*http.Response, string) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", "http://localhost:8086"+path, nil)
	require.NoError(t, err)
	if auth {
		// also see AddDummyIdentityHeader() in main_test.go
		request.Header.Add("X-Rh-Identity", "tester")
	}

	response, err := client.Do(request)
	require.NoError(t, err)

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	return response, string(body)
}

func startServer(t *testing.T, url string) *Server {
	logger, err := logger.NewLogger("DEBUG", nil, nil, nil, nil)
	require.NoError(t, err)

	client, err := cloudapi.NewOsbuildClient(url, nil, nil, nil)
	require.NoError(t, err)

	srv := NewServer(logger, client, "", "", "", "")
	// execute in parallel b/c .Run() will block execution
	go srv.Run("localhost:8086")

	return srv
}

// note: all of the sub-tests below don't actually talk to
// osbuild-composer API that's why they are groupped together
func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv := startServer(t, "http://example.com")
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("VerifyIdentityHeaderMissing", func(t *testing.T) {
		response, body := getResponseBody(t, "/api/image-builder/v1/version", false)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "x-rh-identity header is not present")
	})

	t.Run("GetVersion", func(t *testing.T) {
		response, body := getResponseBody(t, "/api/image-builder/v1/version", true)
		require.Equal(t, 200, response.StatusCode)

		var result Version
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "1.0", result.Version)
	})

	t.Run("GetOpenapiJson", func(t *testing.T) {
		response, _ := getResponseBody(t, "/api/image-builder/v1/openapi.json", true)
		require.Equal(t, 200, response.StatusCode)
		// note: not asserting body b/c response is too big
	})

	t.Run("GetDistributions", func(t *testing.T) {
		response, body := getResponseBody(t, "/api/image-builder/v1/distributions", true)
		require.Equal(t, 200, response.StatusCode)

		var result Distributions
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)

		for _, distro := range result {
			require.Contains(t, []string{"fedora-32", "rhel-8"}, distro.Name)
		}
	})

	t.Run("GetArchitectures", func(t *testing.T) {
		response, body := getResponseBody(t, "/api/image-builder/v1/architectures/fedora-32", true)
		require.Equal(t, 200, response.StatusCode)

		var result Architectures
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, Architectures{
			ArchitectureItem{
				Arch:       "x86_64",
				ImageTypes: []string{"ami"},
			}}, result)
	})
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeStatus(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s := ComposeStatus{
			Status: "running",
		}
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL)
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := getResponseBody(t, "/api/image-builder/v1/composes/xyz-123-test", true)
	require.Equal(t, 200, response.StatusCode)

	var result cloudapi.ComposeStatus
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, cloudapi.ComposeStatus{
		Status: "running",
	}, result)
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeStatus404(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "404 during tests")
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL)
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := getResponseBody(t, "/api/image-builder/v1/composes/xyz-123-test", true)
	require.Equal(t, 404, response.StatusCode)
	require.Contains(t, body, "404 during tests")
}
