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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/provisioning"
	"github.com/osbuild/image-builder/internal/tutils"
)

func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv, tokenSrv := startServer(t, "", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	t.Run("GetVersion", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, 200, respStatusCode)

		var result Version
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "1.0", result.Version)
	})

	t.Run("GetOpenapiJson", func(t *testing.T) {
		respStatusCode, _ := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/openapi.json", &tutils.AuthString0)
		require.Equal(t, 200, respStatusCode)
		// note: not asserting body b/c response is too big
	})

	t.Run("GetDistributions", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/distributions", &tutils.AuthString0)
		require.Equal(t, 200, respStatusCode)

		var result DistributionsResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)

		for _, distro := range result {
			require.Contains(t, []string{"rhel-8", "rhel-8-nightly", "rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-88", "rhel-9", "rhel-9-nightly", "rhel-90", "rhel-91", "rhel-92", "centos-8", "centos-9", "fedora-35", "fedora-36", "fedora-37", "fedora-38", "fedora-39"}, distro.Name)
		}
	})

	t.Run("GetArchitectures", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/architectures/centos-8", &tutils.AuthString0)
		require.Equal(t, 200, respStatusCode)

		var result Architectures
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, Architectures{
			ArchitectureItem{
				Arch:       "x86_64",
				ImageTypes: []string{"aws", "gcp", "azure", "ami", "vhd", "guest-image", "image-installer", "vsphere", "vsphere-ova", "wsl"},
				Repositories: []Repository{
					{
						Baseurl: common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
						Rhsm:    false,
					}, {
						Baseurl: common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
						Rhsm:    false,
					}, {
						Baseurl: common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
						Rhsm:    false,
					},
				},
			},
			ArchitectureItem{
				Arch:       "aarch64",
				ImageTypes: []string{"aws", "guest-image", "image-installer"},
				Repositories: []Repository{
					{
						Baseurl: common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/aarch64/os/"),
						Rhsm:    false,
					}, {
						Baseurl: common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/aarch64/os/"),
						Rhsm:    false,
					}, {
						Baseurl: common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/aarch64/os/"),
						Rhsm:    false,
					},
				},
			}}, result)
	})

	t.Run("GetPackages", func(t *testing.T) {
		architectures := []string{"x86_64", "aarch64"}
		for _, arch := range architectures {
			respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh", arch), &tutils.AuthString0)
			require.Equal(t, 200, respStatusCode)

			var result PackagesResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Contains(t, result.Data[0].Name, "ssh")
			require.Greater(t, result.Meta.Count, 0)
			require.Contains(t, result.Links.First, "search=ssh")
			p1 := result.Data[0]
			p2 := result.Data[1]

			respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=4e3086991b3f452d82eed1f2122aefeb", arch), &tutils.AuthString0)
			require.Equal(t, 200, respStatusCode)
			err = json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Empty(t, result.Data)
			require.Contains(t, body, "\"data\":[]")

			respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?offset=121039&distribution=rhel-8&architecture=%s&search=4e3086991b3f452d82eed1f2122aefeb", arch), &tutils.AuthString0)
			require.Equal(t, 200, respStatusCode)
			err = json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Empty(t, result.Data)
			require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/packages?search=4e3086991b3f452d82eed1f2122aefeb&distribution=rhel-8&architecture=%s&offset=0&limit=100", arch), result.Links.First)
			require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/packages?search=4e3086991b3f452d82eed1f2122aefeb&distribution=rhel-8&architecture=%s&offset=0&limit=100", arch), result.Links.Last)

			respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=1", arch), &tutils.AuthString0)
			require.Equal(t, 200, respStatusCode)
			err = json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Greater(t, result.Meta.Count, 1)
			require.Equal(t, result.Data[0], p1)

			respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=1&offset=1", arch), &tutils.AuthString0)
			require.Equal(t, 200, respStatusCode)
			err = json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Greater(t, result.Meta.Count, 1)
			require.Equal(t, result.Data[0], p2)

			respStatusCode, _ = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=-13", arch), &tutils.AuthString0)
			require.Equal(t, 400, respStatusCode)
			respStatusCode, _ = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=13&offset=-2193", arch), &tutils.AuthString0)
			require.Equal(t, 400, respStatusCode)

			respStatusCode, _ = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/packages?distribution=none&architecture=%s&search=ssh", arch), &tutils.AuthString0)
			require.Equal(t, 400, respStatusCode)
		}
	})

	t.Run("StatusCheck", func(t *testing.T) {
		respStatusCode, _ := tutils.GetResponseBody(t, "http://localhost:8086/status", nil)
		require.Equal(t, 200, respStatusCode)
	})
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeStatus(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		s := composer.ComposeStatus{
			ImageStatus: composer.ImageStatus{
				Status: composer.ImageStatusValueBuilding,
			},
		}
		err := json.NewEncoder(w).Encode(s)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	imageName := "MyImageName"
	id := uuid.New()
	err = dbase.InsertCompose(id, "600000", "000001", &imageName, json.RawMessage(`
		{
			"distribution": "rhel-9",
			"image_requests": [],
			"image_name": "myimage"
		}`))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, "", dbase, "../../distributions", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		id), &tutils.AuthString1)
	require.Equal(t, 200, respStatusCode)

	var result ComposeStatus
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, ComposeStatus{
		ImageStatus: ImageStatus{
			Status: "building",
		},
		Request: ComposeRequest{
			Distribution:  "rhel-9",
			ImageName:     common.ToPtr("myimage"),
			ImageRequests: []ImageRequest{},
		},
	}, result)

	respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		id), &tutils.AuthString1)
	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, ComposeStatus{
		ImageStatus: ImageStatus{
			Status: "building",
		},
		Request: ComposeRequest{
			Distribution:  "rhel-9",
			ImageName:     common.ToPtr("myimage"),
			ImageRequests: []ImageRequest{},
		},
	}, result)
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeStatus404(t *testing.T) {
	id := uuid.New().String()
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		id), &tutils.AuthString0)
	require.Equal(t, 404, respStatusCode)
	require.Contains(t, body, "Compose not found")
}

func TestGetComposeMetadata(t *testing.T) {
	id := uuid.New()
	testPackages := []composer.PackageMetadata{
		{
			Arch:      "ArchTest2",
			Epoch:     common.ToPtr("EpochTest2"),
			Name:      "NameTest2",
			Release:   "ReleaseTest2",
			Sigmd5:    "Sigmd5Test2",
			Signature: common.ToPtr("SignatureTest2"),
			Type:      "TypeTest2",
			Version:   "VersionTest2",
		},
		{
			Arch:      "ArchTest1",
			Epoch:     common.ToPtr("EpochTest1"),
			Name:      "NameTest1",
			Release:   "ReleaseTest1",
			Sigmd5:    "Sigmd5Test1",
			Signature: common.ToPtr("SignatureTest1"),
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
			OstreeCommit: common.ToPtr("test string"),
			Packages:     &testPackages,
		}

		err := json.NewEncoder(w).Encode(m)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	imageName := "MyImageName"
	err = dbase.InsertCompose(id, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)

	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, "", dbase, "../../distributions", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var result composer.ComposeMetadata

	// Get API response and compare
	respStatusCode, body := tutils.GetResponseBody(t,
		fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/metadata", id), &tutils.AuthString0)
	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, *result.Packages, testPackages)
}

func TestGetComposeMetadata404(t *testing.T) {
	id := uuid.New().String()
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/metadata",
		id), &tutils.AuthString0)
	require.Equal(t, 404, respStatusCode)
	require.Contains(t, body, "Compose not found")
}

func TestGetComposes(t *testing.T) {
	id := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServerWithCustomDB(t, "", "", dbase, "../../distributions", "")
	defer func() {
		err := db_srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var result ComposesResponse
	respStatusCode, body := tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes", &tutils.AuthString0)

	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, 0, result.Meta.Count)
	require.Contains(t, body, "\"data\":[]")

	imageName := "MyImageName"
	err = dbase.InsertCompose(id, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)
	err = dbase.InsertCompose(id2, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)
	err = dbase.InsertCompose(id3, "500000", "000000", &imageName, json.RawMessage("{}"))
	require.NoError(t, err)

	composeEntry, err := dbase.GetCompose(id, "000000")
	require.NoError(t, err)

	respStatusCode, body = tutils.GetResponseBody(t, "http://localhost:8086/api/image-builder/v1/composes", &tutils.AuthString0)
	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, 3, result.Meta.Count)
	require.Equal(t, composeEntry.CreatedAt.Format(time.RFC3339), result.Data[2].CreatedAt)
	require.Equal(t, composeEntry.Id, result.Data[2].Id)
}

// TestBuildOSTreeOptions checks if the buildOSTreeOptions utility function
// properly transfers the ostree options to the Composer structure.
func TestBuildOSTreeOptions(t *testing.T) {
	cases := map[ImageRequest]*composer.OSTree{
		{Ostree: nil}: nil,
		{Ostree: &OSTree{Ref: common.ToPtr("someref")}}:                                           {Ref: common.ToPtr("someref")},
		{Ostree: &OSTree{Ref: common.ToPtr("someref"), Url: common.ToPtr("https://example.org")}}: {Ref: common.ToPtr("someref"), Url: common.ToPtr("https://example.org")},
		{Ostree: &OSTree{Url: common.ToPtr("https://example.org")}}:                               {Url: common.ToPtr("https://example.org")},
	}

	for in, expOut := range cases {
		require.Equal(t, expOut, buildOSTreeOptions(in.Ostree), "input: %#v", in)
	}
}

func TestReadinessProbeNotReady(t *testing.T) {
	srv, tokenSrv := startServer(t, "", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, _ := tutils.GetResponseBody(t, "http://localhost:8086/ready", &tutils.AuthString0)
	require.NotEqual(t, 200, respStatusCode)
	require.NotEqual(t, 404, respStatusCode)
}

func TestReadinessProbeReady(t *testing.T) {
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, "http://localhost:8086/ready", &tutils.AuthString0)
	require.Equal(t, 200, respStatusCode)
	require.Contains(t, body, "{\"readiness\":\"ready\"}")
}

func TestMetrics(t *testing.T) {
	srv, tokenSrv := startServer(t, "", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, "http://localhost:8086/metrics", nil)
	require.Equal(t, 200, respStatusCode)
	require.Contains(t, body, "image_builder_crc_compose_requests_total")
	require.Contains(t, body, "image_builder_crc_compose_errors")
}

func TestGetClones(t *testing.T) {
	id := uuid.New()
	cloneId := uuid.New()
	awsAccountId := "123456123456"

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		var cloneReq composer.AWSEC2CloneCompose
		err := json.NewDecoder(r.Body).Decode(&cloneReq)
		require.NoError(t, err)
		require.Equal(t, awsAccountId, (*cloneReq.ShareWithAccounts)[0])

		result := composer.CloneComposeResponse{
			Id: cloneId,
		}
		err = json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	provSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		awsId := struct {
			AccountId *string `json:"account_id,omitempty"`
		}{
			AccountId: &awsAccountId,
		}
		result := provisioning.V1SourceUploadInfoResponse{
			Aws: &awsId,
		}

		require.Equal(t, tutils.AuthString0, r.Header.Get("x-rh-identity"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer provSrv.Close()

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	err = dbase.InsertCompose(id, "500000", "000000", nil, json.RawMessage(`
{
  "image_requests": [
    {
      "image_type": "aws"
    }
  ]
}`))
	require.NoError(t, err)
	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, provSrv.URL, dbase, "../../distributions", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var csResp ClonesResponse
	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/clones", id), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &csResp)
	require.NoError(t, err)
	require.Equal(t, 0, len(csResp.Data))
	require.Contains(t, body, "\"data\":[]")

	cloneReq := AWSEC2Clone{
		Region:           "us-east-2",
		ShareWithSources: &[]string{"1"},
	}
	respStatusCode, body = tutils.PostResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/clone", id), cloneReq)
	require.Equal(t, http.StatusCreated, respStatusCode)

	var cResp CloneResponse
	err = json.Unmarshal([]byte(body), &cResp)
	require.NoError(t, err)
	require.Equal(t, cloneId, cResp.Id)

	respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/clones", id), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &csResp)
	require.NoError(t, err)
	require.Equal(t, 1, len(csResp.Data))
	require.Equal(t, cloneId, csResp.Data[0].Id)

	cloneReqExp, err := json.Marshal(cloneReq)
	require.NoError(t, err)
	cloneReqRecv, err := json.Marshal(csResp.Data[0].Request)
	require.NoError(t, err)
	require.Equal(t, cloneReqExp, cloneReqRecv)
}

func TestGetCloneStatus(t *testing.T) {
	cloneId := uuid.New()
	id := uuid.New()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, fmt.Sprintf("/clones/%v", cloneId)) && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			usO := composer.AWSEC2UploadStatus{
				Ami:    "ami-1",
				Region: "us-east-2",
			}
			result := composer.CloneStatus{
				Options: usO,
				Status:  composer.Success,
				Type:    composer.UploadTypesAws,
			}
			err := json.NewEncoder(w).Encode(result)
			require.NoError(t, err)
		} else if strings.HasSuffix(r.URL.Path, fmt.Sprintf("%v/clone", id)) && r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			result := composer.CloneComposeResponse{
				Id: cloneId,
			}
			err := json.NewEncoder(w).Encode(result)
			require.NoError(t, err)
		} else {
			require.FailNowf(t, "Unexpected request to mocked composer, path: %s", r.URL.Path)
		}
	}))
	defer apiSrv.Close()

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	err = dbase.InsertCompose(id, "500000", "000000", nil, json.RawMessage(`
{
  "image_requests": [
    {
      "image_type": "aws"
    }
  ]
}`))
	require.NoError(t, err)
	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, "", dbase, "../../distributions", "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	cloneReq := AWSEC2Clone{
		Region: "us-east-2",
	}
	respStatusCode, body := tutils.PostResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s/clone", id), cloneReq)
	require.Equal(t, http.StatusCreated, respStatusCode)

	var cResp CloneResponse
	err = json.Unmarshal([]byte(body), &cResp)
	require.NoError(t, err)
	require.Equal(t, cloneId, cResp.Id)

	var usResp UploadStatus
	respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/clones/%s", cloneId), &tutils.AuthString0)

	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &usResp)
	require.NoError(t, err)
	require.Equal(t, UploadStatusStatusSuccess, usResp.Status)
	require.Equal(t, UploadTypesAws, usResp.Type)

	var awsUS AWSUploadStatus
	jsonUO, err := json.Marshal(usResp.Options)
	require.NoError(t, err)
	err = json.Unmarshal(jsonUO, &awsUS)
	require.NoError(t, err)
	require.Equal(t, "ami-1", awsUS.Ami)
	require.Equal(t, "us-east-2", awsUS.Region)
}

func TestValidateSpec(t *testing.T) {
	spec, err := GetSwagger()
	require.NoError(t, err)
	err = spec.Validate(context.Background())
	require.NoError(t, err)
}
