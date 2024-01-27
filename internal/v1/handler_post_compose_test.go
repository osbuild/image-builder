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
	srv, tokenSrv := startServer(t, "", "", nil)
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
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": minimum number of items is 1`)
	})

	t.Run("ErrorsForTwoImageRequests", func(t *testing.T) {
		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type:    UploadTypesAws,
						Options: uo,
					},
				},
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAmi,
					UploadRequest: UploadRequest{
						Type:    UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": maximum number of items is 1`)
	})

	t.Run("ErrorsForEmptyAccountsAndSources", func(t *testing.T) {
		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{}))

		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type:    UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
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

		var auo UploadRequest_Options
		var azureOptions AzureUploadRequestOptions
		err := json.Unmarshal(optionsJSON, &azureOptions)
		require.NoError(t, err)
		require.NoError(t, auo.FromAzureUploadRequestOptions(azureOptions))

		azureRequest := ImageRequest{
			Architecture: "x86_64",
			ImageType:    ImageTypesAzure,
			UploadRequest: UploadRequest{
				Type:    UploadTypesAzure,
				Options: auo,
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
			require.Equal(t, http.StatusBadRequest, respStatusCode)
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
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Regexp(t, "image_requests/0/upload_request/options|image_requests/0/upload_request/type", body)
		require.Regexp(t, "Value is not nullable|value is not one of the allowed values", body)
	})

	t.Run("ISEWhenRepositoriesNotFound", func(t *testing.T) {
		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))

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
						Type:    UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, "Error at \\\"/image_requests/0/architecture\\\"")
	})

	t.Run("ErrorsForUnknownUploadType", func(t *testing.T) {
		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		// UploadRequest Type isn't supported
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-8",
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAzure,
					UploadRequest: UploadRequest{
						Type:    "unknown",
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
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

		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		awsUr := UploadRequest{
			Type:    UploadTypesAws,
			Options: uo,
		}

		var auo UploadRequest_Options
		require.NoError(t, auo.FromAzureUploadRequestOptions(AzureUploadRequestOptions{
			ResourceGroup:  "group",
			SubscriptionId: common.ToPtr("id"),
			TenantId:       common.ToPtr("tenant"),
			ImageName:      common.ToPtr("azure-image"),
		}))
		azureUr := UploadRequest{
			Type:    UploadTypesAzure,
			Options: auo,
		}
		for _, it := range []ImageTypes{ImageTypesAmi, ImageTypesAws} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = awsUr
			respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total AWS image size cannot exceed %d bytes", FSMaxSize))
		}

		for _, it := range []ImageTypes{ImageTypesAzure, ImageTypesVhd} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = azureUr
			respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
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
	clientId := "ui"
	err = dbase.InsertCompose(id, "600000", "user@test.test", "000001", &imageName, json.RawMessage("{}"), &clientId, nil)
	require.NoError(t, err)

	srv, tokenSrv := startServer(t, apiSrv.URL, "", &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/composes/%s",
		id), &tutils.AuthString1)
	require.Equal(t, http.StatusOK, respStatusCode)

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

	srv, tokenSrv := startServer(t, apiSrv.URL, "", nil)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var uo UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type:    UploadTypesAws,
					Options: uo,
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "", nil)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var uo UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))

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
					Type:    UploadTypesAwsS3,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusBadRequest, respStatusCode)
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "", nil)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var uo UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type:    UploadTypesAws,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusInternalServerError, respStatusCode)
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

	srv, tokenSrv := startServer(t, apiSrv.URL, "", nil)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var uo UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type:    UploadTypesAws,
					Options: uo,
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
		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		return ComposeRequest{
			Customizations: nil,
			Distribution:   distro,
			ImageRequests: []ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    ImageTypesAws,
					UploadRequest: UploadRequest{
						Type:    UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
	}

	t.Run("restricted distribution, allowed", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServer(t, apiSrv.URL, "", &ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        allowFile,
		})
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

		srv, tokenSrv := startServer(t, apiSrv.URL, "", &ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        allowFile,
		})
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

		srv, tokenSrv := startServer(t, apiSrv.URL, "", &ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        "",
		})
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

	srv, tokenSrv := startServer(t, apiSrv.URL, provSrv.URL, nil)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var uo UploadRequest_Options
	require.NoError(t, uo.FromAWSS3UploadRequestOptions(AWSS3UploadRequestOptions{}))
	var ec2uo UploadRequest_Options
	require.NoError(t, ec2uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{awsAccountId},
	}))
	var auo UploadRequest_Options
	require.NoError(t, auo.FromAzureUploadRequestOptions(AzureUploadRequestOptions{
		ResourceGroup:  "group",
		SubscriptionId: common.ToPtr("id"),
		TenantId:       common.ToPtr("tenant"),
		ImageName:      common.ToPtr("azure-image"),
	}))
	var auo2 UploadRequest_Options
	require.NoError(t, auo2.FromAzureUploadRequestOptions(AzureUploadRequestOptions{
		ResourceGroup: "group",
		SourceId:      common.ToPtr("2"),
		ImageName:     common.ToPtr("azure-image"),
	}))

	var fileGroup File_Group
	require.NoError(t, fileGroup.FromFileGroup1(FileGroup1(1000)))
	var fileUser File_User
	require.NoError(t, fileUser.FromFileUser1(FileUser1(1000)))
	var composerFileGroup composer.File_Group
	require.NoError(t, composerFileGroup.FromFileGroup1(composer.FileGroup1(1000)))
	var composerFileUser composer.File_User
	require.NoError(t, composerFileUser.FromFileUser1(composer.FileUser1(1000)))

	var dirGroup Directory_Group
	require.NoError(t, dirGroup.FromDirectoryGroup1(DirectoryGroup1(1000)))
	var dirUser Directory_User
	require.NoError(t, dirUser.FromDirectoryUser1(DirectoryUser1(1000)))
	var composerDirGroup composer.Directory_Group
	require.NoError(t, composerDirGroup.FromDirectoryGroup1(DirectoryGroup1(1000)))
	var composerDirUser composer.Directory_User
	require.NoError(t, composerDirUser.FromDirectoryUser1(DirectoryUser1(1000)))

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
					/*
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
					*/
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
						{
							Name:   "user2",
							SshKey: "ssh-rsa AAAAB3NzaC2",
						},
					},
					Groups: common.ToPtr([]Group{
						{
							Name: "group",
						},
					}),
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
					Fips: &FIPS{
						Enabled: common.ToPtr(true),
					},
				},
				Distribution: "centos-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesRhelEdgeInstaller,
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: uo,
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
					/*
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
					*/
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
						{
							Name:   "user2",
							Key:    common.ToPtr("ssh-rsa AAAAB3NzaC2"),
							Groups: &[]string{"wheel"},
						},
					},
					Groups: common.ToPtr([]composer.Group{
						{
							Name: "group",
						},
					}),
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
					Fips: &composer.FIPS{
						Enabled: common.ToPtr(true),
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
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-89",
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
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/baseos/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/appstream/os"),
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
		// ostree, ignition, fdo
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Packages: &[]string{"pkg"},
					Subscription: &Subscription{
						Organization: 000,
					},
					Fdo: &FDO{
						DiunPubKeyHash: common.ToPtr("hash"),
					},
					Ignition: &Ignition{
						Embedded: &IgnitionEmbedded{
							Config: "config",
						},
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
							Options: uo,
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
					Fdo: &composer.FDO{
						DiunPubKeyHash: common.ToPtr("hash"),
					},
					Ignition: &composer.Ignition{
						Embedded: &composer.IgnitionEmbedded{
							Config: "config",
						},
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
							Type:    UploadTypesAzure,
							Options: auo,
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
							Type:    UploadTypesAzure,
							Options: auo2,
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
							Type:    UploadTypesAws,
							Options: ec2uo,
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
							Type:    UploadTypesAws,
							Options: ec2uo,
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
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-89",
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
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/baseos/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/appstream/os"),
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
				Distribution: "rhel-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesOci,
						UploadRequest: UploadRequest{
							Type:    UploadTypesOciObjectstorage,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-89",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesOci,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/baseos/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/appstream/os"),
							CheckGpg:    nil,
							Gpgkey:      nil,
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.OCIUploadOptions{}),
				},
			},
		},
		// One partition + partition_mode lvm
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Filesystem: &[]Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
					PartitioningMode: common.ToPtr(Lvm),
				},
				Distribution: "rhel-8",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesGuestImage,
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-89",
				Customizations: &composer.Customizations{
					Filesystem: &[]composer.Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
					PartitioningMode: common.ToPtr(composer.Lvm),
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl: common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/baseos/os"),
							Rhsm:    common.ToPtr(true),
						},
						{
							Baseurl: common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/appstream/os"),
							Rhsm:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		// files & directories customization
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Files: &[]File{
						{
							Data:          common.ToPtr("data"),
							EnsureParents: common.ToPtr(true),
							Group:         &fileGroup,
							Path:          "/etc/custom-file",
							User:          &fileUser,
						},
						{
							Data:          common.ToPtr("data"),
							EnsureParents: common.ToPtr(true),
							Path:          "/etc/custom-file2",
						},
					},
					Directories: &[]Directory{
						{
							EnsureParents: common.ToPtr(true),
							Group:         &dirGroup,
							Path:          "/etc/custom-file",
							User:          &dirUser,
							Mode:          common.ToPtr("0755"),
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
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-89",
				Customizations: &composer.Customizations{
					Files: &[]composer.File{
						{
							Data:          common.ToPtr("data"),
							EnsureParents: common.ToPtr(true),
							Group:         &composerFileGroup,
							Path:          "/etc/custom-file",
							User:          &composerFileUser,
						},
						{
							Data:          common.ToPtr("data"),
							EnsureParents: common.ToPtr(true),
							Path:          "/etc/custom-file2",
						},
					},
					Directories: &[]composer.Directory{
						{
							EnsureParents: common.ToPtr(true),
							Group:         &composerDirGroup,
							Path:          "/etc/custom-file",
							User:          &composerDirUser,
							Mode:          common.ToPtr("0755"),
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl: common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/baseos/os"),
							Rhsm:    common.ToPtr(true),
						},
						{
							Baseurl: common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/appstream/os"),
							Rhsm:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		// firewall, services, locale, tz, containers
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Firewall: &FirewallCustomization{
						Ports: common.ToPtr([]string{"1"}),
					},
					Services: &Services{
						Disabled: common.ToPtr([]string{"service"}),
					},
					Locale: &Locale{
						Keyboard: common.ToPtr("piano"),
					},
					Timezone: &Timezone{
						Timezone: common.ToPtr("antarctica"),
					},
					Containers: &[]Container{
						{
							Source: "container.io/test",
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
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-89",
				Customizations: &composer.Customizations{
					Firewall: &composer.FirewallCustomization{
						Ports: common.ToPtr([]string{"1"}),
					},
					Services: &composer.Services{
						Disabled: common.ToPtr([]string{"service"}),
					},
					Locale: &composer.Locale{
						Keyboard: common.ToPtr("piano"),
					},
					Timezone: &composer.Timezone{
						Timezone: common.ToPtr("antarctica"),
					},
					Containers: &[]composer.Container{
						{
							Source: "container.io/test",
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl: common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/baseos/os"),
							Rhsm:    common.ToPtr(true),
						},
						{
							Baseurl: common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.9/x86_64/appstream/os"),
							Rhsm:    common.ToPtr(true),
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
