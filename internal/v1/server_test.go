package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/tutils"

	"github.com/labstack/echo/v4"
)

func startServer(t *testing.T, url string, orgIds string, accountIds string) *echo.Echo {
	logger, err := logger.NewLogger("DEBUG", nil, nil, nil, nil)
	require.NoError(t, err)

	client, err := cloudapi.NewOsbuildClient(url, nil, nil, nil)
	require.NoError(t, err)

	echoServer := echo.New()
	Attach(echoServer, logger, client, tutils.InitDB(), AWSConfig{}, GCPConfig{}, AzureConfig{}, strings.Split(orgIds, ";"), strings.Split(accountIds, ";"), "../../distributions")
	// execute in parallel b/c .Run() will block execution
	go echoServer.Start("localhost:8086")

	// wait until server is ready
	tries := 0
	for tries < 5 {
		resp, err := tutils.GetResponseError("http://localhost:8086/status")
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		} else if tries == 4 {
			require.NoError(t, err)
		}
		time.Sleep(time.Second)
		tries += 1
	}

	return echoServer
}

// note: all of the sub-tests below don't actually talk to
// osbuild-composer API that's why they are groupped together
func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv :=startServer(t, "http://example.com", "000000", "")
	defer func() {
		err := srv.Shutdown(context.Background())
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
			require.Contains(t, []string{"rhel-84", "rhel-8", "centos-8"}, distro.Name)
		}
	})

	t.Run("GetArchitectures", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/architectures/centos-8", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)

		var result Architectures
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, Architectures{
			ArchitectureItem{
				Arch:       "x86_64",
				ImageTypes: []string{"ami", "vhd"},
			}}, result)
	})

	t.Run("GetPacakges", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-84&architecture=x86_64&search=ssh", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)

		var result PackagesResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Contains(t, result.Data[0].Name, "ssh")
		require.Greater(t, result.Meta.Count, 0)
		require.Contains(t, result.Links.First, "search=ssh")
		p1 := result.Data[0]
		p2 := result.Data[1]

		response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-84&architecture=x86_64&search=ssh&limit=1", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		err = json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Greater(t, result.Meta.Count, 1)
		require.Equal(t, result.Data[0], p1)

		response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-84&architecture=x86_64&search=ssh&limit=1&offset=1", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		err = json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Greater(t, result.Meta.Count, 1)
		require.Equal(t, result.Data[0], p2)

		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-84&architecture=x86_64&search=ssh&limit=-13", &tutils.AuthString0)
		require.Equal(t, 400, response.StatusCode)
		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-84&architecture=x86_64&search=ssh&limit=13&offset=-2193", &tutils.AuthString0)
		require.Equal(t, 400, response.StatusCode)
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
		require.Contains(t, body, "Organization or account not allowed")
	})

	t.Run("OrgIdNotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization or account not allowed")
	})

	t.Run("StatusCheck", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/status", nil)
		require.Equal(t, 200, response.StatusCode)
	})
}

func TestEmptyAllowedIds(t *testing.T) {
	srv := startServer(t, "http://example.com", "", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("NotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization or account not allowed")
	})
}

func TestOrgIds(t *testing.T) {
	srv := startServer(t, "http://example.com", "000000", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("Authorized", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
	})

	t.Run("NotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization or account not allowed")
	})
}

func TestAccountNumbers(t *testing.T) {
	srv := startServer(t, "http://example.com", "", "500000")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("Authorized", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
	})

	t.Run("NotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization or account not allowed")
	})
}

func TestOrgIdWildcard(t *testing.T) {
	srv := startServer(t, "http://example.com", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("Authorized", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
	})
}

func TestAccountNumberWildcard(t *testing.T) {
	srv := startServer(t, "http://example.com", "", "*")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("Authorized", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
	})
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeStatus(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s := ComposeStatus{
			ImageStatus: ImageStatus{
				Status: "building",
			},
		}
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", &tutils.AuthString0)
	require.Equal(t, 200, response.StatusCode)

	var result cloudapi.ComposeStatus
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, cloudapi.ComposeStatus{
		ImageStatus: cloudapi.ImageStatus{
			Status: "building",
		},
	}, result)

	// With a wildcard orgIds either auth should work
	response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes/xyz-123-test", &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, cloudapi.ComposeStatus{
		ImageStatus: cloudapi.ImageStatus{
			Status: "building",
		},
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

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
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
	srv := startServer(t, "http://example.com", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Run("ErrorsForZeroImageRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests:  []ImageRequest{},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Exactly one image request should be included")
	})

	t.Run("ErrorsForTwoImageRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture: "x86_64",
					ImageType:    "qcow2",
					UploadRequest: UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
				ImageRequest{
					Architecture: "x86_64",
					ImageType:    "ami",
					UploadRequest: UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
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
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture:  "x86_64",
					ImageType:     "qcow2",
					UploadRequest: UploadRequest{},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Regexp(t, "image_requests/0/upload_request/options|image_requests/0/upload_request/type", body)
		require.Regexp(t, "Value is not nullable|value is not one of the allowed values", body)
	})

	t.Run("ISEWhenRepositoriesNotFound", func(t *testing.T) {
		// Distro arch isn't supported which triggers error when searching
		// for repositories
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture: "unsupported-arch",
					ImageType:    "qcow2",
					UploadRequest: UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "Architecture not supported")
	})

	t.Run("ErrorsForUnknownUploadType", func(t *testing.T) {
		// UploadRequest Type isn't supported
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				ImageRequest{
					Architecture: "x86_64",
					ImageType:    "qcow2",
					UploadRequest: UploadRequest{
						Type: "unknown",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
			},
		}
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "image_requests/0/upload_request/type")
		require.Contains(t, body, "value is not one of the allowed values")
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

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			ImageRequest{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				UploadRequest: UploadRequest{
					Type: "aws",
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: []string{"test-account"},
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

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			ImageRequest{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				UploadRequest: UploadRequest{
					Type: "aws",
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: []string{"test-account"},
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
			Id: "3aa7375a-534a-4de3-8caf-011e04f402d3",
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			ImageRequest{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				UploadRequest: UploadRequest{
					Type: "aws",
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: []string{"test-account"},
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
	require.Equal(t, "3aa7375a-534a-4de3-8caf-011e04f402d3", result.Id)
}

// convenience function for string pointer fields
func strptr(s string) *string {
	return &s
}

func TestComposeCustomizations(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := cloudapi.ComposeResult{
			Id: "fe93fb55-ae04-4e21-a8b4-25ba95c3fa64",
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	payloads := []ComposeRequest{
		{
			Customizations: &Customizations{
				Packages: &[]string{
					"some",
					"packages",
				},
			},
			Distribution: "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    "qcow2",
					UploadRequest: UploadRequest{
						Type: "aws",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
			},
		},
		{
			Customizations: &Customizations{
				Packages: nil,
			},
			Distribution: "rhel-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    "rhel-edge-commit",
					Ostree: &OSTree{
						Ref: strptr("edge/ref"),
					},
					UploadRequest: UploadRequest{
						Type:    "aws.s3",
						Options: AWSS3UploadRequestOptions{},
					},
				},
			},
		},
		{
			Customizations: &Customizations{
				Packages: &[]string{"pkg"},
				Subscription: &Subscription{
					Organization: 000,
				},
			},
			Distribution: "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    "rhel-edge-commit",
					Ostree: &OSTree{
						Ref: strptr("test/edge/ref"),
						Url: strptr("https://ostree.srv/"),
					},
					UploadRequest: UploadRequest{
						Type:    "aws.s3",
						Options: AWSS3UploadRequestOptions{},
					},
				},
			},
		},
	}

	for idx, payload := range payloads {
		fmt.Printf("TT payload %d\n", idx)
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusCreated, response.StatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "fe93fb55-ae04-4e21-a8b4-25ba95c3fa64", result.Id)
	}
}

// TestBuildOSTreeOptions checks if the buildOSTreeOptions utility function
// properly transfers the ostree options to the CloudAPI structure.
func TestBuildOSTreeOptions(t *testing.T) {
	cases := map[ImageRequest]*cloudapi.OSTree{
		{Ostree: nil}: nil,
		{Ostree: &OSTree{Ref: strptr("someref")}}:                                     {Ref: strptr("someref")},
		{Ostree: &OSTree{Ref: strptr("someref"), Url: strptr("https://example.org")}}: {Ref: strptr("someref"), Url: strptr("https://example.org")},
		{Ostree: &OSTree{Url: strptr("https://example.org")}}:                         {Url: strptr("https://example.org")},
	}

	for in, expOut := range cases {
		require.Equal(t, expOut, buildOSTreeOptions(in.Ostree), "input: %#v", in)
	}
}

func TestIdentityAllowed(t *testing.T) {
	var header IdentityHeader

	header.Identity.Internal.OrgId = ""
	require.False(t, identityAllowed(header, []string{}, []string{}))
	require.True(t, identityAllowed(header, []string{"*"}, []string{}))
	header.Identity.Internal.OrgId = "12345"
	require.False(t, identityAllowed(header, []string{}, []string{}))
	require.True(t, identityAllowed(header, []string{"*"}, []string{}))
	require.True(t, identityAllowed(header, []string{"12345"}, []string{}))
	require.True(t, identityAllowed(header, []string{"12345"}, []string{"54321"}))
	require.True(t, identityAllowed(header, []string{"12345", "12322"}, []string{}))
	require.False(t, identityAllowed(header, []string{"123456"}, []string{}))
	require.False(t, identityAllowed(header, []string{"123456"}, []string{"54321"}))
	require.True(t, identityAllowed(header, []string{"123456", "*"}, []string{}))

	header.Identity.AccountNumber = "54321"
	require.False(t, identityAllowed(header, []string{}, []string{}))
	require.True(t, identityAllowed(header, []string{"*"}, []string{}))
	require.True(t, identityAllowed(header, []string{""}, []string{"*"}))
	require.True(t, identityAllowed(header, []string{"*"}, []string{"*"}))
	require.True(t, identityAllowed(header, []string{""}, []string{"54321"}))
	require.False(t, identityAllowed(header, []string{""}, []string{"54322"}))
	require.True(t, identityAllowed(header, []string{""}, []string{"54321", "54322"}))

	header.Identity.Internal.OrgId = ""
	require.False(t, identityAllowed(header, []string{"12345"}, []string{}))
	require.True(t, identityAllowed(header, []string{"12345"}, []string{"54321"}))
}

func TestReadinessProbeNotReady(t *testing.T) {
	srv := startServer(t, "http://example.com", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, _ := tutils.GetResponseBody(t, "http://localhost:8086/ready", &tutils.AuthString0)
	require.NotEqual(t, 200, response.StatusCode)
	require.NotEqual(t, 404, response.StatusCode)
}

func TestReadinessProbeReady(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"version\":\"fake\"}")
	}))
	defer api_srv.Close()

	srv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/ready", &tutils.AuthString0)
	require.Equal(t, 200, response.StatusCode)
	require.Contains(t, body, "{\"readiness\":\"ready\"}")
}

func TestMetrics(t *testing.T) {
	// simulate osbuild-composer API
	srv := startServer(t, "", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/metrics", nil)
	require.Equal(t, 200, response.StatusCode)
	require.Contains(t, body, "image_builder_compose_requests_total")
	require.Contains(t, body, "image_builder_compose_errors")
}
