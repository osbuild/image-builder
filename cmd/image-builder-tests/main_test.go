// +build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/server"
	"github.com/osbuild/image-builder/internal/tutils"

	"github.com/stretchr/testify/require"
)

func RunTestWithClient(t *testing.T, ib string) {
	// wait until server is reachable
	tries := 0
	for tries < 5 {
		resp, err := tutils.GetResponseError(ib + "/version")
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		} else if tries == 4 {
			require.NoError(t, err)
		}
		time.Sleep(time.Second)
		tries += 1
	}

	response, body := tutils.GetResponseBody(t, ib+"/distributions", &tutils.AuthString0)
	require.Equalf(t, http.StatusOK, response.StatusCode, "Error: got non-200 status. Full response: %s", body)

	require.NotNil(t, body)
	require.NotEmpty(t, body)

	var distributions server.Distributions
	err := json.Unmarshal([]byte(body), &distributions)
	require.NoError(t, err)

	distro := distributions[0].Name
	response, body = tutils.GetResponseBody(t, ib+"/architectures/"+distro, &tutils.AuthString0)
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

	composeRequestCases := []struct {
		imageType     string
		uploadRequest server.UploadRequest
	}{
		{
			imageType: "ami",
			uploadRequest: server.UploadRequest{
				Type: "aws",
				Options: server.AWSUploadRequestOptions{
					ShareWithAccounts: []string{"012345678912"},
				},
			},
		},
		{
			imageType: "vhd",
			uploadRequest: server.UploadRequest{
				Type: "gcp",
				Options: server.GCPUploadRequestOptions{
					ShareWithAccounts: []string{"user:somebody@example.com"},
				},
			},
		},
	}

	for _, c := range composeRequestCases {
		t.Run("composeRequest/"+string(c.uploadRequest.Type), func(t *testing.T) {

			composeRequest := server.ComposeRequest{
				Distribution: distro,
				ImageRequests: []server.ImageRequest{
					server.ImageRequest{
						Architecture: arch,
						ImageType:    c.imageType,
						UploadRequests: []server.UploadRequest{
							c.uploadRequest,
						},
					},
				},
				Customizations: &server.Customizations{
					Packages: &[]string{"postgresql"},
				},
			}

			response, body = tutils.PostResponseBody(t, ib+"/compose", composeRequest)
			require.Equal(t, http.StatusCreated, response.StatusCode, "Error: got non-201 status. Full response: %s", body)
			require.NotNil(t, body)
			require.NotEmpty(t, body)

			var composeResp server.ComposeResponse
			err = json.Unmarshal([]byte(body), &composeResp)
			require.NoError(t, err)
			id := composeResp.Id

			response, body = tutils.GetResponseBody(t, ib+"/composes/"+id, &tutils.AuthString0)
			require.Equal(t, http.StatusOK, response.StatusCode, "Error: got non-200 status. Full response: %s", body)
			require.NotNil(t, body)
			require.NotEmpty(t, body)

			var composeStatus cloudapi.ComposeStatus
			err = json.Unmarshal([]byte(body), &composeStatus)
			require.NoError(t, err)
			require.Contains(t, []string{"pending", "running"}, composeStatus.ImageStatus.Status)
		})
	}

	// Check if we get 404 without the identity header
	response, body = tutils.GetResponseBody(t, ib+"/version", nil)
	require.Equalf(t, http.StatusNotFound, response.StatusCode, "Error: got non-404 status. Full response: %s", body)
}

// Same test as above but against existing contain on localhost:8087
func TestImageBuilderContainer(t *testing.T) {
	RunTestWithClient(t, "http://127.0.0.1:8087/api/image-builder/v1")
	RunTestWithClient(t, "http://127.0.0.1:8087/api/image-builder/v1.0")
}
