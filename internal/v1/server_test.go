package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/osbuild/image-builder/internal/distribution"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/tutils"

	"github.com/labstack/echo/v4"
)

var UUIDTest string = "d1f631ff-b3a6-4eec-aa99-9e81d99bc93d"

// Create a temporary file containing quotas, returns the file name as a string
func initQuotaFile(t *testing.T) (string, error) {
	// create quotas with only the default values
	quotas := map[string]common.Quota{
		"default": {Quota: common.DefaultQuota, SlidingWindow: common.DefaultSlidingWindow},
	}
	jsonQuotas, err := json.Marshal(quotas)
	if err != nil {
		return "", err
	}

	// get a temp file to store the quotas
	file, err := ioutil.TempFile(t.TempDir(), "account_quotas.*.json")
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

func makeUploadOptions(t *testing.T, uploadOptions interface{}) *composer.UploadOptions {
	data, err := json.Marshal(uploadOptions)
	require.NoError(t, err)

	var result composer.UploadOptions
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	return &result
}

func startServerWithCustomDB(t *testing.T, url string, dbase db.DB, distsDir string, allowFile string) (*echo.Echo, *httptest.Server) {
	var log = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}

	err := logger.ConfigLogger(log, "DEBUG", "", "", "", "", "")
	require.NoError(t, err)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "rhsm-api", r.FormValue("client_id"))
		require.Equal(t, "offlinetoken", r.FormValue("refresh_token"))
		require.Equal(t, "refresh_token", r.FormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"access_token"`
		}{
			AccessToken: "accesstoken",
		})
		require.NoError(t, err)
	}))

	client, err := composer.NewClient(composer.ComposerClientConfig{
		ComposerURL:  url,
		TokenURL:     tokenServer.URL,
		ClientId:     "rhsm-api",
		OfflineToken: "offlinetoken",
	})
	require.NoError(t, err)

	//store the quotas in a temporary file
	quotaFile, err := initQuotaFile(t)
	require.NoError(t, err)

	adr, err := distribution.LoadDistroRegistry(distsDir)
	require.NoError(t, err)

	echoServer := echo.New()
	err = Attach(echoServer, client, dbase, AWSConfig{}, GCPConfig{}, AzureConfig{}, quotaFile, allowFile, adr)
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

func startServer(t *testing.T, url string) (*echo.Echo, *httptest.Server) {
	return startServerWithCustomDB(t, url, tutils.InitDB(), "../../distributions", "")
}

func startServerWithAllowFile(t *testing.T, url string, distsDir string, allowFile string) (*echo.Echo, *httptest.Server) {
	return startServerWithCustomDB(t, url, tutils.InitDB(), distsDir, allowFile)
}

// note: all of the sub-tests below don't actually talk to
// osbuild-composer API that's why they are groupped together
func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv, tokenSrv := startServer(t, "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	t.Run("VerifyIdentityHeaderMissing", func(t *testing.T) {
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", nil)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "missing x-rh-identity header")
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

		var result DistributionsResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)

		for _, distro := range result {
			require.Contains(t, []string{"rhel-8", "rhel-84", "rhel-85", "rhel-86", "rhel-9", "rhel-90", "centos-8", "centos-9"}, distro.Name)
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

	t.Run("GetPackages", func(t *testing.T) {
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
		require.Greater(t, result.Meta.Count, 1)
		require.Equal(t, result.Data[0], p1)

		response, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=1&offset=1", &tutils.AuthString0)
		require.Equal(t, 200, response.StatusCode)
		err = json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Greater(t, result.Meta.Count, 1)
		require.Equal(t, result.Data[0], p2)

		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=-13", &tutils.AuthString0)
		require.Equal(t, 400, response.StatusCode)
		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=x86_64&search=ssh&limit=13&offset=-2193", &tutils.AuthString0)
		require.Equal(t, 400, response.StatusCode)

		response, _ = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/packages?distribution=none&architecture=x86_64&search=ssh", &tutils.AuthString0)
		require.Equal(t, 400, response.StatusCode)
	})

	t.Run("AccountNumberFallback", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0WithoutEntitlements)
		require.Equal(t, 200, response.StatusCode)
	})

	t.Run("BogusAuthString", func(t *testing.T) {
		auth := "notbase64"
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "unable to b64 decode x-rh-identity header")
	})

	t.Run("BogusBase64AuthString", func(t *testing.T) {
		auth := "dGhpcyBpcyBkZWZpbml0ZWx5IG5vdCBqc29uCg=="
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "does not contain valid JSON")
	})

	t.Run("EmptyAccountNumber", func(t *testing.T) {
		// AccoundNumber equals ""
		auth := tutils.GetCompleteBas64Header("", "000000")
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "invalid or missing account number")
	})

	t.Run("EmptyOrgID", func(t *testing.T) {
		// OrgID equals ""
		auth := tutils.GetCompleteBas64Header("000000", "")
		response, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &auth)
		require.Equal(t, 400, response.StatusCode)
		require.Contains(t, body, "invalid or missing org_id")
	})

	t.Run("StatusCheck", func(t *testing.T) {
		response, _ := tutils.GetResponseBody(t, "http://localhost:8086/status", nil)
		require.Equal(t, 200, response.StatusCode)
	})
}

func TestOrgIdWildcard(t *testing.T) {
	srv, tokenSrv := startServer(t, "")
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
	srv, tokenSrv := startServer(t, "")
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
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		s := composer.ComposeStatus{
			ImageStatus: composer.ImageStatus{
				Status: composer.ImageStatusValue_building,
			},
		}
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	// insert a compose in the mock database
	dbase := tutils.InitDB()
	imageName := "MyImageName"
	err := dbase.InsertCompose(UUIDTest, "600000", "000001", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, dbase, "../../distributions", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		UUIDTest), &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)

	var result ComposeStatus
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, ComposeStatus{
		ImageStatus: ImageStatus{
			Status: "building",
		},
	}, result)

	response, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		UUIDTest), &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, ComposeStatus{
		ImageStatus: ImageStatus{
			Status: "building",
		},
	}, result)
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeStatus404(t *testing.T) {
	// simulate osbuild-composer API
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "404 during tests")
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
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
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		m := composer.ComposeMetadata{
			OstreeCommit: strptr("test string"),
			Packages:     &testPackages,
		}

		err := json.NewEncoder(w).Encode(m)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	// insert a compose in the mock database
	dbase := tutils.InitDB()
	imageName := "MyImageName"
	err := dbase.InsertCompose(UUIDTest, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, dbase, "../../distributions", "")
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
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "404 during tests")
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
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
	imageName := "MyImageName"
	err := dbase.InsertCompose(UUIDTest, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)
	err = dbase.InsertCompose(UUIDTest2, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)
	err = dbase.InsertCompose(UUIDTest3, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)

	composeEntry, err := dbase.GetCompose(UUIDTest, "000000")
	require.NoError(t, err)

	db_srv, tokenSrv := startServerWithCustomDB(t, "", dbase, "../../distributions", "")
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
	srv, tokenSrv := startServer(t, "")
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
		require.Contains(t, body, `Error at \"/image_requests\": minimum number of items is 1`)
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
		require.Contains(t, body, `Error at \"/image_requests\": maximum number of items is 1`)
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
		require.Contains(t, body, `request body has an error: doesn't match the schema: Error at \"/image_requests/0/architecture\": value is not one of the allowed values`)
	})

	t.Run("ErrorUserCustomizationNotAllowed", func(t *testing.T) {
		// User customization only permitted for installer types
		payload := ComposeRequest{
			Customizations: &Customizations{
				Packages: &[]string{
					"some",
					"packages",
				},
				Users: &[]User{
					{
						Name:   "user-name0",
						SshKey: "",
					},
					{
						Name:   "user-name1",
						SshKey: "",
					},
				},
			},
			Distribution: "centos-8",
			ImageRequests: []ImageRequest{
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
		require.Contains(t, body, "User customization only applies to installer image types")
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
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		s := "deliberately returning !201 during tests"
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
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
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	require.Contains(t, body, "Failed posting compose request to osbuild-composer")
}

func TestComposeImageErrorsWhenCannotParseResponse(t *testing.T) {
	// simulate osbuild-composer API
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		s := "not a composer.ComposeId data structure"
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
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
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := composer.ComposeId{
			Id: "3aa7375a-534a-4de3-8caf-011e04f402d3",
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
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

func TestComposeImageAllowList(t *testing.T) {
	distsDir := "../distribution/testdata/distributions"
	allowFile := "../common/testdata/allow.json"

	createApiSrv := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if "Bearer" == r.Header.Get("Authorization") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			result := composer.ComposeId{
				Id: "3aa7375a-534a-4de3-8caf-011e04f402d3",
			}
			err := json.NewEncoder(w).Encode(result)
			require.NoError(t, err)
		}))
	}

	createPayload := func(distro Distributions) ComposeRequest {
		return ComposeRequest{
			Customizations: nil,
			Distribution:   distro,
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
	}

	t.Run("restricted distribution, allowed", func(t *testing.T) {
		// simulate osbuild-composer API
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServerWithAllowFile(t, apiSrv.URL, distsDir, allowFile)
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("centos-8")

		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusCreated, response.StatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "3aa7375a-534a-4de3-8caf-011e04f402d3", result.Id)
	})

	t.Run("restricted distribution, forbidden", func(t *testing.T) {
		// simulate osbuild-composer API
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServerWithAllowFile(t, apiSrv.URL, distsDir, allowFile)
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("rhel-8")

		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, response.StatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "", result.Id)
	})

	t.Run("restricted distribution, forbidden (no allowFile)", func(t *testing.T) {
		// simulate osbuild-composer API
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServerWithAllowFile(t, apiSrv.URL, distsDir, "")
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("centos-8")

		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, response.StatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "", result.Id)
	})
}

// convenience function for string pointer fields
func strptr(s string) *string {
	return &s
}

func TestComposeCustomizations(t *testing.T) {
	// simulate osbuild-composer API
	var composerRequest composer.ComposeRequest
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))

		err := json.NewDecoder(r.Body).Decode(&composerRequest)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := composer.ComposeId{
			Id: "fe93fb55-ae04-4e21-a8b4-25ba95c3fa64",
		}
		err = json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	payloads := []struct {
		imageBuilderRequest ComposeRequest
		composerRequest     composer.ComposeRequest
	}{
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Packages: &[]string{
						"some",
						"packages",
					},
					PayloadRepositories: &[]Repository{
						{
							Baseurl:   common.StringToPtr("https://some-repo-base-url.org"),
							CheckGpg:  common.BoolToPtr(false),
							Gpgkey:    common.StringToPtr("some-gpg-key"),
							IgnoreSsl: common.BoolToPtr(false),
							Rhsm:      false,
						},
					},
					Filesystem: &[]Filesystem{
						{
							Mountpoint: "/",
							MinSize:    2147483648,
						},
						{
							Mountpoint: "/var",
							MinSize:    1073741824,
						},
					},
					Users: &[]User{
						{
							Name:   "user",
							SshKey: "ssh-rsa AAAAB3NzaC1",
						},
					},
				},
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypes_rhel_edge_installer,
						UploadRequest: UploadRequest{
							Type:    UploadTypes_aws_s3,
							Options: AWSS3UploadRequestOptions{},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "centos-8",
				Customizations: &composer.Customizations{
					Packages: &[]string{
						"some",
						"packages",
					},
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl:   common.StringToPtr("https://some-repo-base-url.org"),
							CheckGpg:  common.BoolToPtr(false),
							Gpgkey:    common.StringToPtr("some-gpg-key"),
							IgnoreSsl: common.BoolToPtr(false),
							Rhsm:      common.BoolToPtr(false),
						},
					},
					Filesystem: &[]composer.Filesystem{
						{
							Mountpoint: "/",
							MinSize:    2147483648,
						},
						{
							Mountpoint: "/var",
							MinSize:    1073741824,
						},
					},
					Users: &[]composer.User{
						{
							Name:   "user",
							Key:    common.StringToPtr("ssh-rsa AAAAB3NzaC1"),
							Groups: &[]string{"wheel"},
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypes_edge_installer,
					Ostree:       nil,
					Repositories: []composer.Repository{

						{
							Baseurl:     common.StringToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(false),
						},
						{
							Baseurl:     common.StringToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(false),
						},
						{
							Baseurl:     common.StringToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Packages: nil,
				},
				Distribution: "rhel-8",
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
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-86",
				Customizations: &composer.Customizations{
					Packages: nil,
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypes_edge_commit,
					Ostree: &composer.OSTree{
						Ref: strptr("edge/ref"),
					},
					Repositories: []composer.Repository{
						{
							Baseurl:     common.StringToPtr("https://cdn.redhat.com/content/dist/rhel8/8.6/x86_64/baseos/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(true),
						},
						{
							Baseurl:     common.StringToPtr("https://cdn.redhat.com/content/dist/rhel8/8.6/x86_64/appstream/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		{
			imageBuilderRequest: ComposeRequest{
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
							Ref:    strptr("test/edge/ref"),
							Url:    strptr("https://ostree.srv/"),
							Parent: strptr("test/edge/ref2"),
						},
						UploadRequest: UploadRequest{
							Type:    UploadTypes_aws_s3,
							Options: AWSS3UploadRequestOptions{},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "centos-8",
				Customizations: &composer.Customizations{
					Packages: &[]string{
						"pkg",
					},
					Subscription: &composer.Subscription{
						ActivationKey: "",
						BaseUrl:       "",
						Insights:      false,
						Organization:  "0",
						ServerUrl:     "",
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypes_edge_commit,
					Ostree: &composer.OSTree{
						Ref:    strptr("test/edge/ref"),
						Url:    strptr("https://ostree.srv/"),
						Parent: strptr("test/edge/ref2"),
					},
					Repositories: []composer.Repository{

						{
							Baseurl:     common.StringToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(false),
						},
						{
							Baseurl:     common.StringToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(false),
						},
						{
							Baseurl:     common.StringToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.BoolToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
	}

	for idx, payload := range payloads {
		fmt.Printf("TT payload %d\n", idx)
		response, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload.imageBuilderRequest)
		require.Equal(t, http.StatusCreated, response.StatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "fe93fb55-ae04-4e21-a8b4-25ba95c3fa64", result.Id)

		//compare expected compose request with actual receieved compose request
		require.Equal(t, payload.composerRequest, composerRequest)
		composerRequest = composer.ComposeRequest{}
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

func TestReadinessProbeNotReady(t *testing.T) {
	srv, tokenSrv := startServer(t, "")
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
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"version\":\"fake\"}")
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL)
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
	srv, tokenSrv := startServer(t, "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, "http://localhost:8086/metrics", nil)
	require.Equal(t, 200, response.StatusCode)
	require.Contains(t, body, "image_builder_crc_compose_requests_total")
	require.Contains(t, body, "image_builder_crc_compose_errors")
}

func TestComposeStatusError(t *testing.T) {
	// simulate osbuild-composer API
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		//nolint
		var manifestErrorDetails interface{}
		manifestErrorDetails = []composer.ComposeStatusError{
			composer.ComposeStatusError{
				Id:     23,
				Reason: "Marking errors: package",
			},
		}

		//nolint
		var osbuildErrorDetails interface{}
		osbuildErrorDetails = []composer.ComposeStatusError{
			composer.ComposeStatusError{
				Id:      5,
				Reason:  "dependency failed",
				Details: &manifestErrorDetails,
			},
		}

		s := composer.ComposeStatus{
			ImageStatus: composer.ImageStatus{
				Status: composer.ImageStatusValue_failure,
				Error: &composer.ComposeStatusError{
					Id:      9,
					Reason:  "depenceny failed",
					Details: &osbuildErrorDetails,
				},
			},
		}

		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	// insert a compose in the mock database
	dbase := tutils.InitDB()
	imageName := "MyImageName"
	err := dbase.InsertCompose(UUIDTest, "600000", "000001", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, dbase, "../../distributions", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	response, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		UUIDTest), &tutils.AuthString1)
	require.Equal(t, 200, response.StatusCode)

	var result ComposeStatus
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, ComposeStatus{
		ImageStatus: ImageStatus{
			Status: "failure",
			Error: &ComposeStatusError{
				Id:     23,
				Reason: "Marking errors: package",
			},
		},
	}, result)

}
