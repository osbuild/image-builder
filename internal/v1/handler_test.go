package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/clients/provisioning"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/tutils"
	v1 "github.com/osbuild/image-builder/internal/v1"
)

func TestWithoutOsbuildComposerBackend(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	t.Run("GetVersion", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/version", &tutils.AuthString0)
		require.Equal(t, http.StatusOK, respStatusCode)

		var result v1.Version
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, "1.0", result.Version)
	})

	t.Run("GetOpenapiJson", func(t *testing.T) {
		// should work with AND without authentication
		testCases := []struct {
			url          string
			testCaseName string
			authString   *string
		}{
			{
				url:          "/api/image-builder/v1/openapi.json",
				testCaseName: "Test without Authentication",
				authString:   nil,
			},
			{
				url:          "/api/image-builder/v1/openapi.json",
				testCaseName: "Test with Authentication",
				authString:   &tutils.AuthString0,
			},
			{
				url:          "/api/image-builder/v1.0/openapi.json",
				testCaseName: "Test without Authentication (v1.0 URL)",
				authString:   nil,
			},
			{
				url:          "/openapi.json",
				testCaseName: "Test without Authentication (basic URL)",
				authString:   nil,
			},
		}

		for _, tc := range testCases {
			respStatusCode, body := tutils.GetResponseBody(t, srv.URL+tc.url, tc.authString)
			require.Equal(t, http.StatusOK, respStatusCode, tc.testCaseName)

			var swagger *openapi3.T
			var specB []byte
			var err error
			swagger, err = v1.GetSwagger()
			require.NoError(t, err)

			specB, err = swagger.MarshalJSON()
			require.NoError(t, err)

			spec := string(specB) + "\n"
			// improve readability of the diff - in case of errors
			spec = strings.ReplaceAll(spec, ",", ",\n")
			body = strings.ReplaceAll(body, ",", ",\n")

			require.Equal(t, spec, body)
			require.Equal(t, len(spec), len(body))

		}

	})

	t.Run("StatusCheck", func(t *testing.T) {
		respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+"/status", nil)
		require.Equal(t, http.StatusOK, respStatusCode)
	})
}

// note: this scenario needs to talk to a simulated osbuild-composer API
func TestGetComposeEntryNotFoundResponse(t *testing.T) {
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s",
		id), &tutils.AuthString0)
	require.Equal(t, http.StatusNotFound, respStatusCode)
	require.Contains(t, body, "Compose entry not found")
}

func TestGetComposeStatusErrorResponse(t *testing.T) {
	testCases := []struct {
		statusCode       int
		checkImageStatus bool
		expectedBody     string
	}{
		{http.StatusNotFound,
			true,
			"",
		},
		{http.StatusInternalServerError,
			false,
			"Failed querying compose status",
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		composeId := uuid.New()

		var composerStatus composer.ComposeStatus
		apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if "Bearer" == r.Header.Get("Authorization") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tc.statusCode)
			err := json.NewEncoder(w).Encode(composerStatus)
			require.NoError(t, err)
		}))
		defer apiSrv.Close()

		cr := v1.ComposeRequest{
			Distribution: "rhel-9",
			Customizations: &v1.Customizations{
				// Since we are not calling handleCommonCompose but inserting directly to DB
				// there is no point in using plaintext passwords
				// If there is plaintext password in DB there is problem elsewhere (eg. CreateBlueprint)
				Users: &[]v1.User{
					{
						Name:     "user",
						SshKey:   common.ToPtr("ssh-rsa AAAAB3NzaC2"),
						Password: common.ToPtr("$6$password123"),
					},
				},
			}}

		crRaw, err := json.Marshal(cr)
		require.NoError(t, err)
		srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
			DistributionsDir: "../../distributions",
		})
		defer srv.Shutdown(t)
		err = srv.DB.InsertCompose(ctx, composeId, "000000", "user000000@test.test", "000000", cr.ImageName, crRaw, (*string)(cr.ClientId), nil)
		require.NoError(t, err)

		payloads := []struct {
			composerStatus composer.ComposeStatus
			imageStatus    v1.ImageStatus
		}{
			{
				composerStatus: composer.ComposeStatus{
					ImageStatus: composer.ImageStatus{},
				},
			},
		}
		for idx, payload := range payloads {
			fmt.Printf("TT payload %d\n", idx)
			composerStatus = payload.composerStatus

			respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf(srv.URL+"/api/image-builder/v1/composes/%s",
				composeId), &tutils.AuthString0)
			require.Equal(t, tc.statusCode, respStatusCode)
			var result v1.ComposeStatus
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			if tc.checkImageStatus {
				require.Equal(t, payload.imageStatus, result.ImageStatus)
			}
			if tc.expectedBody != "" {
				require.Contains(t, body, tc.expectedBody)
			}
		}
		srv.Shutdown(t)
		apiSrv.Close()
	}
}

func TestGetComposeMetadata(t *testing.T) {
	ctx := context.Background()
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
		DistributionsDir: "../../distributions",
	})
	defer srv.Shutdown(t)

	err := srv.DB.InsertCompose(ctx, id, "500000", "user500000@test.test", "000000", common.ToPtr("MyImageName"), json.RawMessage("{}"), common.ToPtr("ui"), nil)
	require.NoError(t, err)

	var result composer.ComposeMetadata

	// Get API response and compare
	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+
		fmt.Sprintf("/api/image-builder/v1/composes/%s/metadata", id), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s/metadata",
		id), &tutils.AuthString0)
	require.Equal(t, http.StatusNotFound, respStatusCode)
	require.Contains(t, body, "Compose entry not found")
}

func TestGetComposes(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()
	id5 := uuid.New()
	id6 := uuid.New()

	srv := startServer(t, &testServerClientsConf{}, &v1.ServerConfig{
		DistributionsDir: "../../distributions",
	})
	defer srv.Shutdown(t)

	var result v1.ComposesResponse
	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/composes", &tutils.AuthString0)

	require.Equal(t, http.StatusOK, respStatusCode)
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, 0, result.Meta.Count)
	require.Contains(t, body, "\"data\":[]")

	imageName := "MyImageName"
	clientId := "ui"
	err = srv.DB.InsertCompose(ctx, id, "500000", "user500000@test.test", "000000", &imageName, json.RawMessage("{}"), &clientId, nil)
	require.NoError(t, err)
	err = srv.DB.InsertCompose(ctx, id2, "500000", "user500000@test.test", "000000", &imageName, json.RawMessage("{}"), &clientId, nil)
	require.NoError(t, err)
	err = srv.DB.InsertCompose(ctx, id3, "500000", "user500000@test.test", "000000", &imageName, json.RawMessage("{}"), &clientId, nil)
	require.NoError(t, err)

	composeEntry, err := srv.DB.GetCompose(ctx, id, "000000")
	require.NoError(t, err)

	respStatusCode, body = tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/composes", &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, composeEntry.CreatedAt.Format(time.RFC3339), result.Data[2].CreatedAt)
	require.Equal(t, composeEntry.Id, result.Data[2].Id)
	require.Nil(t, result.Data[2].BlueprintId)
	require.Nil(t, result.Data[2].BlueprintVersion)
	require.Equal(t, "/api/image-builder/v1.0/composes?limit=100&offset=0", result.Links.First)
	require.Equal(t, "/api/image-builder/v1.0/composes?limit=100&offset=2", result.Links.Last)
	require.Equal(t, 3, result.Meta.Count)
	require.Equal(t, 3, len(result.Data))

	bpId := uuid.New()
	versionId := uuid.New()
	err = srv.DB.InsertBlueprint(ctx, bpId, versionId, "000000", "500000", "bpName", "desc", json.RawMessage("{}"), json.RawMessage("{}"))
	require.NoError(t, err)

	err = srv.DB.InsertCompose(ctx, id4, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "edge-installer"}]}`), &clientId, &versionId)
	require.NoError(t, err)
	err = srv.DB.InsertCompose(ctx, id5, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &versionId)
	require.NoError(t, err)
	err = srv.DB.InsertCompose(ctx, id6, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "edge-commit"}]}`), &clientId, &versionId)
	require.NoError(t, err)

	respStatusCode, body = tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/composes?ignoreImageTypes=edge-installer&ignoreImageTypes=aws", &tutils.AuthString0)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Data))
	require.Equal(t, 1, result.Meta.Count)
	require.Equal(t, bpId, *result.Data[0].BlueprintId)
	require.Equal(t, 1, *result.Data[0].BlueprintVersion)
}

// TestBuildOSTreeOptions checks if the buildOSTreeOptions utility function
// properly transfers the ostree options to the Composer structure.
func TestBuildOSTreeOptions(t *testing.T) {
	cases := []struct {
		in  *v1.OSTree
		out *composer.OSTree
	}{
		{
			nil,
			nil,
		},
		{
			&v1.OSTree{Ref: common.ToPtr("someref")},
			&composer.OSTree{Ref: common.ToPtr("someref")},
		},
		{
			&v1.OSTree{Ref: common.ToPtr("someref"), Url: common.ToPtr("https://example.org")},
			&composer.OSTree{Ref: common.ToPtr("someref"), Url: common.ToPtr("https://example.org")},
		},
		{
			&v1.OSTree{Url: common.ToPtr("https://example.org")},
			&composer.OSTree{Url: common.ToPtr("https://example.org")},
		},
	}

	for _, c := range cases {
		require.Equal(t, c.out, v1.BuildOSTreeOptions(c.in), "input: %#v", c.in)
	}
}

func TestReadinessProbeNotReady(t *testing.T) {
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+"/ready", &tutils.AuthString0)
	require.NotEqual(t, http.StatusOK, respStatusCode)
	require.NotEqual(t, http.StatusNotFound, respStatusCode)
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/ready", &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	require.Contains(t, body, "{\"readiness\":\"ready\"}")
}

func TestMetrics(t *testing.T) {
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/metrics", nil)
	require.Equal(t, http.StatusOK, respStatusCode)
	require.Contains(t, body, "image_builder_crc_compose_requests_total")
	require.Contains(t, body, "image_builder_crc_compose_errors")
}

func TestGetClones(t *testing.T) {
	ctx := context.Background()
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL, ProvURL: provSrv.URL}, &v1.ServerConfig{
		DistributionsDir: "../../distributions",
	})
	defer srv.Shutdown(t)

	err := srv.DB.InsertCompose(ctx, id, "500000", "user500000@test.test", "000000", nil, json.RawMessage(`
{
  "image_requests": [
    {
      "image_type": "aws"
    }
  ]
}`), nil, nil)
	require.NoError(t, err)

	var csResp v1.ClonesResponse
	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s/clones", id), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &csResp)
	require.NoError(t, err)
	require.Equal(t, 0, len(csResp.Data))
	require.Contains(t, body, "\"data\":[]")

	cloneReq := v1.AWSEC2Clone{
		Region:           "us-east-2",
		ShareWithSources: &[]string{"1"},
	}
	respStatusCode, body = tutils.PostResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s/clone", id), cloneReq)
	require.Equal(t, http.StatusCreated, respStatusCode)

	var cResp v1.CloneResponse
	err = json.Unmarshal([]byte(body), &cResp)
	require.NoError(t, err)
	require.Equal(t, cloneId, cResp.Id)

	respStatusCode, body = tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s/clones", id), &tutils.AuthString0)
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
	ctx := context.Background()
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
			var uo composer.CloneStatus_Options
			require.NoError(t, uo.FromAWSEC2UploadStatus(composer.AWSEC2UploadStatus{
				Ami:    "ami-1",
				Region: "us-east-2",
			}))
			result := composer.CloneStatus{
				Options: uo,
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
		DistributionsDir: "../../distributions",
	})
	defer srv.Shutdown(t)

	err := srv.DB.InsertCompose(ctx, id, "500000", "user500000@test.test", "000000", nil, json.RawMessage(`
{
  "image_requests": [
    {
      "image_type": "aws"
    }
  ]
}`), nil, nil)
	require.NoError(t, err)

	cloneReq := v1.AWSEC2Clone{
		Region: "us-east-2",
	}
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s/clone", id), cloneReq)
	require.Equal(t, http.StatusCreated, respStatusCode)

	var cResp v1.CloneResponse
	err = json.Unmarshal([]byte(body), &cResp)
	require.NoError(t, err)
	require.Equal(t, cloneId, cResp.Id)

	var usResp v1.CloneStatusResponse
	respStatusCode, body = tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/clones/%s", cloneId), &tutils.AuthString0)

	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &usResp)
	require.NoError(t, err)
	require.Equal(t, v1.CloneStatusResponseStatusSuccess, usResp.Status)
	require.Equal(t, v1.UploadTypesAws, usResp.Type)
	require.Equal(t, id, *usResp.ComposeId)

	var awsUS v1.AWSUploadStatus
	jsonUO, err := json.Marshal(usResp.Options)
	require.NoError(t, err)
	err = json.Unmarshal(jsonUO, &awsUS)
	require.NoError(t, err)
	require.Equal(t, "ami-1", awsUS.Ami)
	require.Equal(t, "us-east-2", awsUS.Region)
}

func TestGetCloneEntryNotFoundResponse(t *testing.T) {
	id := uuid.New().String()
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/clones/%s",
		id), &tutils.AuthString0)
	require.Equal(t, http.StatusNotFound, respStatusCode)
	require.Contains(t, body, "Clone not found")
}

func TestValidateSpec(t *testing.T) {
	spec, err := v1.GetSwagger()
	require.NoError(t, err)
	err = spec.Validate(context.Background())
	require.NoError(t, err)
}

func TestGetArchitectures(t *testing.T) {
	distsDir := "../../distributions"
	allowFile := "../common/testdata/allow.json"
	srv := startServer(t, &testServerClientsConf{}, &v1.ServerConfig{
		DistributionsDir: distsDir,
		AllowFile:        allowFile,
	})
	defer srv.Shutdown(t)

	t.Run("Basic centos-9", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/architectures/centos-9", &tutils.AuthString0)
		require.Equal(t, http.StatusOK, respStatusCode)

		var result v1.Architectures
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, v1.Architectures{
			v1.ArchitectureItem{
				Arch:       "x86_64",
				ImageTypes: []string{"ami", "vhd", "aws", "gcp", "azure", "edge-commit", "edge-installer", "rhel-edge-commit", "rhel-edge-installer", "guest-image", "image-installer", "oci", "vsphere", "vsphere-ova", "wsl"},
				Repositories: []v1.Repository{
					{
						Baseurl: common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
						Rhsm:    false,
					}, {
						Baseurl: common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
						Rhsm:    false,
					},
				},
			},
			v1.ArchitectureItem{
				Arch:       "aarch64",
				ImageTypes: []string{"aws", "guest-image", "image-installer"},
				Repositories: []v1.Repository{
					{
						Baseurl: common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/"),
						Rhsm:    false,
					}, {
						Baseurl: common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/"),
						Rhsm:    false,
					},
				},
			}}, result)
	})

	t.Run("Restricted distribution", func(t *testing.T) {
		respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/architectures/fedora-39", &tutils.AuthString1)
		require.Equal(t, http.StatusForbidden, respStatusCode)
	})

	t.Run("Restricted, but allowed distribution", func(t *testing.T) {
		respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/architectures/fedora-39", &tutils.AuthString0)
		require.Equal(t, http.StatusOK, respStatusCode)
	})
}

func TestGetPackages(t *testing.T) {
	distsDir := "../../distributions"
	allowFile := "../common/testdata/allow.json"
	srv := startServer(t, &testServerClientsConf{}, &v1.ServerConfig{
		DistributionsDir: distsDir,
		AllowFile:        allowFile,
	})
	defer srv.Shutdown(t)
	architectures := []string{"x86_64", "aarch64"}

	t.Run("Simple search", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)

			var result v1.PackagesResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Contains(t, result.Data[0].Name, "ssh")
			require.Greater(t, result.Meta.Count, 0)
			require.Contains(t, result.Links.First, "search=ssh")
		}
	})

	t.Run("Empty search", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=4e3086991b3f452d82eed1f2122aefeb", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.PackagesResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Empty(t, result.Data)
			require.Contains(t, body, "\"data\":[]")

			respStatusCode, body = tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?offset=121039&distribution=rhel-8&architecture=%s&search=4e3086991b3f452d82eed1f2122aefeb", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			err = json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Empty(t, result.Data)
			require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/packages?search=4e3086991b3f452d82eed1f2122aefeb&distribution=rhel-8&architecture=%s&offset=0&limit=100", arch), result.Links.First)
			require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/packages?search=4e3086991b3f452d82eed1f2122aefeb&distribution=rhel-8&architecture=%s&offset=0&limit=100", arch), result.Links.Last)
		}
	})

	t.Run("Search with limit", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=1", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.PackagesResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Greater(t, result.Meta.Count, 1)
		}
	})

	t.Run("Search with offset", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=1&offset=1", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.PackagesResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Greater(t, result.Meta.Count, 1)

		}
	})

	t.Run("Search with invalid parameters", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=-13", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			respStatusCode, _ = tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=rhel-8&architecture=%s&search=ssh&limit=13&offset=-2193", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			respStatusCode, _ = tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=none&architecture=%s&search=ssh", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
		}
	})

	t.Run("Search restricted distribution", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=fedora-39&architecture=%s&search=ssh", arch), &tutils.AuthString1)
			require.Equal(t, http.StatusForbidden, respStatusCode)
		}
	})

	t.Run("Search restricted, but allowed distribution", func(t *testing.T) {
		for _, arch := range architectures {
			respStatusCode, _ := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/packages?distribution=fedora-39&architecture=%s&search=ssh", arch), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
		}
	})
}

func TestGetDistributions(t *testing.T) {
	distsDir := "../../distributions"
	allowFile := "../common/testdata/allow.json"
	srv := startServer(t, &testServerClientsConf{}, &v1.ServerConfig{
		DistributionsDir: distsDir,
		AllowFile:        allowFile,
	})
	defer srv.Shutdown(t)

	t.Run("Access to restricted distributions", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/distributions", &tutils.AuthString0)
		require.Equal(t, http.StatusOK, respStatusCode)
		var result v1.DistributionsResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		distros := []string{}
		for _, distro := range result {
			distros = append(distros, distro.Name)
		}
		require.ElementsMatch(t, []string{"rhel-8", "rhel-8-nightly", "rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-88", "rhel-89", "rhel-8.10", "rhel-9", "rhel-9-beta", "rhel-9-nightly", "rhel-90", "rhel-91", "rhel-92", "rhel-93", "rhel-94", "rhel-95", "rhel-10-beta", "rhel-10-nightly", "centos-9", "centos-10", "fedora-37", "fedora-38", "fedora-39", "fedora-40", "fedora-41"}, distros)
	})

	t.Run("No access to restricted distributions except global filter", func(t *testing.T) {
		respStatusCode, body := tutils.GetResponseBody(t, srv.URL+"/api/image-builder/v1/distributions", &tutils.AuthString1)
		require.Equal(t, http.StatusOK, respStatusCode)
		var result v1.DistributionsResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		distros := []string{}
		for _, distro := range result {
			distros = append(distros, distro.Name)
		}
		require.ElementsMatch(t, []string{"rhel-8-nightly", "rhel-8", "rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-88", "rhel-89", "rhel-8.10", "rhel-9-beta", "rhel-9-nightly", "rhel-9", "rhel-90", "rhel-91", "rhel-92", "rhel-93", "rhel-94", "rhel-95", "rhel-10-beta", "rhel-10-nightly", "centos-9"}, distros)
	})
}

func TestGetProfiles(t *testing.T) {
	distsDir := "../../distributions"
	allowFile := "../common/testdata/allow.json"
	srv := startServer(t, &testServerClientsConf{}, &v1.ServerConfig{
		DistributionsDir: distsDir,
		AllowFile:        allowFile,
	})
	defer srv.Shutdown(t)

	t.Run("Access profiles on all rhel8 variants returns a correct list of profiles", func(t *testing.T) {
		for _, dist := range []v1.Distributions{
			v1.Rhel8, v1.Rhel84, v1.Rhel85, v1.Rhel86, v1.Rhel87, v1.Rhel88, v1.Rhel89, v1.Rhel8Nightly,
		} {
			respStatusCode, body := tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/profiles", dist), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.DistributionProfileResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.ElementsMatch(t, v1.DistributionProfileResponse{
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28Enhanced,
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28High,
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28Intermediary,
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28Minimal,
				v1.XccdfOrgSsgprojectContentProfileCis,
				v1.XccdfOrgSsgprojectContentProfileCisServerL1,
				v1.XccdfOrgSsgprojectContentProfileCisWorkstationL1,
				v1.XccdfOrgSsgprojectContentProfileCisWorkstationL2,
				v1.XccdfOrgSsgprojectContentProfileCui,
				v1.XccdfOrgSsgprojectContentProfileE8,
				v1.XccdfOrgSsgprojectContentProfileHipaa,
				v1.XccdfOrgSsgprojectContentProfileIsmO,
				v1.XccdfOrgSsgprojectContentProfileOspp,
				v1.XccdfOrgSsgprojectContentProfilePciDss,
				v1.XccdfOrgSsgprojectContentProfileStig,
				v1.XccdfOrgSsgprojectContentProfileStigGui,
			}, result)
		}
	})

	t.Run("Access profiles on all rhel9 variants returns a correct list of profiles", func(t *testing.T) {
		for _, dist := range []v1.Distributions{
			v1.Rhel9, v1.Rhel91, v1.Rhel92, v1.Rhel93, v1.Rhel94, v1.Rhel9Nightly, v1.Centos9,
		} {
			respStatusCode, body := tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/profiles", dist), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.DistributionProfileResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.ElementsMatch(t, v1.DistributionProfileResponse{
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28Enhanced,
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28High,
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28Intermediary,
				v1.XccdfOrgSsgprojectContentProfileAnssiBp28Minimal,
				v1.XccdfOrgSsgprojectContentProfileCcnAdvanced,
				v1.XccdfOrgSsgprojectContentProfileCcnBasic,
				v1.XccdfOrgSsgprojectContentProfileCcnIntermediate,
				v1.XccdfOrgSsgprojectContentProfileCis,
				v1.XccdfOrgSsgprojectContentProfileCisServerL1,
				v1.XccdfOrgSsgprojectContentProfileCisWorkstationL1,
				v1.XccdfOrgSsgprojectContentProfileCisWorkstationL2,
				v1.XccdfOrgSsgprojectContentProfileCui,
				v1.XccdfOrgSsgprojectContentProfileE8,
				v1.XccdfOrgSsgprojectContentProfileHipaa,
				v1.XccdfOrgSsgprojectContentProfileIsmO,
				v1.XccdfOrgSsgprojectContentProfileOspp,
				v1.XccdfOrgSsgprojectContentProfilePciDss,
				v1.XccdfOrgSsgprojectContentProfileStig,
				v1.XccdfOrgSsgprojectContentProfileStigGui,
			}, result)
		}
	})

	t.Run("Access profiles on the other distros returns an error", func(t *testing.T) {
		for _, dist := range []v1.Distributions{v1.Fedora37, v1.Fedora38, v1.Fedora39, v1.Fedora40, v1.Fedora41, v1.Rhel90} {
			respStatusCode, _ := tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/profiles", dist), &tutils.AuthString0)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
		}
	})
}

func TestGetCustomizations(t *testing.T) {
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	t.Run("Access all customizations and check that they match", func(t *testing.T) {
		for _, dist := range []v1.Distributions{
			v1.Rhel8, v1.Rhel84, v1.Rhel85, v1.Rhel86, v1.Rhel87, v1.Rhel88, v1.Rhel8Nightly, v1.Rhel9, v1.Rhel91, v1.Rhel92, v1.Rhel9Nightly, v1.Centos9,
		} {
			respStatusCode, body := tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/profiles", dist), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.DistributionProfileResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			for _, profile := range result {
				// Get the customization from the API
				var result v1.Customizations
				respStatusCode, body := tutils.GetResponseBody(t,
					srv.URL+
						fmt.Sprintf("/api/image-builder/v1/oscap/%s/%s/customizations", dist, profile), &tutils.AuthString0)
				require.Equal(t, http.StatusOK, respStatusCode)
				err := json.Unmarshal([]byte(body), &result)
				require.NoError(t, err)
				// load the corresponding file from the disk
				require.NoError(t, err)
				jsonFile, err := os.Open(
					path.Join(
						"../../distributions",
						string(dist),
						"oscap",
						string(profile),
						"customizations.json"))
				require.NoError(t, err)
				defer jsonFile.Close()
				bytes, err := io.ReadAll(jsonFile)
				require.NoError(t, err)
				var customizations v1.Customizations
				err = json.Unmarshal(bytes, &customizations)
				require.NoError(t, err)
				// Make sure we get the same result both ways
				if result.Packages == nil {
					require.Nil(t, customizations.Packages)
				} else {
					require.ElementsMatch(t, *customizations.Packages, *result.Packages)
				}
				if result.Filesystem == nil {
					require.Nil(t, customizations.Filesystem)
				} else {
					require.ElementsMatch(t, *customizations.Filesystem, *result.Filesystem)
				}
				if result.Openscap == nil {
					require.Nil(t, customizations.Openscap)
				} else {
					require.Equal(t, *customizations.Openscap, *result.Openscap)
				}
			}
		}
	})
	t.Run("Access customizations on a distro that does not have customizations returns an error", func(t *testing.T) {
		for _, dist := range []v1.Distributions{v1.Rhel8} {
			respStatusCode, body := tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/profiles", dist), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.DistributionProfileResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			for _, profile := range result {
				respStatusCode, _ := tutils.GetResponseBody(t,
					srv.URL+
						fmt.Sprintf("/api/image-builder/v1/oscap/%s/%s/customizations", v1.Fedora40, profile), &tutils.AuthString0)
				require.Equal(t, http.StatusBadRequest, respStatusCode)
			}
		}
	})
	t.Run("Access non existing customizations on a distro returns an error", func(t *testing.T) {
		for _, dist := range []v1.Distributions{v1.Rhel8} {
			respStatusCode, body := tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/profiles", dist), &tutils.AuthString0)
			require.Equal(t, http.StatusOK, respStatusCode)
			var result v1.DistributionProfileResponse
			err := json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			respStatusCode, _ = tutils.GetResponseBody(t,
				srv.URL+
					fmt.Sprintf("/api/image-builder/v1/oscap/%s/%s/customizations", dist, "badprofile"), &tutils.AuthString0)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
		}
	})
}
