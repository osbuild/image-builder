package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/tutils"

	"github.com/labstack/echo/v4"
)

var UUIDTest string = "d1f631ff-b3a6-4eec-aa99-9e81d99bc93d"

// Create a temporary file containing quotas, returns the file name as a string
func initQuotaFile() (string, error) {
	// create quotas with only the default values
	quotas := map[string]common.Quota{
		"default": {Quota: common.DefaultQuota, SlidingWindow: common.DefaultSlidingWindow},
	}
	jsonQuotas, err := json.Marshal(quotas)
	if err != nil {
		return "", err
	}

	// get a temp file to store the quotas
	file, err := ioutil.TempFile("", "account_quotas.*.json")
	if err != nil {
		return "", err
	}

	// write to disk
	jsonFile, err := os.Create(file.Name())
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	_, err = jsonFile.Write(jsonQuotas)
	if err != nil {
		return "", err
	}
	err = jsonFile.Close()
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}

func startServerWithCustomDB(t *testing.T, url string, orgIds string, accountNumbers string, dbase db.DB) (*echo.Echo, *httptest.Server) {
	logger, err := logger.NewLogger("DEBUG", nil, nil, nil, nil)
	require.NoError(t, err)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"access_token"`
		}{
			AccessToken: "accesstoken",
		})
		require.NoError(t, err)
	}))

	client, err := composer.NewClient(url, tokenServer.URL, "offlinetoken")
	require.NoError(t, err)

	//store the quotas in a temporary file
	quotaFile, err := initQuotaFile()
	require.NoError(t, err)

	echoServer := echo.New()
	err = Attach(echoServer, logger, client, dbase, AWSConfig{}, GCPConfig{}, AzureConfig{}, strings.Split(orgIds, ";"),
		strings.Split(accountNumbers, ";"), "../../distributions", quotaFile)
	require.NoError(t, err)
	// execute in parallel b/c .Run() will block execution
	go func() {
		_ = echoServer.Start("localhost:8086")
	}()

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

	return echoServer, tokenServer
}

func startServer(t *testing.T, url string, orgIds string, accountNumbers string) (*echo.Echo, *httptest.Server) {
	return startServerWithCustomDB(t, url, orgIds, accountNumbers, tutils.InitDB())
}

// note: all of the sub-tests below don't actually talk to
// osbuild-composer API that's why they are groupped together
func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv, tokenSrv := startServer(t, "http://example.com", "000000", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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
			require.Contains(t, []string{"rhel-84", "rhel-85", "centos-8"}, distro.Name)
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
				ImageTypes: []string{"aws", "gcp", "azure", "ami", "vhd"},
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

	t.Run("EmptyAccountNumber", func(t *testing.T) {
		// AccoundNumber equals ""
		auth := tutils.GetCompleteBas64Header("", "000000")
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "The Account Number is missing in the Identity Header")
	})

	t.Run("NoOrgId", func(t *testing.T) {
		// no org_id key is present
		auth := tutils.GetBas64HeaderWithoutOrgId("000000")
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
	srv, tokenSrv := startServer(t, "http://example.com", "", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	t.Run("NotAuthorized", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString1)
		require.Equal(t, 404, response.StatusCode)
		require.Contains(t, body, "Organization or account not allowed")
	})
}

func TestOrgIds(t *testing.T) {
	srv, tokenSrv := startServer(t, "http://example.com", "000000", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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
	srv, tokenSrv := startServer(t, "http://example.com", "", "500000")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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
	srv, tokenSrv := startServer(t, "http://example.com", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	t.Run("Authorized", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
	})
}

func TestAccountNumberWildcard(t *testing.T) {
	srv, tokenSrv := startServer(t, "http://example.com", "", "*")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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

	// insert a compose in the mock database
	dbase := tutils.InitDB()
	err := dbase.InsertCompose(UUIDTest, "600000", "000001", json.RawMessage("{}"))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, api_srv.URL, "*", "", dbase)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		UUIDTest), &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)

	var result composer.ComposeStatus
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, composer.ComposeStatus{
		ImageStatus: composer.ImageStatus{
			Status: "building",
		},
	}, result)

	// With a wildcard orgIds either auth should work
	response, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		UUIDTest), &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, composer.ComposeStatus{
		ImageStatus: composer.ImageStatus{
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

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		UUIDTest), &tutils.AuthString0)
	require.Equal(t, 404, response.StatusCode)
	require.Contains(t, body, "Compose not found")
}

func TestGetComposeMetadata(t *testing.T) {
	// simulate osbuild-composer API
	testPackages := []composer.PackageMetadata{
		{
			Arch:      "ArchTest2",
			Epoch:     strptr("EpochTest2"),
			Name:      "NameTest2",
			Release:   "ReleaseTest2",
			Sigmd5:    "Sigmd5Test2",
			Signature: strptr("SignatureTest2"),
			Type:      "TypeTest2",
			Version:   "VersionTest2",
		},
		{
			Arch:      "ArchTest1",
			Epoch:     strptr("EpochTest1"),
			Name:      "NameTest1",
			Release:   "ReleaseTest1",
			Sigmd5:    "Sigmd5Test1",
			Signature: strptr("SignatureTest1"),
			Type:      "TypeTest1",
			Version:   "VersionTest1",
		},
	}
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		m := composer.ComposeMetadata{
			OstreeCommit: strptr("test string"),
			Packages:     &testPackages,
		}

		err := json.NewEncoder(w).Encode(m)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	// insert a compose in the mock database
	dbase := tutils.InitDB()
	err := dbase.InsertCompose(UUIDTest, "500000", "000000", json.RawMessage("{}"))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, api_srv.URL, "*", "", dbase)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var result composer.ComposeMetadata

	// Get API response and compare
	response, body := tutils.GetResponseBody(t,
		fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/metadata", UUIDTest), &tutils.AuthString0)
	require.Equal(t, 200, response.StatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, *result.Packages, testPackages)
}

func TestGetComposeMetadata404(t *testing.T) {
	// simulate osbuild-composer API
	api_srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "404 during tests")
	}))
	defer api_srv.Close()

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/metadata",
		UUIDTest), &tutils.AuthString0)
	require.Equal(t, 404, response.StatusCode)
	require.Contains(t, body, "Compose not found")
}

func TestGetComposes(t *testing.T) {
	var UUIDTest2 string = "d1f631ff-b3a6-4eec-aa99-9e81d99bc222"
	var UUIDTest3 string = "d1f631ff-b3a6-4eec-aa99-9e81d99bc333"

	dbase := tutils.InitDB()
	err := dbase.InsertCompose(UUIDTest, "500000", "000000", json.RawMessage("{}"))
	require.NoError(t, err)
	err = dbase.InsertCompose(UUIDTest2, "500000", "000000", json.RawMessage("{}"))
	require.NoError(t, err)
	err = dbase.InsertCompose(UUIDTest3, "500000", "000000", json.RawMessage("{}"))
	require.NoError(t, err)

	composeEntry, err := dbase.GetCompose(UUIDTest, "500000")
	require.NoError(t, err)

	db_srv, tokenSrv := startServerWithCustomDB(t, "", "*", "", dbase)
	defer func() {
		err := db_srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var result ComposesResponse

	resp, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes", &tutils.AuthString0)

	require.Equal(t, 200, resp.StatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)

	require.Equal(t, 3, result.Meta.Count)
	require.Equal(t, composeEntry.CreatedAt.String(), result.Data[0].CreatedAt)
	require.Equal(t, UUIDTest, result.Data[0].Id)
}

// note: these scenarios don't needs to talk to a simulated osbuild-composer API
func TestComposeImage(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv, tokenSrv := startServer(t, "http://example.com", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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
				{
					Architecture: "x86_64",
					ImageType:    ImageTypes_aws,
					UploadRequest: UploadRequest{
						Type: UploadTypes_aws,
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: []string{"test-account"},
						},
					},
				},
				{
					Architecture: "x86_64",
					ImageType:    ImageTypes_ami,
					UploadRequest: UploadRequest{
						Type: UploadTypes_aws,
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
				{
					Architecture:  "x86_64",
					ImageType:     ImageTypes_azure,
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
				{
					Architecture: "unsupported-arch",
					ImageType:    ImageTypes_aws,
					UploadRequest: UploadRequest{
						Type: UploadTypes_aws,
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
				{
					Architecture: "x86_64",
					ImageType:    ImageTypes_azure,
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

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypes_aws,
				UploadRequest: UploadRequest{
					Type: UploadTypes_aws,
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
		s := "not a composer.ComposeId data structure"
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypes_aws,
				UploadRequest: UploadRequest{
					Type: UploadTypes_aws,
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
		result := composer.ComposeId{
			Id: "3aa7375a-534a-4de3-8caf-011e04f402d3",
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypes_aws,
				UploadRequest: UploadRequest{
					Type: UploadTypes_aws,
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
		result := composer.ComposeId{
			Id: "fe93fb55-ae04-4e21-a8b4-25ba95c3fa64",
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer api_srv.Close()

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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
					ImageType:    ImageTypes_aws,
					UploadRequest: UploadRequest{
						Type: UploadTypes_aws,
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
			Distribution: "rhel-84",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypes_edge_commit,
					Ostree: &OSTree{
						Ref: strptr("edge/ref"),
					},
					UploadRequest: UploadRequest{
						Type:    UploadTypes_aws_s3,
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
					ImageType:    ImageTypes_rhel_edge_commit,
					Ostree: &OSTree{
						Ref: strptr("test/edge/ref"),
						Url: strptr("https://ostree.srv/"),
					},
					UploadRequest: UploadRequest{
						Type:    UploadTypes_aws_s3,
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
// properly transfers the ostree options to the Composer structure.
func TestBuildOSTreeOptions(t *testing.T) {
	cases := map[ImageRequest]*composer.OSTree{
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
	srv, tokenSrv := startServer(t, "http://example.com", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

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

	srv, tokenSrv := startServer(t, api_srv.URL, "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/ready", &tutils.AuthString0)
	require.Equal(t, 200, response.StatusCode)
	require.Contains(t, body, "{\"readiness\":\"ready\"}")
}

func TestMetrics(t *testing.T) {
	// simulate osbuild-composer API
	srv, tokenSrv := startServer(t, "", "*", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/metrics", nil)
	require.Equal(t, 200, response.StatusCode)
	require.Contains(t, body, "image_builder_compose_requests_total")
	require.Contains(t, body, "image_builder_compose_errors")
}
