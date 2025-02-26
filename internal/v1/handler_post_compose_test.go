package v1_test

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

	"github.com/osbuild/image-builder-crc/internal/clients/composer"
	"github.com/osbuild/image-builder-crc/internal/clients/provisioning"
	"github.com/osbuild/image-builder-crc/internal/common"
	"github.com/osbuild/image-builder-crc/internal/tutils"
	v1 "github.com/osbuild/image-builder-crc/internal/v1"
	"github.com/osbuild/image-builder-crc/internal/v1/mocks"
)

func TestValidateComposeRequest(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	t.Run("ErrorsForZeroImageRequests", func(t *testing.T) {
		payload := v1.ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests:  []v1.ImageRequest{},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": minimum number of items is 1`)
	})

	t.Run("ErrorsForTwoImageRequests", func(t *testing.T) {
		var uo v1.UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		payload := v1.ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests: []v1.ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    v1.ImageTypesAws,
					UploadRequest: v1.UploadRequest{
						Type:    v1.UploadTypesAws,
						Options: uo,
					},
				},
				{
					Architecture: "x86_64",
					ImageType:    v1.ImageTypesAmi,
					UploadRequest: v1.UploadRequest{
						Type:    v1.UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": maximum number of items is 1`)
	})

	t.Run("ErrorsForEmptyAccountsAndSources", func(t *testing.T) {
		var uo v1.UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{}))

		payload := v1.ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests: []v1.ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    v1.ImageTypesAws,
					UploadRequest: v1.UploadRequest{
						Type:    v1.UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, "Expected at least one source or account to share the image with")
	})

	azureRequest := func(source_id, subscription_id, tenant_id string) v1.ImageRequest {
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

		var auo v1.UploadRequest_Options
		var azureOptions v1.AzureUploadRequestOptions
		err := json.Unmarshal(optionsJSON, &azureOptions)
		require.NoError(t, err)
		require.NoError(t, auo.FromAzureUploadRequestOptions(azureOptions))

		azureRequest := v1.ImageRequest{
			Architecture: "x86_64",
			ImageType:    v1.ImageTypesAzure,
			UploadRequest: v1.UploadRequest{
				Type:    v1.UploadTypesAzure,
				Options: auo,
			},
		}

		return azureRequest
	}

	azureTests := []struct {
		name    string
		request v1.ImageRequest
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
			payload := v1.ComposeRequest{
				Customizations: nil,
				Distribution:   "centos-9",
				ImageRequests:  []v1.ImageRequest{tc.request},
			}
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, "Request must contain either (1) a source id, and no tenant or subscription ids or (2) tenant and subscription ids, and no source id.")
		})
	}

	t.Run("ErrorsForZeroUploadRequests", func(t *testing.T) {
		payload := v1.ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests: []v1.ImageRequest{
				{
					Architecture:  "x86_64",
					ImageType:     v1.ImageTypesAzure,
					UploadRequest: v1.UploadRequest{},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Regexp(t, "image_requests/0/upload_request/options|image_requests/0/upload_request/type", body)
		require.Regexp(t, "Value is not nullable|value is not one of the allowed values|doesn't match any schema from", body)
	})

	t.Run("ISEWhenRepositoriesNotFound", func(t *testing.T) {
		var uo v1.UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))

		// Distro arch isn't supported which triggers error when searching
		// for repositories
		payload := v1.ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests: []v1.ImageRequest{
				{
					Architecture: "unsupported-arch",
					ImageType:    v1.ImageTypesAws,
					UploadRequest: v1.UploadRequest{
						Type:    v1.UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, "Error at \\\"/image_requests/0/architecture\\\"")
	})

	t.Run("ErrorsForUnknownUploadType", func(t *testing.T) {
		var uo v1.UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		// UploadRequest Type isn't supported
		payload := v1.ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests: []v1.ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    v1.ImageTypesAzure,
					UploadRequest: v1.UploadRequest{
						Type:    "unknown",
						Options: uo,
					},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, "Error at \\\"/image_requests/0/upload_request/type\\\"")
	})

	t.Run("ErrorMaxSizeForAWSAndAzure", func(t *testing.T) {
		// 66 GiB total
		payload := v1.ComposeRequest{
			Customizations: &v1.Customizations{
				Filesystem: &[]v1.Filesystem{
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
			Distribution: "centos-9",
			ImageRequests: []v1.ImageRequest{
				{
					Architecture:  "x86_64",
					ImageType:     v1.ImageTypesAmi,
					UploadRequest: v1.UploadRequest{},
				},
			},
		}

		var uo v1.UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		awsUr := v1.UploadRequest{
			Type:    v1.UploadTypesAws,
			Options: uo,
		}

		var auo v1.UploadRequest_Options
		require.NoError(t, auo.FromAzureUploadRequestOptions(v1.AzureUploadRequestOptions{
			ResourceGroup:  "group",
			SubscriptionId: common.ToPtr("id"),
			TenantId:       common.ToPtr("tenant"),
			ImageName:      common.ToPtr("azure-image"),
		}))
		azureUr := v1.UploadRequest{
			Type:    v1.UploadTypesAzure,
			Options: auo,
		}
		for _, it := range []v1.ImageTypes{v1.ImageTypesAmi, v1.ImageTypesAws} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = awsUr
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total AWS image size cannot exceed %d bytes", v1.FSMaxSize))
		}

		for _, it := range []v1.ImageTypes{v1.ImageTypesAzure, v1.ImageTypesVhd} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = azureUr
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total Azure image size cannot exceed %d bytes", v1.FSMaxSize))
		}
	})

	t.Run("ValidateFSSizes", func(t *testing.T) {
		buildComposeRequest := func(fsSize *uint64, imgSize *uint64, imgType v1.ImageTypes) *v1.ComposeRequest {
			cr := &v1.ComposeRequest{
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture:  "x86_64",
						ImageType:     imgType,
						Size:          imgSize,
						UploadRequest: v1.UploadRequest{},
					},
				},
			}

			// Add a filesystem size
			if fsSize != nil {
				cr.Customizations = &v1.Customizations{
					Filesystem: &[]v1.Filesystem{
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
			{common.ToPtr(uint64(68719476736)), nil, false}, // Just filesystem size, smaller than v1.FSMaxSize

			{nil, common.ToPtr(uint64(13958643712)), false},                                        // Just image size, smaller than v1.FSMaxSize
			{common.ToPtr(uint64(v1.FSMaxSize + 1)), nil, true},                                    // Just filesystem size, larger than v1.FSMaxSize
			{nil, common.ToPtr(uint64(v1.FSMaxSize + 1)), true},                                    // Just image side, larger than v1.FSMaxSize
			{common.ToPtr(uint64(68719476736)), common.ToPtr(uint64(13958643712)), false},          // filesystem smaller, image smaller
			{common.ToPtr(uint64(v1.FSMaxSize + 1)), common.ToPtr(uint64(13958643712)), true},      // filesystem larger, image smaller
			{common.ToPtr(uint64(68719476736)), common.ToPtr(uint64(v1.FSMaxSize + 1)), true},      // filesystem smaller, image larger
			{common.ToPtr(uint64(v1.FSMaxSize + 1)), common.ToPtr(uint64(v1.FSMaxSize + 1)), true}, // filesystem larger, image larger
		}

		// Guest Image has no errors even when the size is larger
		for idx, td := range testData {
			assert.Nil(t, v1.ValidateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, v1.ImageTypesGuestImage)), "%v: idx=%d", v1.ImageTypesGuestImage, idx)
		}

		// Test the aws and azure types for expected errors
		for _, it := range []v1.ImageTypes{v1.ImageTypesAmi, v1.ImageTypesAws, v1.ImageTypesAzure, v1.ImageTypesVhd} {
			for idx, td := range testData {
				if td.isError {
					assert.Error(t, v1.ValidateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, it)), "%v: idx=%d", it, idx)
				} else {
					assert.Nil(t, v1.ValidateComposeRequest(buildComposeRequest(td.fsSize, td.imgSize, it)), "%v: idx=%d", it, idx)
				}
			}
		}
	})
}

func TestComposeStatusError(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		//nolint
		var manifestErrorDetails any = []composer.ComposeStatusError{
			{
				Id:     23,
				Reason: "Marking errors: package",
			},
		}

		//nolint
		var osbuildErrorDetails any = []composer.ComposeStatusError{
			{
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
		DistributionsDir: "../../distributions",
	})
	defer srv.Shutdown(t)

	err := srv.DB.InsertCompose(ctx, id, "600000", "user@test.test", "000001", common.ToPtr("MyImageName"), json.RawMessage("{}"), common.ToPtr("ui"), nil)
	require.NoError(t, err)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s",
		id), &tutils.AuthString1)
	require.Equal(t, http.StatusOK, respStatusCode)

	var result v1.ComposeStatus
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, v1.ComposeStatus{
		ImageStatus: v1.ImageStatus{
			Status: "failure",
			Error: &v1.ComposeStatusError{
				Id:     23,
				Reason: "Marking errors: package",
			},
		},
		Request: v1.ComposeRequest{},
	}, result)
}

func TestComposeImageErrorsWhenStatusCodeIsNotStatusCreated(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := v1.ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-9",
		ImageRequests: []v1.ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    v1.ImageTypesAws,
				UploadRequest: v1.UploadRequest{
					Type:    v1.UploadTypesAws,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusInternalServerError, respStatusCode)
	require.Contains(t, body, "Failed posting compose request to osbuild-composer")
}

func TestComposeImageErrorResolvingOSTree(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))

	payload := v1.ComposeRequest{
		Customizations: &v1.Customizations{
			Packages: nil,
		},
		Distribution: "centos-9",
		ImageRequests: []v1.ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    v1.ImageTypesEdgeCommit,
				Ostree: &v1.OSTree{
					Ref: common.ToPtr("edge/ref"),
				},
				UploadRequest: v1.UploadRequest{
					Type:    v1.UploadTypesAwsS3,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusBadRequest, respStatusCode)
	require.Contains(t, body, "Error resolving OSTree repo")
}

func TestComposeImageErrorsWhenCannotParseResponse(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := v1.ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-9",
		ImageRequests: []v1.ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    v1.ImageTypesAws,
				UploadRequest: v1.UploadRequest{
					Type:    v1.UploadTypesAws,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusInternalServerError, respStatusCode)
	require.Contains(t, body, "Internal Server Error")
}

// This test case queries the image-builder for a non existing type of the os distribution
// osbuild-composer is not being mock here as the error should be intercepted by image-builder
func TestComposeImageErrorsWhenDistributionNotExists(t *testing.T) {
	srv := startServer(t, &testServerClientsConf{}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := v1.ComposeRequest{
		Customizations: nil,
		Distribution:   "fedoros",
		ImageRequests: []v1.ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    v1.ImageTypesAws,
				UploadRequest: v1.UploadRequest{
					Type:    v1.UploadTypesAws,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, _ := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusBadRequest, respStatusCode)
}

func TestComposeImageReturnsIdWhenNoErrors(t *testing.T) {
	id := uuid.New()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
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

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{"test-account"},
	}))
	payload := v1.ComposeRequest{
		Customizations: nil,
		Distribution:   "centos-9",
		ImageRequests: []v1.ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    v1.ImageTypesAws,
				UploadRequest: v1.UploadRequest{
					Type:    v1.UploadTypesAws,
					Options: uo,
				},
			},
		},
	}
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusCreated, respStatusCode)

	var result v1.ComposeResponse
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
			if r.Header.Get("Authorization") == "Bearer" {
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

	createPayload := func(distro v1.Distributions) v1.ComposeRequest {
		var uo v1.UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
			ShareWithAccounts: &[]string{"test-account"},
		}))
		return v1.ComposeRequest{
			Customizations: nil,
			Distribution:   distro,
			ImageRequests: []v1.ImageRequest{
				{
					Architecture: "x86_64",
					ImageType:    v1.ImageTypesAws,
					UploadRequest: v1.UploadRequest{
						Type:    v1.UploadTypesAws,
						Options: uo,
					},
				},
			},
		}
	}

	t.Run("restricted distribution, allowed", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        allowFile,
		})
		defer srv.Shutdown(t)

		payload := createPayload("centos-9")

		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusCreated, respStatusCode)

		var result v1.ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, id, result.Id)
	})

	t.Run("restricted distribution, forbidden", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        allowFile,
		})
		defer srv.Shutdown(t)

		payload := createPayload("rhel-8")

		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, respStatusCode)

		var result v1.ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil, result.Id)
	})

	t.Run("restricted distribution, forbidden (no allowFile)", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &v1.ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        "",
		})
		defer srv.Shutdown(t)

		payload := createPayload("rhel-8")

		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, respStatusCode)

		var result v1.ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil, result.Id)
	})
}

func TestComposeWithSnapshots(t *testing.T) {
	var composeId uuid.UUID
	var composerRequest composer.ComposeRequest
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		err := json.NewDecoder(r.Body).Decode(&composerRequest)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		composeId = uuid.New()
		result := composer.ComposeId{
			Id: composeId,
		}
		err = json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSS3UploadRequestOptions(v1.AWSS3UploadRequestOptions{}))
	var uoGCP v1.UploadRequest_Options
	require.NoError(t, uoGCP.FromGCPUploadRequestOptions(v1.GCPUploadRequestOptions{
		ShareWithAccounts: common.ToPtr([]string{"user:example@example.com"}),
	}))
	payloads := []struct {
		imageBuilderRequest v1.ComposeRequest
		composerRequest     composer.ComposeRequest
	}{
		// basic without payload or custom repositories
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30T00:00:00Z"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		// basic with old snapshotting date format
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
		},
		// 1 payload 2 custom repositories
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30T00:00:00Z"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
				Customizations: &v1.Customizations{
					PayloadRepositories: &[]v1.Repository{
						{
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
						},
					},
					CustomRepositories: &[]v1.CustomRepository{
						{
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							CheckGpg: common.ToPtr(true),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
						{
							Baseurl: &[]string{"https://some-repo-base-url2.org"},
							Id:      mocks.RepoPLID2,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
				Customizations: &composer.Customizations{
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl:  common.ToPtr("https://content-sources.org/snappy/payload"),
							CheckGpg: common.ToPtr(true),
							Gpgkey:   common.ToPtr("some-gpg-key"),
							Rhsm:     common.ToPtr(false),
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Baseurl:  &[]string{"http://snappy-url/snappy/payload"},
							Name:     common.ToPtr("payload"),
							CheckGpg: common.ToPtr(true),
							Enabled:  common.ToPtr(false),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
						{
							Baseurl: &[]string{"http://snappy-url/snappy/payload2"},
							Name:    common.ToPtr("payload2"),
							Enabled: common.ToPtr(false),
							Id:      mocks.RepoPLID2,
						},
					},
				},
			},
		},
		// 2 payload 1 custom repository
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30T00:00:00Z"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
				Customizations: &v1.Customizations{
					PayloadRepositories: &[]v1.Repository{
						{
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
						},
						{
							Baseurl: common.ToPtr("https://some-repo-base-url2.org"),
						},
					},
					CustomRepositories: &[]v1.CustomRepository{
						{
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							CheckGpg: common.ToPtr(true),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
				Customizations: &composer.Customizations{
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl:  common.ToPtr("https://content-sources.org/snappy/payload"),
							CheckGpg: common.ToPtr(true),
							Gpgkey:   common.ToPtr("some-gpg-key"),
							Rhsm:     common.ToPtr(false),
						},
						{
							Baseurl: common.ToPtr("https://content-sources.org/snappy/payload2"),
							Rhsm:    common.ToPtr(false),
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Baseurl:  &[]string{"http://snappy-url/snappy/payload"},
							Name:     common.ToPtr("payload"),
							CheckGpg: common.ToPtr(true),
							Enabled:  common.ToPtr(false),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
					},
				},
			},
		},
		// repositories by uuid
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30T00:00:00Z"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
				Customizations: &v1.Customizations{
					PayloadRepositories: &[]v1.Repository{
						{
							Id:           common.ToPtr(mocks.RepoPLID),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
						},
					},
					CustomRepositories: &[]v1.CustomRepository{
						{
							CheckGpg: common.ToPtr(true),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
						{
							Id: mocks.RepoPLID2,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
				Customizations: &composer.Customizations{
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl:  common.ToPtr("https://content-sources.org/snappy/payload"),
							CheckGpg: common.ToPtr(true),
							Gpgkey:   common.ToPtr("some-gpg-key"),
							Rhsm:     common.ToPtr(false),
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Baseurl:  &[]string{"http://snappy-url/snappy/payload"},
							Name:     common.ToPtr("payload"),
							CheckGpg: common.ToPtr(true),
							Enabled:  common.ToPtr(false),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
						{
							Baseurl: &[]string{"http://snappy-url/snappy/payload2"},
							Name:    common.ToPtr("payload2"),
							Enabled: common.ToPtr(false),
							Id:      mocks.RepoPLID2,
						},
					},
				},
			},
		},
		// gcp
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGcp,
						SnapshotDate: common.ToPtr("1999-01-30T00:00:00Z"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesGcp,
							Options: uoGCP,
						},
					},
				},
				Customizations: &v1.Customizations{
					PayloadRepositories: &[]v1.Repository{
						{
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
						},
					},
					CustomRepositories: &[]v1.CustomRepository{
						{
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							CheckGpg: common.ToPtr(true),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGcp,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://packages.cloud.google.com/yum/repos/google-compute-engine-el9-x86_64-stable"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.GcpGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://packages.cloud.google.com/yum/repos/cloud-sdk-el9-x86_64"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.GcpGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						}, {
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.GCPUploadOptions{
						Region:            "",
						Bucket:            common.ToPtr(""),
						ShareWithAccounts: common.ToPtr([]string{"user:example@example.com"}),
					}),
				},
				Customizations: &composer.Customizations{
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl:  common.ToPtr("https://content-sources.org/snappy/payload"),
							CheckGpg: common.ToPtr(true),
							Gpgkey:   common.ToPtr("some-gpg-key"),
							Rhsm:     common.ToPtr(false),
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Baseurl:  &[]string{"http://snappy-url/snappy/payload"},
							Name:     common.ToPtr("payload"),
							CheckGpg: common.ToPtr(true),
							Enabled:  common.ToPtr(false),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       mocks.RepoPLID,
						},
					},
				},
			},
		},
		// 1 payload & custom repository with an empty, but not-nil gpg key
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-95",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30T00:00:00Z"),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
				Customizations: &v1.Customizations{
					PayloadRepositories: &[]v1.Repository{
						{
							Id: common.ToPtr(mocks.RepoPLID3),
						},
					},
					CustomRepositories: &[]v1.CustomRepository{
						{
							Id: mocks.RepoPLID3,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-9.5",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
				Customizations: &composer.Customizations{
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl: common.ToPtr("https://content-sources.org/snappy/payload3"),
							Rhsm:    common.ToPtr(false),
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Baseurl: &[]string{"http://snappy-url/snappy/payload3"},
							Name:    common.ToPtr("payload3"),
							Enabled: common.ToPtr(false),
							Id:      mocks.RepoPLID3,
						},
					},
				},
			},
		},
	}

	for idx, payload := range payloads {
		fmt.Printf("TT payload %d\n", idx)
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload.imageBuilderRequest)
		if respStatusCode != http.StatusCreated {
			fmt.Printf("Body: %s\n", body)
		}
		require.Equal(t, http.StatusCreated, respStatusCode)
		var result v1.ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, composeId, result.Id)
		require.Equal(t, payload.composerRequest, composerRequest)
		composerRequest = composer.ComposeRequest{}
	}
}

func TestComposeCustomizations(t *testing.T) {
	var id uuid.UUID
	var composerRequest composer.ComposeRequest
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
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

	policyID := uuid.New()
	complSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fmt.Sprintf("/policies/%s", policyID):
			policyData := struct {
				Data struct {
					ID             string `json:"id"`
					RefID          string `json:"ref_id"`
					OSMajorVersion int    `json:"os_major_version"`
				} `json:"data"`
			}{
				Data: struct {
					ID             string `json:"id"`
					RefID          string `json:"ref_id"`
					OSMajorVersion int    `json:"os_major_version"`
				}{
					ID:             policyID.String(),
					RefID:          "openscap-ref-id",
					OSMajorVersion: 8,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(policyData)
			require.NoError(t, err)
		case fmt.Sprintf("/policies/%s/tailorings/10/tailoring_file.json", policyID):
			tailoringData := "{ \"data\": \"some-tailoring-data\"}"
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(tailoringData))
			require.NoError(t, err)
		}
	}))

	srv := startServer(t, &testServerClientsConf{
		ComposerURL:   apiSrv.URL,
		ProvURL:       provSrv.URL,
		ComplianceURL: complSrv.URL,
	}, nil)
	defer srv.Shutdown(t)

	var uo v1.UploadRequest_Options
	require.NoError(t, uo.FromAWSS3UploadRequestOptions(v1.AWSS3UploadRequestOptions{}))
	var ec2uo v1.UploadRequest_Options
	require.NoError(t, ec2uo.FromAWSUploadRequestOptions(v1.AWSUploadRequestOptions{
		ShareWithAccounts: &[]string{awsAccountId},
	}))
	var auo v1.UploadRequest_Options
	require.NoError(t, auo.FromAzureUploadRequestOptions(v1.AzureUploadRequestOptions{
		ResourceGroup:    "group",
		SubscriptionId:   common.ToPtr("id"),
		TenantId:         common.ToPtr("tenant"),
		ImageName:        common.ToPtr("azure-image"),
		HyperVGeneration: common.ToPtr(v1.V2),
	}))
	var auo2 v1.UploadRequest_Options
	require.NoError(t, auo2.FromAzureUploadRequestOptions(v1.AzureUploadRequestOptions{
		ResourceGroup: "group",
		SourceId:      common.ToPtr("2"),
		ImageName:     common.ToPtr("azure-image"),
	}))

	var openscap v1.OpenSCAP
	require.NoError(t, openscap.FromOpenSCAPProfile(v1.OpenSCAPProfile{
		ProfileId: "test-profile",
	}))
	var openscapTailoring v1.OpenSCAP
	require.NoError(t, openscapTailoring.FromOpenSCAPCompliance(v1.OpenSCAPCompliance{
		PolicyId: policyID,
	}))

	var fileGroup v1.File_Group
	require.NoError(t, fileGroup.FromFileGroup1(v1.FileGroup1(1000)))
	var fileUser v1.File_User
	require.NoError(t, fileUser.FromFileUser1(v1.FileUser1(1000)))
	var composerFileGroup composer.File_Group
	require.NoError(t, composerFileGroup.FromFileGroup1(composer.FileGroup1(1000)))
	var composerFileUser composer.File_User
	require.NoError(t, composerFileUser.FromFileUser1(composer.FileUser1(1000)))

	var dirGroup v1.Directory_Group
	require.NoError(t, dirGroup.FromDirectoryGroup1(v1.DirectoryGroup1(1000)))
	var dirUser v1.Directory_User
	require.NoError(t, dirUser.FromDirectoryUser1(v1.DirectoryUser1(1000)))
	var composerDirGroup composer.Directory_Group
	require.NoError(t, composerDirGroup.FromDirectoryGroup1(v1.DirectoryGroup1(1000)))
	var composerDirUser composer.Directory_User
	require.NoError(t, composerDirUser.FromDirectoryUser1(v1.DirectoryUser1(1000)))

	payloads := []struct {
		imageBuilderRequest         v1.ComposeRequest
		composerRequest             composer.ComposeRequest
		passwordsPresentAndRedacted bool // if False then passwords are redacted thus can't compare for equality
	}{
		// Customizations
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Users: &[]v1.User{
						{
							Name:   "user1",
							SshKey: common.ToPtr("ssh-rsa AAAAB3NzaC1"),
						},
						{
							Name:     "user2",
							SshKey:   common.ToPtr("ssh-rsa AAAAB3NzaC2"),
							Password: common.ToPtr("$6$password123"),
						},
						{
							Name:     "user3",
							Password: common.ToPtr("$6$password123"),
						},
					},
				},
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesRhelEdgeInstaller,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "centos-9",
				Customizations: &composer.Customizations{
					Users: &[]composer.User{
						{
							Name:   "user1",
							Key:    common.ToPtr("ssh-rsa AAAAB3NzaC1"),
							Groups: &[]string{"wheel"},
						},
						{
							Name:     "user2",
							Key:      common.ToPtr("ssh-rsa AAAAB3NzaC2"),
							Password: common.ToPtr("$6$password123"),
							Groups:   &[]string{"wheel"},
						},
						{
							Name:     "user3",
							Password: common.ToPtr("$6$password123"),
							Groups:   &[]string{"wheel"},
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesEdgeInstaller,
					Ostree:       nil,
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: true,
		},
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Packages: &[]string{
						"some",
						"packages",
					},
					PayloadRepositories: &[]v1.Repository{
						{
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
						},
					},
					Filesystem: &[]v1.Filesystem{
						{
							Mountpoint: "/",
							MinSize:    2147483648,
						},
						{
							Mountpoint: "/var",
							MinSize:    1073741824,
						},
					},
					Groups: common.ToPtr([]v1.Group{
						{
							Name: "group",
						},
					}),
					CustomRepositories: &[]v1.CustomRepository{
						{
							Id:       "some-repo-id",
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
					},
					Openscap: &openscap,
					Fips: &v1.FIPS{
						Enabled: common.ToPtr(true),
					},
					Installer: &v1.Installer{
						Unattended:   common.ToPtr(true),
						SudoNopasswd: &[]string{"admin", "%wheel"},
					},
					Cacerts: &v1.CACertsCustomization{
						PemCerts: []string{"---BEGIN CERTIFICATE---\nMIIC0DCCAbigAwIBAgIUI...\n---END CERTIFICATE---"},
					},
				},
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesRhelEdgeInstaller,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "centos-9",
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
					Installer: &composer.Installer{
						Unattended:   common.ToPtr(true),
						SudoNopasswd: &[]string{"admin", "%wheel"},
					},
					Cacerts: &composer.CACertsCustomization{
						PemCerts: []string{"---BEGIN CERTIFICATE---\nMIIC0DCCAbigAwIBAgIUI...\n---END CERTIFICATE---"},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesEdgeInstaller,
					Ostree:       nil,
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// payload & custom repos by ID
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					PayloadRepositories: &[]v1.Repository{
						{
							Id:           common.ToPtr(mocks.RepoPLID),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         false,
						},
						{
							Id: common.ToPtr(mocks.RepoPLID3),
						},
						{
							Id: common.ToPtr(mocks.RepoUplID),
						},
					},
					CustomRepositories: &[]v1.CustomRepository{
						{
							Id:       mocks.RepoPLID,
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
						{
							Id: mocks.RepoPLID3,
						},
						{
							Id:       "non-content-sources",
							Baseurl:  &[]string{"non-content-sources.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
						{
							Id: mocks.RepoUplID,
						},
					},
				},
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "centos-9",
				Customizations: &composer.Customizations{
					PayloadRepositories: &[]composer.Repository{
						{
							Baseurl:      common.ToPtr("https://some-repo-base-url.org"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(true),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							IgnoreSsl:    common.ToPtr(false),
							Rhsm:         common.ToPtr(false),
						},
						{
							Baseurl:      common.ToPtr("https://some-repo-base-url3.org"),
							CheckRepoGpg: common.ToPtr(false),
							Rhsm:         common.ToPtr(false),
						},
						{
							Baseurl:      common.ToPtr("https://content-sources.org"),
							Gpgkey:       common.ToPtr("some-gpg-key"),
							CheckGpg:     common.ToPtr(true),
							CheckRepoGpg: common.ToPtr(false),
							Rhsm:         common.ToPtr(false),
						},
					},
					CustomRepositories: &[]composer.CustomRepository{
						{
							Id:       mocks.RepoPLID,
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
							Name:     common.ToPtr("payload"),
						},
						{
							Id:      mocks.RepoPLID3,
							Baseurl: &[]string{"https://some-repo-base-url3.org"},
							Name:    common.ToPtr("payload3"),
						},
						{
							Id:       "non-content-sources",
							Baseurl:  &[]string{"non-content-sources.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
						{
							Id:       mocks.RepoUplID,
							Name:     common.ToPtr("upload"),
							Baseurl:  &[]string{"https://content-sources.org"},
							Gpgkey:   &[]string{"some-gpg-key"},
							CheckGpg: common.ToPtr(true),
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Ostree:       nil,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Packages: nil,
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesEdgeCommit,
						Ostree: &v1.OSTree{
							Ref: common.ToPtr("edge/ref"),
						},
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
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
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// ostree, ignition, fdo
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Packages: &[]string{"pkg"},
					Subscription: &v1.Subscription{
						Organization: 000,
					},
					Fdo: &v1.FDO{
						DiunPubKeyHash: common.ToPtr("hash"),
					},
					Ignition: &v1.Ignition{
						Embedded: &v1.IgnitionEmbedded{
							Config: "config",
						},
					},
				},
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesRhelEdgeCommit,
						Ostree: &v1.OSTree{
							Ref:        common.ToPtr("test/edge/ref"),
							Url:        common.ToPtr("https://ostree.srv/"),
							Contenturl: common.ToPtr("https://ostree.srv/content"),
							Parent:     common.ToPtr("test/edge/ref2"),
							Rhsm:       common.ToPtr(true),
						},
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "centos-9",
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
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// Test Azure with SubscriptionId and TenantId
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesAzure,
						Ostree: &v1.OSTree{
							Ref:    common.ToPtr("test/edge/ref"),
							Url:    common.ToPtr("https://ostree.srv/"),
							Parent: common.ToPtr("test/edge/ref2"),
						},
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAzure,
							Options: auo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-9",
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
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AzureUploadOptions{
						ImageName:        common.ToPtr("azure-image"),
						ResourceGroup:    "group",
						SubscriptionId:   "id",
						TenantId:         "tenant",
						HyperVGeneration: common.ToPtr(composer.V2),
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// Test Azure with SourceId
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesAzure,
						Ostree: &v1.OSTree{
							Ref:    common.ToPtr("test/edge/ref"),
							Url:    common.ToPtr("https://ostree.srv/"),
							Parent: common.ToPtr("test/edge/ref2"),
						},
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAzure,
							Options: auo2,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-9",
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
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
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
			passwordsPresentAndRedacted: false,
		},
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesAws,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAws,
							Options: ec2uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-9",
				Customizations: nil,
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesAws,
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSEC2UploadOptions{
						ShareWithAccounts: []string{awsAccountId},
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "centos-9",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesAws,
						Size:         common.ToPtr(uint64(13958643712)),
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAws,
							Options: ec2uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution:   "centos-9",
				Customizations: nil,
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesAws,
					Size:         common.ToPtr(uint64(13958643712)),
					Repositories: []composer.Repository{

						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(mocks.CentosGPG),
							CheckGpg:    common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSEC2UploadOptions{
						ShareWithAccounts: []string{awsAccountId},
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// just one partition
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Filesystem: &[]v1.Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
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
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		{
			imageBuilderRequest: v1.ComposeRequest{
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesOci,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesOciObjectstorage,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesOci,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(mocks.RhelGPG),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.OCIUploadOptions{}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// One partition + partition_mode lvm
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Filesystem: &[]v1.Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
					PartitioningMode: common.ToPtr(v1.Lvm),
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
				Customizations: &composer.Customizations{
					Filesystem: &[]composer.Filesystem{
						{
							MinSize:    10 * 1024 * 1024 * 1024,
							Mountpoint: "/",
						},
					},
					PartitioningMode: common.ToPtr(composer.CustomizationsPartitioningModeLvm),
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// files & directories customization
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Files: &[]v1.File{
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
					Directories: &[]v1.Directory{
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
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
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
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// firewall, services, locale, tz, containers, hostname
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Firewall: &v1.FirewallCustomization{
						Ports: common.ToPtr([]string{"1"}),
					},
					Services: &v1.Services{
						Disabled: common.ToPtr([]string{"service"}),
						Masked:   common.ToPtr([]string{"service2"}),
					},
					Locale: &v1.Locale{
						Keyboard: common.ToPtr("piano"),
					},
					Timezone: &v1.Timezone{
						Timezone: common.ToPtr("antarctica"),
					},
					Containers: &[]v1.Container{
						{
							Source: "container.io/test",
						},
					},
					Hostname: common.ToPtr("test-host"),
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
				Customizations: &composer.Customizations{
					Firewall: &composer.FirewallCustomization{
						Ports: common.ToPtr([]string{"1"}),
					},
					Services: &composer.Services{
						Disabled: common.ToPtr([]string{"service"}),
						Masked:   common.ToPtr([]string{"service2"}),
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
					Hostname: common.ToPtr("test-host"),
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// subscriptions, openscap with services customizations
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Subscription: &v1.Subscription{
						Insights: true,
					},
					Openscap: &openscap,
					Services: &v1.Services{
						Enabled: &[]string{"test_service"},
						Masked:  &[]string{"test_service2"},
					},
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
				Customizations: &composer.Customizations{
					Subscription: &composer.Subscription{
						ActivationKey: "",
						BaseUrl:       "",
						Insights:      true,
						Rhc:           common.ToPtr(false),
						Organization:  "0",
						ServerUrl:     "",
					},
					Openscap: &composer.OpenSCAP{
						ProfileId: "test-profile",
					},
					Services: &composer.Services{
						Enabled: &[]string{"test_service", "rhcd"},
						Masked:  &[]string{"test_service2"},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// subscriptions, openscap with no services customizations
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Subscription: &v1.Subscription{
						Insights: true,
					},
					Openscap: &openscap,
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
				Customizations: &composer.Customizations{
					Subscription: &composer.Subscription{
						ActivationKey: "",
						BaseUrl:       "",
						Insights:      true,
						Rhc:           common.ToPtr(false),
						Organization:  "0",
						ServerUrl:     "",
					},
					Openscap: &composer.OpenSCAP{
						ProfileId: "test-profile",
					},
					Services: &composer.Services{
						Enabled: &[]string{"rhcd"},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.AWSS3UploadOptions{
						Region: "",
					}),
				},
			},
			passwordsPresentAndRedacted: false,
		},
		// openscap with tailoring
		{
			imageBuilderRequest: v1.ComposeRequest{
				Customizations: &v1.Customizations{
					Openscap: &openscapTailoring,
				},
				Distribution: "rhel-8",
				ImageRequests: []v1.ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    v1.ImageTypesGuestImage,
						UploadRequest: v1.UploadRequest{
							Type:    v1.UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-8.10",
				Customizations: &composer.Customizations{
					Openscap: &composer.OpenSCAP{
						ProfileId: "openscap-ref-id",
						PolicyId:  &policyID,
						JsonTailoring: &composer.OpenSCAPJSONTailoring{
							ProfileId: "openscap-ref-id",
							Filepath:  "/etc/osbuild/openscap-tailoring.json",
						},
					},
					Directories: &[]composer.Directory{
						{
							Path: "/etc/osbuild",
						},
					},
					Files: &[]composer.File{
						{
							Path: "/etc/osbuild/openscap-tailoring.json",
							Data: common.ToPtr("{ \"data\": \"some-tailoring-data\"}"),
						},
					},
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(mocks.RhelGPG),
							CheckGpg: common.ToPtr(true),
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
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload.imageBuilderRequest)
		require.Equal(t, http.StatusCreated, respStatusCode)

		var result v1.ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, id, result.Id)

		if !payload.passwordsPresentAndRedacted {
			//compare expected compose request with actual receieved compose request
			require.Equal(t, payload.composerRequest, composerRequest)
		} else {
			require.Equal(t, payload.composerRequest.Distribution, composerRequest.Distribution)
			require.Equal(t, payload.composerRequest.ImageRequest, composerRequest.ImageRequest)

			// Check that the password returned is redacted
			for _, u := range *composerRequest.Customizations.Users {
				require.True(t, u.Password == nil)
			}
		}
		composerRequest = composer.ComposeRequest{}
	}
}
