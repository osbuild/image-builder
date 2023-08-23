package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/provisioning"
	"github.com/osbuild/image-builder/internal/tutils"
)

func TestValidateComposeRequest(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv, tokenSrv := startServer(t, "", "")
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
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": minimum number of items is 1`)
	})

	t.Run("ErrorsForTwoImageRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type: UploadTypesAws,
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: &[]string{"test-account"},
						},
					},
				},
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAmi,
					UploadRequest: UploadRequest{
						Type: UploadTypesAws,
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: &[]string{"test-account"},
						},
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": maximum number of items is 1`)
	})

	t.Run("ErrorsForEmptyAccountsAndSources", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type:    UploadTypesAws,
						Options: AWSUploadRequestOptions{},
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, respStatusCode)
		require.Contains(t, body, "Expected at least one source or account to share the image with")
	})

	azureRequest := func(source_id, subscription_id, tenant_id string) ImageRequest {
		options := make(map[string]string)
		options["resource_group"] = "group"
		if source_id != "" {
			options["source_id"] = source_id
		}
		if subscription_id != "" {
			options["subscription_id"] = subscription_id
		}
		if tenant_id != "" {
			options["tenant_id"] = tenant_id
		}
		optionsJSON, _ := json.Marshal(options)

		var azureOptions AzureUploadRequestOptions
		err := json.Unmarshal(optionsJSON, &azureOptions)
		require.NoError(t, err)

		azureRequest := ImageRequest{
			Architecture: "x86_64",
			ImageType:    ImageTypesAzure,
			UploadRequest: UploadRequest{
				Type:    UploadTypesAzure,
				Options: azureOptions,
			},
		}

		return azureRequest
	}

	azureTests := []struct {
		name    string
		request ImageRequest
	}{
		{name: "AzureInvalid1", request: azureRequest("", "", "")},
		{name: "AzureInvalid2", request: azureRequest("", "1", "")},
		{name: "AzureInvalid3", request: azureRequest("", "", "1")},
		{name: "AzureInvalid4", request: azureRequest("1", "1", "")},
		{name: "AzureInvalid5", request: azureRequest("1", "", "1")},
		{name: "AzureInvalid6", request: azureRequest("1", "1", "1")},
	}

	for _, tc := range azureTests {
		t.Run(tc.name, func(t *testing.T) {
			payload := ComposeRequest{
				Customizations: nil,
				Distribution:   "centos-8",
				ImageRequests:  []ImageRequest{tc.request},
			}
			respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
			require.Equal(t, 400, respStatusCode)
			require.Contains(t, body, "Request must contain either (1) a source id, and no tenant or subscription ids or (2) tenant and subscription ids, and no source id.")
		})
	}

	t.Run("ErrorsForZeroUploadRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture:  "x86_64",
					ImageType:     ImageTypesAzure,
					UploadRequest: UploadRequest{},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, respStatusCode)
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
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type: UploadTypesAws,
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: &[]string{"test-account"},
						},
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, respStatusCode)
		require.Contains(t, body, "Error at \\\"/image_requests/0/architecture\\\"")
	})

	t.Run("ErrorsForUnknownUploadType", func(t *testing.T) {
		// UploadRequest Type isn't supported
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAzure,
					UploadRequest: UploadRequest{
						Type: "unknown",
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: &[]string{"test-account"},
						},
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, 400, respStatusCode)
		require.Contains(t, body, "Error at \\\"/image_requests/0/upload_request/type\\\"")
	})

	t.Run("ErrorMaxSizeForAWSAndAzure", func(t *testing.T) {
		// 66 GiB total
		payload := ComposeRequest{
			Customizations: &Customizations{
				Filesystem: &[]Filesystem{
					{
						Mountpoint: "/",
						MinSize:    2147483648,
					},
					{
						Mountpoint: "/var",
						MinSize:    68719476736,
					},
				},
			},
			Distribution: "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture:  "x86_64",
					ImageType:     ImageTypesAmi,
					UploadRequest: UploadRequest{},
				},
			},
		}

		awsUr := UploadRequest{
			Type: UploadTypesAws,
			Options: AWSUploadRequestOptions{
				ShareWithAccounts: &[]string{"test-account"},
			},
		}

		azureUr := UploadRequest{
			Type: UploadTypesAzure,
			Options: AzureUploadRequestOptions{
				ResourceGroup:  "group",
				SubscriptionId: common.ToPtr("id"),
				TenantId:       common.ToPtr("tenant"),
				ImageName:      common.ToPtr("azure-image"),
			},
		}
		for _, it := range []ImageTypes{ImageTypesAmi, ImageTypesAws} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = awsUr
			respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
			require.Equal(t, 400, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total AWS image size cannot exceed %d bytes", FSMaxSize))
		}

		for _, it := range []ImageTypes{ImageTypesAzure, ImageTypesVhd} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = azureUr
			respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
			require.Equal(t, 400, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total Azure image size cannot exceed %d bytes", FSMaxSize))
		}
	})

	t.Run("ValidateFSSizes", func(t *testing.T) {
		buildComposeRequest := func(fsSize *uint64, imgSize *uint64, imgType ImageTypes) *ComposeRequest {
			cr := &ComposeRequest{
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture:  "x86_64",
						ImageType:     imgType,
						Size:          imgSize,
						UploadRequest: UploadRequest{},
					},
				},
			}

			// Add a filesystem size
			if fsSize != nil {
				cr.Customizations = &Customizations{
					Filesystem: &[]Filesystem{
						{
							Mountpoint: "/var",
							MinSize:    *fsSize,
						},
					},
				}
			}

			return cr
		}

		testData := []struct {
			fsSize  *uint64
			imgSize *uint64
			isError bool
		}{
			// Filesystem, Image, Error expected for ami/azure images
			{nil, nil, false}, // No sizes
			{common.ToPtr(uint64(68719476736)), nil, false}, // Just filesystem size, smaller than FSMaxSize

			{nil, common.ToPtr(uint64(13958643712)), false},                                  // Just image size, smaller than FSMaxSize
			{common.ToPtr(uint64(FSMaxSize + 1)), nil, true},                                 // Just filesystem size, larger than FSMaxSize
			{nil, common.ToPtr(uint64(FSMaxSize + 1)), true},                                 // Just image side, larger than FSMaxSize
			{common.ToPtr(uint64(68719476736)), common.ToPtr(uint64(13958643712)), false},    // filesystem smaller, image smaller
			{common.ToPtr(uint64(FSMaxSize + 1)), common.ToPtr(uint64(13958643712)), true},   // filesystem larger, image smaller
			{common.ToPtr(uint64(68719476736)), common.ToPtr(uint64(FSMaxSize + 1)), true},   // filesystem smaller, image larger
			{common.ToPtr(uint64(FSMaxSize + 1)), common.ToPtr(uint64(FSMaxSize + 1)), true}, // filesystem larger, image larger
		}

		// Guest Image has no errors even when the size is larger
		for idx, td := range testData {
			assert.Nil(t, validateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, ImageTypesGuestImage)), "%v: idx=%d", ImageTypesGuestImage, idx)
		}

		// Test the aws and azure types for expected errors
		for _, it := range []ImageTypes{ImageTypesAmi, ImageTypesAws, ImageTypesAzure, ImageTypesVhd} {
			for idx, td := range testData {
				if td.isError {
					assert.Error(t, validateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, it)), "%v: idx=%d", it, idx)
				} else {
					assert.Nil(t, validateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, it)), "%v: idx=%d", it, idx)
				}
			}
		}
	})
}

func TestComposeStatusError(t *testing.T) {
	id := uuid.New()
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
				Status: composer.ImageStatusValueFailure,
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

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	imageName := "MyImageName"
	err = dbase.InsertCompose(id, "600000", "000001", &imageName, json.RawMessage("{}"))
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
			Status: "failure",
			Error: &ComposeStatusError{
				Id:     23,
				Reason: "Marking errors: package",
			},
		},
		Request: ComposeRequest{},
	}, result)
}

func TestComposeImageErrorsWhenStatusCodeIsNotStatusCreated(t *testing.T) {
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
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
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type: UploadTypesAws,
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: &[]string{"test-account"},
					},
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusInternalServerError, respStatusCode)
	require.Contains(t, body, "Failed posting compose request to osbuild-composer")
}

func TestComposeImageErrorResolvingOSTree(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		serviceStat := &composer.Error{
			Id:     "10",
			Reason: "not ok",
		}
		err := json.NewEncoder(w).Encode(serviceStat)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	payload := ComposeRequest{
		Customizations: &Customizations{
			Packages: nil,
		},
		Distribution: "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypesEdgeCommit,
				Ostree: &OSTree{
					Ref: common.ToPtr("edge/ref"),
				},
				UploadRequest: UploadRequest{
					Type: UploadTypesAwsS3,
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: &[]string{"test-account"},
					},
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, 400, respStatusCode)
	require.Contains(t, body, "Error resolving OSTree repo")
}

func TestComposeImageErrorsWhenCannotParseResponse(t *testing.T) {
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
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
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type: UploadTypesAws,
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: &[]string{"test-account"},
					},
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, 500, respStatusCode)
	require.Contains(t, body, "Internal Server Error")
}

func TestComposeImageReturnsIdWhenNoErrors(t *testing.T) {
	id := uuid.New()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := composer.ComposeId{
			Id: id,
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, apiSrv.URL, "")
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
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type: UploadTypesAws,
					Options: AWSUploadRequestOptions{
						ShareWithAccounts: &[]string{"test-account"},
					},
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusCreated, respStatusCode)

	var result ComposeResponse
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, id, result.Id)
}

func TestComposeImageAllowList(t *testing.T) {
	distsDir := "../distribution/testdata/distributions"
	allowFile := "../common/testdata/allow.json"
	id := uuid.New()

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
				Id: id,
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
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type: UploadTypesAws,
						Options: AWSUploadRequestOptions{
							ShareWithAccounts: &[]string{"test-account"},
						},
					},
				},
			},
		}
	}

	t.Run("restricted distribution, allowed", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServerWithAllowFile(t, apiSrv.URL, "", "", distsDir, allowFile)
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("centos-8")

		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusCreated, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, id, result.Id)
	})

	t.Run("restricted distribution, forbidden", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServerWithAllowFile(t, apiSrv.URL, "", "", distsDir, allowFile)
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("rhel-8")

		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil, result.Id)
	})

	t.Run("restricted distribution, forbidden (no allowFile)", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServerWithAllowFile(t, apiSrv.URL, "", "", distsDir, "")
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("centos-8")

		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil, result.Id)
	})
}

func TestComposeCustomizations(t *testing.T) {
	var id uuid.UUID
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
		id = uuid.New()
		result := composer.ComposeId{
			Id: id,
		}
		err = json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	awsAccountId := "123456123456"
	provSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result provisioning.V1SourceUploadInfoResponse

		if r.URL.Path == "/sources/1/upload_info" {
			awsId := struct {
				AccountId *string `json:"account_id,omitempty"`
			}{
				AccountId: &awsAccountId,
			}
			result.Aws = &awsId
		}

		if r.URL.Path == "/sources/2/upload_info" {
			azureInfo := struct {
				ResourceGroups *[]string `json:"resource_groups,omitempty"`
				SubscriptionId *string   `json:"subscription_id,omitempty"`
				TenantId       *string   `json:"tenant_id,omitempty"`
			}{
				SubscriptionId: common.ToPtr("id"),
				TenantId:       common.ToPtr("tenant"),
				ResourceGroups: &[]string{"group"},
			}
			result.Azure = &azureInfo
		}

		require.Equal(t, tutils.AuthString0, r.Header.Get("x-rh-identity"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))

	srv, tokenSrv := startServer(t, apiSrv.URL, provSrv.URL)
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
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
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
					CustomRepositories: &[]CustomRepository{
						{
							Id:       "some-repo-id",
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
					},
					Openscap: &OpenSCAP{
						ProfileId: "test-profile",
					},
				},
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesRhelEdgeInstaller,
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
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
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         common.ToPtr(false),
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
							Key:    common.ToPtr("ssh-rsa AAAAB3NzaC1"),
							Groups: &[]string{"wheel"},
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Id:       "some-repo-id",
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
					},
					Openscap: &composer.OpenSCAP{
						ProfileId: "test-profile",
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesEdgeInstaller,
					Ostree:       nil,
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
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
						ImageType:    ImageTypesEdgeCommit,
						Ostree: &OSTree{
							Ref: common.ToPtr("edge/ref"),
						},
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: AWSS3UploadRequestOptions{},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-88",
				Customizations: &composer.Customizations{
					Packages: nil,
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesEdgeCommit,
					Ostree: &composer.OSTree{
						Ref: common.ToPtr("edge/ref"),
					},
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.8/x86_64/baseos/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.8/x86_64/appstream/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
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
						ImageType:    ImageTypesRhelEdgeCommit,
						Ostree: &OSTree{
							Ref:        common.ToPtr("test/edge/ref"),
							Url:        common.ToPtr("https://ostree.srv/"),
							Contenturl: common.ToPtr("https://ostree.srv/content"),
							Parent:     common.ToPtr("test/edge/ref2"),
							Rhsm:       common.ToPtr(true),
						},
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
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
						Rhc:           common.ToPtr(false),
						Organization:  "0",
						ServerUrl:     "",
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesEdgeCommit,
					Ostree: &composer.OSTree{
						Ref:        common.ToPtr("test/edge/ref"),
						Url:        common.ToPtr("https://ostree.srv/"),
						Contenturl: common.ToPtr("https://ostree.srv/content"),
						Parent:     common.ToPtr("test/edge/ref2"),
						Rhsm:       common.ToPtr(true),
					},
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		// Test Azure with SubscriptionId and TenantId
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesAzure,
						Ostree: &OSTree{
							Ref:    common.ToPtr("test/edge/ref"),
							Url:    common.ToPtr("https://ostree.srv/"),
							Parent: common.ToPtr("test/edge/ref2"),
						},
						UploadRequest: UploadRequest{
							Type: UploadTypesAzure,
							Options: AzureUploadRequestOptions{
								ResourceGroup:  "group",
								SubscriptionId: common.ToPtr("id"),
								TenantId:       common.ToPtr("tenant"),
								ImageName:      common.ToPtr("azure-image"),
							},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-8",
				Customizations: nil,
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesAzure,
					Ostree: &composer.OSTree{
						Ref:    common.ToPtr("test/edge/ref"),
						Url:    common.ToPtr("https://ostree.srv/"),
						Parent: common.ToPtr("test/edge/ref2"),
					},
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AzureUploadOptions{
						ImageName:      common.ToPtr("azure-image"),
						ResourceGroup:  "group",
						SubscriptionId: "id",
						TenantId:       "tenant",
					}),
				},
			},
		},
		// Test Azure with SourceId
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesAzure,
						Ostree: &OSTree{
							Ref:    common.ToPtr("test/edge/ref"),
							Url:    common.ToPtr("https://ostree.srv/"),
							Parent: common.ToPtr("test/edge/ref2"),
						},
						UploadRequest: UploadRequest{
							Type: UploadTypesAzure,
							Options: AzureUploadRequestOptions{
								ResourceGroup: "group",
								SourceId:      common.ToPtr("2"),
								ImageName:     common.ToPtr("azure-image"),
							},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-8",
				Customizations: nil,
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesAzure,
					Ostree: &composer.OSTree{
						Ref:    common.ToPtr("test/edge/ref"),
						Url:    common.ToPtr("https://ostree.srv/"),
						Parent: common.ToPtr("test/edge/ref2"),
					},
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AzureUploadOptions{
						ImageName:      common.ToPtr("azure-image"),
						ResourceGroup:  "group",
						SubscriptionId: "id",
						TenantId:       "tenant",
					}),
				},
			},
		},
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesAws,
						UploadRequest: UploadRequest{
							Type: UploadTypesAws,
							Options: AWSUploadRequestOptions{
								ShareWithSources: &[]string{"1"},
							},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-8",
				Customizations: nil,
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesAws,
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSEC2UploadOptions{
						ShareWithAccounts: []string{awsAccountId},
					}),
				},
			},
		},
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesAws,
						Size:         common.ToPtr(uint64(13958643712)),
						UploadRequest: UploadRequest{
							Type: UploadTypesAws,
							Options: AWSUploadRequestOptions{
								ShareWithSources: &[]string{"1"},
							},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-8",
				Customizations: nil,
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesAws,
					Size:         common.ToPtr(uint64(13958643712)),
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.centos.org/centos/8-stream/extras/x86_64/os/"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSEC2UploadOptions{
						ShareWithAccounts: []string{awsAccountId},
					}),
				},
			},
		},
		// just one partition
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Filesystem: &[]Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
				},
				Distribution: "rhel-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesGuestImage,
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: AWSS3UploadRequestOptions{},
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-88",
				Customizations: &composer.Customizations{
					Filesystem: &[]composer.Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.8/x86_64/baseos/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.8/x86_64/appstream/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
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
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload.imageBuilderRequest)
		require.Equal(t, http.StatusCreated, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, id, result.Id)

		//compare expected compose request with actual receieved compose request
		require.Equal(t, payload.composerRequest, composerRequest)
		composerRequest = composer.ComposeRequest{}
	}
}
