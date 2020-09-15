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


func TestImageBuilder(t *testing.T) {
	// run image builder
	cmd := exec.Command("/usr/libexec/image-builder/image-builder")
	err := cmd.Start()
	require.NoError(t, err)
	defer cmd.Process.Kill()

	client, err := server.NewClientWithResponses("http://127.0.0.1:8086/api/image-builder/v1")
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

	resp, err := client.GetDistributionsWithResponse(ctx)
	require.NoError(t, err)

	require.Equalf(t, http.StatusOK, resp.StatusCode(), "Error: got non-200 status. Full response: %v", string(resp.Body))
	require.NotNil(t, resp.JSON200)
	require.NotEmpty(t, resp.JSON200)

	distro := (*resp.JSON200)[0].Name
	client.GetArchitectures(ctx, distro)
	require.Equalf(t, http.StatusOK, resp.StatusCode(), "Error: got non-200 status. Full response: %v", string(resp.Body))
	require.NotNil(t, resp.JSON200)
	require.NotEmpty(t, resp.JSON200)
}
