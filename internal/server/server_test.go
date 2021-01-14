package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/tutils"
)

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
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", false)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "x-rh-identity header is not present")
	})

	t.Run("GetVersion", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", true)
		require.Equal(t, 200, response.StatusCode)

		var result Version
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "1.0", result.Version)
	})

	t.Run("GetOpenapiJson", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/openapi.json", true)
		require.Equal(t, 200, response.StatusCode)
		// note: not asserting body b/c response is too big
	})

	t.Run("GetDistributions", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/distributions", true)
		require.Equal(t, 200, response.StatusCode)

		var result Distributions
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)

		for _, distro := range result {
			require.Contains(t, []string{"fedora-32", "rhel-8"}, distro.Name)
		}
	})

	t.Run("GetArchitectures", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/architectures/fedora-32", true)
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

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", true)
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

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", true)
	require.Equal(t, 404, response.StatusCode)
	require.Contains(t, body, "404 during tests")
}

// note: these scenarios don't needs to talk to a simulated osbuild-composer API
func TestComposeImage(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv := startServer(t, "http://example.com")
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("ErrorsForZeroImageRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "fedora-32",
			ImageRequests:  []ImageRequest{},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Exactly one image request should be included")
	})

	t.Run("ErrorsForTwoImageRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "fedora-32",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture:   "x86_64",
					ImageType:      "qcow2",
					UploadRequests: nil,
				},
				ImageRequest{
					Architecture:   "x86_64",
					ImageType:      "ami",
					UploadRequests: nil,
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Exactly one image request should be included")
	})

	t.Run("ErrorsForZeroUploadRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "fedora-32",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture:   "x86_64",
					ImageType:      "qcow2",
					UploadRequests: []UploadRequest{},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Exactly one upload request should be included")
	})

	t.Run("ErrorsForTwoUploadRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "fedora-32",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture: "x86_64",
					ImageType:    "qcow2",
					UploadRequests: []UploadRequest{
						UploadRequest{
							Type: "aws",
							Options: AWSUploadRequestOptions{
								ShareWithAccounts: []string{"test-account"},
							},
						},
						UploadRequest{
							Type: "aws",
							Options: AWSUploadRequestOptions{
								ShareWithAccounts: []string{"test-account"},
							},
						},
					},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Exactly one upload request should be included")
	})

	t.Run("ISEWhenRepositoriesNotFound", func(t *testing.T) {
		// Distro arch isn't supported which triggers error when searching
		// for repositories
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "fedora-32",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture: "unsupported-arch",
					ImageType:    "qcow2",
					UploadRequests: []UploadRequest{
						UploadRequest{
							Type: "aws",
							Options: AWSUploadRequestOptions{
								ShareWithAccounts: []string{"test-account"},
							},
						},
					},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 500, response.StatusCode)
		require.Contains(t, body, "Internal Server Error")
	})

	t.Run("ErrorsForUnknownUploadType", func(t *testing.T) {
		// UploadRequest Type isn't supported
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "fedora-32",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture: "x86_64",
					ImageType:    "qcow2",
					UploadRequests: []UploadRequest{
						UploadRequest{
							Type: "unknown",
							Options: AWSUploadRequestOptions{
								ShareWithAccounts: []string{"test-account"},
							},
						},
					},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Unknown UploadRequest type")
	})
}

func TestComposeImageErrorsWhenStatusCodeIsNotStatusCreated(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		s := "deliberately returning !201 during tests"
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL)
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "fedora-32",
		ImageRequests: []ImageRequest{
			ImageRequest{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				UploadRequests: []UploadRequest{
					UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
			},
		},
	}
	response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusTeapot, response.StatusCode)
	require.Contains(t, body, "Failed posting compose request to osbuild-composer")
}

func TestComposeImageErrorsWhenCannotParseResponse(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		s := "not a cloudapi.ComposeResult data structure"
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL)
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "fedora-32",
		ImageRequests: []ImageRequest{
			ImageRequest{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				UploadRequests: []UploadRequest{
					UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
			},
		},
	}
	response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, 500, response.StatusCode)
	require.Contains(t, body, "Internal Server Error")
}

func TestComposeImageReturnsIdWhenNoErrors(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := cloudapi.ComposeResult{
			Id: "a-new-test-compose-id",
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL)
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "fedora-32",
		ImageRequests: []ImageRequest{
			ImageRequest{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				UploadRequests: []UploadRequest{
					UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
			},
		},
	}
	response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusCreated, response.StatusCode)

	var result ComposeResponse
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, "a-new-test-compose-id", result.Id)
}
