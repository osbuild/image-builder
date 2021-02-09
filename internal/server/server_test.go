package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/tutils"
)

func startServer(t *testing.T, url string, orgIds string) *Server {
	logger, err := logger.NewLogger("DEBUG", nil, nil, nil, nil)
	require.NoError(t, err)

	client, err := cloudapi.NewOsbuildClient(url, nil, nil, nil)
	require.NoError(t, err)

	srv := NewServer(logger, client, "", "", "", "", strings.Split(orgIds, ";"), "../../distributions")
	// execute in parallel b/c .Run() will block execution
	go srv.Run("localhost:8086")

	return srv
}

// note: all of the sub-tests below don't actually talk to
// osbuild-composer API that's why they are groupped together
func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv := startServer(t, "http://example.com", "000000")
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("VerifyIdentityHeaderMissing", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", nil)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Auth header is not present")
	})

	t.Run("GetVersion", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)

		var result Version
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "1.0", result.Version)
	})

	t.Run("GetOpenapiJson", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/openapi.json", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		// note: not asserting body b/c response is too big
	})

	t.Run("GetDistributions", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/distributions", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)

		var result Distributions
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)

		for _, distro := range result {
			require.Contains(t, []string{"fedora-32", "rhel-8"}, distro.Name)
		}
	})

	t.Run("GetArchitectures", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/architectures/fedora-32", &tutils.AuthString0)
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

	t.Run("GetPacakges", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)

		var result PackagesResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Contains(t, result.Data[0].Name, "ssh")
		require.Greater(t, result.Meta.Count, 0)
		require.Contains(t, result.Links.First, "search=ssh")
		p1 := result.Data[0]
		p2 := result.Data[1]

		response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=1", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		err = json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, result.Meta.Count, 1)
		require.Equal(t, result.Data[0], p1)

		response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=1&offset=1", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		err = json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, result.Meta.Count, 1)
		require.Equal(t, result.Data[0], p2)

		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=-13", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=-13&offset=-2193", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
	})

	t.Run("BogusAuthString", func(t *testing.T) {
		auth := "notbase64"
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Auth header has incorrect format")
	})

	t.Run("BogusBase64AuthString", func(t *testing.T) {
		auth := "dGhpcyBpcyBkZWZpbml0ZWx5IG5vdCBqc29uCg=="
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Auth header has incorrect format")
	})

	t.Run("NoOrgId", func(t *testing.T) {
		// no org_id key is present
		auth := "eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnt9fX0="
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization not allowed")
	})

	t.Run("OrgIdNotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization not allowed")
	})

	t.Run("StatusCheck", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/status", nil)
		require.Equal(t, 200, response.StatusCode)
	})
}

func TestEmptyOrgIds(t *testing.T) {
	srv := startServer(t, "http://example.com", "")
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("OrgIdNotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization not allowed")
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

	srv := startServer(t, api_srv.URL, "*")
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", &tutils.AuthString0)
	require.Equal(t, 200, response.StatusCode)

	var result cloudapi.ComposeStatus
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, cloudapi.ComposeStatus{
		Status: "running",
	}, result)

	// With a wildcard orgIds either auth should work
	response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)
	err = json.Unmarshal([]byte(body), &result)
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

	srv := startServer(t, api_srv.URL, "*")
	defer func() {
		err := srv.echo.Server.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", &tutils.AuthString0)
	require.Equal(t, 404, response.StatusCode)
	require.Contains(t, body, "404 during tests")
}

// note: these scenarios don't needs to talk to a simulated osbuild-composer API
func TestComposeImage(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv := startServer(t, "http://example.com", "*")
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

	srv := startServer(t, api_srv.URL, "*")
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

	srv := startServer(t, api_srv.URL, "*")
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

	srv := startServer(t, api_srv.URL, "*")
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

func TestOrgIdAllowed(t *testing.T) {
	var header IdentityHeader

	header.Identity.Internal.OrgId = ""
	require.False(t, orgIdAllowed(header, []string{}))
	require.True(t, orgIdAllowed(header, []string{"*"}))
	header.Identity.Internal.OrgId = "12345"
	require.False(t, orgIdAllowed(header, []string{}))
	require.True(t, orgIdAllowed(header, []string{"*"}))
	require.True(t, orgIdAllowed(header, []string{"12345"}))
	require.True(t, orgIdAllowed(header, []string{"12345", "12322"}))
	require.False(t, orgIdAllowed(header, []string{"123456"}))
	require.True(t, orgIdAllowed(header, []string{"123456", "*"}))
}
