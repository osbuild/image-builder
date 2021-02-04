// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/server"
	"github.com/osbuild/image-builder/internal/tutils"

	"github.com/stretchr/testify/require"
)

func RunTestWithClient(t *testing.T, ib string)  {
	// wait until server is reachable
	tries := 0
	for tries  < 5 {
		resp, err := tutils.GetResponseError(ib + "/version")
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		} else if tries == 4 {
			require.NoError(t, err)
		}
		time.Sleep(time.Second)
		tries += 1
	}

	response, body := tutils.GetResponseBody(t, ib + "/distributions", &tutils.AuthString0)
	require.Equalf(t, http.StatusOK, response.StatusCode, "Error: got non-200 status. Full response: %s", body)

	require.NotNil(t, body)
	require.NotEmpty(t, body)

	var distributions server.Distributions
	err := json.Unmarshal([]byte(body), &distributions)
	require.NoError(t, err)

	distro := distributions[0].Name
	response, body = tutils.GetResponseBody(t, ib + "/architectures/" + distro, &tutils.AuthString0)
	require.Equalf(t, http.StatusOK, response.StatusCode, "Error: got non-200 status. Full response: %s", body)
	require.NotNil(t, body)
	require.NotEmpty(t, body)

	var architectures server.Architectures
	err = json.Unmarshal([]byte(body), &architectures)
	require.NoError(t, err)
	arch := architectures[0].Arch

	// Get a list of packages
	response, body = tutils.GetResponseBody(t, fmt.Sprintf("%v/packages?search=ssh&distribution=%v&architecture=%v&limit=10&offset=0", ib, distro, arch), &tutils.AuthString0)
	require.Equalf(t, http.StatusOK, response.StatusCode, "Error: got non-200 status. Full response: %s", body)
	var packages server.PackagesResponse
	err = json.Unmarshal([]byte(body), &packages)
	// Make sure we get some packages
	require.Greater(t, packages.Meta.Count, 0)
	// The links supplied should point to the same search
	require.Contains(t, packages.Links.First, "search=ssh")



	// Build a composerequest
	composeRequest := server.ComposeRequest{
		Distribution: distro,
		ImageRequests: []server.ImageRequest{
			server.ImageRequest{
				Architecture: arch,
				ImageType: architectures[0].ImageTypes[0],
				UploadRequests: []server.UploadRequest{
					{
						Type: "aws",
						Options: server.AWSUploadRequestOptions{
							ShareWithAccounts: []string{"012345678912"},
						},
					},
				},
			},
		},
	}

	response, body = tutils.PostResponseBody(t, ib + "/compose", composeRequest)
	require.Equal(t, http.StatusCreated, response.StatusCode, "Error: got non-201 status. Full response: %s", body)

	require.NotNil(t, body)
	require.NotEmpty(t, body)

	var composeResp server.ComposeResponse
	err = json.Unmarshal([]byte(body), &composeResp)
	require.NoError(t, err)

	id := composeResp.Id

	response, body = tutils.GetResponseBody(t, ib + "/composes/" + id, &tutils.AuthString0)
	require.Equal(t, http.StatusOK, response.StatusCode, "Error: got non-200 status. Full response: %s", body)

	require.NotNil(t, body)
	require.NotEmpty(t, body)

	var composeStatus cloudapi.ComposeStatus
	err = json.Unmarshal([]byte(body), &composeStatus)
	require.NoError(t, err)

	require.Contains(t, []string{"pending", "running"}, composeStatus.Status)

	// Check if we get 404 without the identity header
	response, body = tutils.GetResponseBody(t, ib + "/version", nil)
	require.Equalf(t, http.StatusNotFound, response.StatusCode, "Error: got non-404 status. Full response: %s", body)
}

func TestImageBuilder(t *testing.T) {
	// allow to run against existing instance
	// run image builder

	/* OsbuildURL      string  `env:"OSBUILD_URL"`
	   OsbuildCert     string  `env:"OSBUILD_CERT_PATH"`
	   OsbuildKey      string  `env:"OSBUILD_KEY_PATH"`
	   OsbuildCA       string  `env:"OSBUILD_CA_PATH"` */
	cmd := exec.Command("/usr/libexec/image-builder/image-builder")
	cmd.Env = append(os.Environ(),
		"OSBUILD_URL=https://localhost:443",
		"OSBUILD_CA_PATH=/etc/osbuild-composer/ca-crt.pem",
		"OSBUILD_CERT_PATH=/etc/osbuild-composer/client-crt.pem",
		"OSBUILD_KEY_PATH=/etc/osbuild-composer/client-key.pem",
		"ALLOWED_ORG_IDS=*",
	)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	err := cmd.Start()
	require.NoError(t, err)
	defer func() {
		fmt.Println("Out: ", buf.String())
		cmd.Process.Kill()
	}()

	RunTestWithClient(t, "http://127.0.0.1:8086/api/image-builder/v1")
	RunTestWithClient(t, "http://127.0.0.1:8086/api/image-builder/v1.0")
}

// Same test as above but against existing contain on localhost:8087
func TestImageBuilderContainer(t *testing.T) {
	RunTestWithClient(t, "http://127.0.0.1:8087/api/image-builder/v1")
	RunTestWithClient(t, "http://127.0.0.1:8087/api/image-builder/v1.0")
}
