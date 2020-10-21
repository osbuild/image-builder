// +build integration

package main

import (
	"context"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/server"
	"github.com/stretchr/testify/require"
)

func AddDummyIdentityHeader(ctx context.Context, request *http.Request) error {
	request.Header.Add("x-rh-identity", "eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQ==")
	return nil
}

func ConfigureClient(client *server.Client) error {
	client.RequestEditor = AddDummyIdentityHeader
	return nil
}

func TestImageBuilder(t *testing.T) {
	// run image builder
	cmd := exec.Command("/usr/libexec/image-builder/image-builder")
	err := cmd.Start()
	require.NoError(t, err)
	defer cmd.Process.Kill()

	client, err := server.NewClientWithResponses("http://127.0.0.1:8086/api/image-builder/v1", ConfigureClient)
	require.NoError(t, err)

	// wait until server is reachable
	ctx := context.Background()
	tries := 0
	for tries  < 5 {
		resp, err := client.GetVersionWithResponse(ctx)
		if err == nil && resp.StatusCode() == http.StatusOK {
			break
		} else if tries == 4 {
			require.NoError(t, err)
		}
		time.Sleep(time.Second)
		tries += 1
	}

	distroResp, err := client.GetDistributionsWithResponse(ctx)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, distroResp.StatusCode(), "Error: got non-200 status. Full response: %s", distroResp.Body)
	require.NotNil(t, distroResp.JSON200)
	require.NotEmpty(t, distroResp.JSON200)

	distro := (*distroResp.JSON200)[0].Name
	archResp, err := client.GetArchitecturesWithResponse(ctx, distro)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, archResp.StatusCode(), "Error: got non-200 status. Full response: %s", archResp.Body)
	require.NotEmpty(t, archResp.JSON200)

	client, err = server.NewClientWithResponses("http://127.0.0.1:8086/api/image-builder/v1")
	versionResp, err := client.GetVersionWithResponse(ctx)
	require.NoError(t, err)
	require.Equalf(t, http.StatusNotFound, versionResp.StatusCode(), "Error: got non-401 status. Full response: %s", versionResp.Body)

	client, err = server.NewClientWithResponses("http://127.0.0.1:8086/api/image-builder/v1.0", ConfigureClient)
	versionResp, err = client.GetVersionWithResponse(ctx)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, versionResp.StatusCode(), "Error: got non-200 status. Full response: %s", versionResp.Body)
}
