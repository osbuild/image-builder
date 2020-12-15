package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/logger"
)

func getResponseBody(t *testing.T, path string, auth bool) (*http.Response, string) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", "http://localhost:8086" + path, nil)
	require.NoError(t, err)
	if auth {
		request.Header.Add("X-Rh-Identity", "tester")
	}

	response, err := client.Do(request)
	require.NoError(t, err)

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	return response, string(body)
}

func TestMain(m *testing.M) {
	logger, err := logger.NewLogger("DEBUG", nil, nil, nil, nil)
	if err != nil {
		log.Fatalf("ERROR: logger setup failed: %v\n", err)
	}

	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	client, err := cloudapi.NewOsbuildClient("http://example.com", nil, nil, nil)
	if err != nil {
		log.Fatalf("ERROR: client setup failed: %v\n", err)
	}

	srv := NewServer(logger, client, "", "", "", "")
	// execute in parallel b/c .Run() will block execution
	go srv.Run("localhost:8086")

	os.Exit(m.Run())
}

func TestVerifyIdentityHeaderMissing(t *testing.T) {
	response, body := getResponseBody(t, "/api/image-builder/version", false)
	require.Equal(t, 404, response.StatusCode)
	require.Contains(t, body, "x-rh-identity header is not present")
}

func TestGetVersion(t *testing.T) {
	response, body := getResponseBody(t, "/api/image-builder/version", true)

	fmt.Printf("**** body=%v\n", body)

	require.Equal(t, 200, response.StatusCode)
}
