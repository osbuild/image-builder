package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/clients/content_sources"
	"github.com/osbuild/image-builder/internal/clients/provisioning"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/tutils"
)

const (
	centosGpg = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFzMWxkBEADHrskpBgN9OphmhRkc7P/YrsAGSvvl7kfu+e9KAaU6f5MeAVyn\nrIoM43syyGkgFyWgjZM8/rur7EMPY2yt+2q/1ZfLVCRn9856JqTIq0XRpDUe4nKQ\n8BlA7wDVZoSDxUZkSuTIyExbDf0cpw89Tcf62Mxmi8jh74vRlPy1PgjWL5494b3X\n5fxDidH4bqPZyxTBqPrUFuo+EfUVEqiGF94Ppq6ZUvrBGOVo1V1+Ifm9CGEK597c\naevcGc1RFlgxIgN84UpuDjPR9/zSndwJ7XsXYvZ6HXcKGagRKsfYDWGPkA5cOL/e\nf+yObOnC43yPUvpggQ4KaNJ6+SMTZOKikM8yciyBwLqwrjo8FlJgkv8Vfag/2UR7\nJINbyqHHoLUhQ2m6HXSwK4YjtwidF9EUkaBZWrrskYR3IRZLXlWqeOi/+ezYOW0m\nvufrkcvsh+TKlVVnuwmEPjJ8mwUSpsLdfPJo1DHsd8FS03SCKPaXFdD7ePfEjiYk\nnHpQaKE01aWVSLUiygn7F7rYemGqV9Vt7tBw5pz0vqSC72a5E3zFzIIuHx6aANry\nGat3aqU3qtBXOrA/dPkX9cWE+UR5wo/A2UdKJZLlGhM2WRJ3ltmGT48V9CeS6N9Y\nm4CKdzvg7EWjlTlFrd/8WJ2KoqOE9leDPeXRPncubJfJ6LLIHyG09h9kKQARAQAB\ntDpDZW50T1MgKENlbnRPUyBPZmZpY2lhbCBTaWduaW5nIEtleSkgPHNlY3VyaXR5\nQGNlbnRvcy5vcmc+iQI3BBMBAgAhBQJczFsZAhsDBgsJCAcDAgYVCAIJCgsDFgIB\nAh4BAheAAAoJEAW1VbOEg8ZdjOsP/2ygSxH9jqffOU9SKyJDlraL2gIutqZ3B8pl\nGy/Qnb9QD1EJVb4ZxOEhcY2W9VJfIpnf3yBuAto7zvKe/G1nxH4Bt6WTJQCkUjcs\nN3qPWsx1VslsAEz7bXGiHym6Ay4xF28bQ9XYIokIQXd0T2rD3/lNGxNtORZ2bKjD\nvOzYzvh2idUIY1DgGWJ11gtHFIA9CvHcW+SMPEhkcKZJAO51ayFBqTSSpiorVwTq\na0cB+cgmCQOI4/MY+kIvzoexfG7xhkUqe0wxmph9RQQxlTbNQDCdaxSgwbF2T+gw\nbyaDvkS4xtR6Soj7BKjKAmcnf5fn4C5Or0KLUqMzBtDMbfQQihn62iZJN6ZZ/4dg\nq4HTqyVpyuzMXsFpJ9L/FqH2DJ4exGGpBv00ba/Zauy7GsqOc5PnNBsYaHCply0X\n407DRx51t9YwYI/ttValuehq9+gRJpOTTKp6AjZn/a5Yt3h6jDgpNfM/EyLFIY9z\nV6CXqQQ/8JRvaik/JsGCf+eeLZOw4koIjZGEAg04iuyNTjhx0e/QHEVcYAqNLhXG\nrCTTbCn3NSUO9qxEXC+K/1m1kaXoCGA0UWlVGZ1JSifbbMx0yxq/brpEZPUYm+32\no8XfbocBWljFUJ+6aljTvZ3LQLKTSPW7TFO+GXycAOmCGhlXh2tlc6iTc41PACqy\nyy+mHmSv\n=kkH7\n-----END PGP PUBLIC KEY BLOCK-----\n"
	rhelGpg   = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBErgSTsBEACh2A4b0O9t+vzC9VrVtL1AKvUWi9OPCjkvR7Xd8DtJxeeMZ5eF\n0HtzIG58qDRybwUe89FZprB1ffuUKzdE+HcL3FbNWSSOXVjZIersdXyH3NvnLLLF\n0DNRB2ix3bXG9Rh/RXpFsNxDp2CEMdUvbYCzE79K1EnUTVh1L0Of023FtPSZXX0c\nu7Pb5DI5lX5YeoXO6RoodrIGYJsVBQWnrWw4xNTconUfNPk0EGZtEnzvH2zyPoJh\nXGF+Ncu9XwbalnYde10OCvSWAZ5zTCpoLMTvQjWpbCdWXJzCm6G+/hx9upke546H\n5IjtYm4dTIVTnc3wvDiODgBKRzOl9rEOCIgOuGtDxRxcQkjrC+xvg5Vkqn7vBUyW\n9pHedOU+PoF3DGOM+dqv+eNKBvh9YF9ugFAQBkcG7viZgvGEMGGUpzNgN7XnS1gj\n/DPo9mZESOYnKceve2tIC87p2hqjrxOHuI7fkZYeNIcAoa83rBltFXaBDYhWAKS1\nPcXS1/7JzP0ky7d0L6Xbu/If5kqWQpKwUInXtySRkuraVfuK3Bpa+X1XecWi24JY\nHVtlNX025xx1ewVzGNCTlWn1skQN2OOoQTV4C8/qFpTW6DTWYurd4+fE0OJFJZQF\nbuhfXYwmRlVOgN5i77NTIJZJQfYFj38c/Iv5vZBPokO6mffrOTv3MHWVgQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChyZWxlYXNlIGtleSAyKSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjYEEwECACAFAkrgSTsCGwMGCwkIBwMCBBUCCAMEFgIDAQIeAQIXgAAK\nCRAZni+R/UMdUWzpD/9s5SFR/ZF3yjY5VLUFLMXIKUztNN3oc45fyLdTI3+UClKC\n2tEruzYjqNHhqAEXa2sN1fMrsuKec61Ll2NfvJjkLKDvgVIh7kM7aslNYVOP6BTf\nC/JJ7/ufz3UZmyViH/WDl+AYdgk3JqCIO5w5ryrC9IyBzYv2m0HqYbWfphY3uHw5\nun3ndLJcu8+BGP5F+ONQEGl+DRH58Il9Jp3HwbRa7dvkPgEhfFR+1hI+Btta2C7E\n0/2NKzCxZw7Lx3PBRcU92YKyaEihfy/aQKZCAuyfKiMvsmzs+4poIX7I9NQCJpyE\nIGfINoZ7VxqHwRn/d5mw2MZTJjbzSf+Um9YJyA0iEEyD6qjriWQRbuxpQXmlAJbh\n8okZ4gbVFv1F8MzK+4R8VvWJ0XxgtikSo72fHjwha7MAjqFnOq6eo6fEC/75g3NL\nGht5VdpGuHk0vbdENHMC8wS99e5qXGNDued3hlTavDMlEAHl34q2H9nakTGRF5Ki\nJUfNh3DVRGhg8cMIti21njiRh7gyFI2OccATY7bBSr79JhuNwelHuxLrCFpY7V25\nOFktl15jZJaMxuQBqYdBgSay2G0U6D1+7VsWufpzd/Abx1/c3oi9ZaJvW22kAggq\ndzdA27UUYjWvx42w9menJwh/0jeQcTecIUd0d0rFcw/c1pvgMMl/Q73yzKgKYw==\n=zbHE\n-----END PGP PUBLIC KEY BLOCK-----\n-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFsy23UBEACUKSphFEIEvNpy68VeW4Dt6qv+mU6am9a2AAl10JANLj1oqWX+\noYk3en1S6cVe2qehSL5DGVa3HMUZkP3dtbD4SgzXzxPodebPcr4+0QNWigkUisri\nXGL5SCEcOP30zDhZvg+4mpO2jMi7Kc1DLPzBBkgppcX91wa0L1pQzBcvYMPyV/Dh\nKbQHR75WdkP6OA2JXdfC94nxYq+2e0iPqC1hCP3Elh+YnSkOkrawDPmoB1g4+ft/\nxsiVGVy/W0ekXmgvYEHt6si6Y8NwXgnTMqxeSXQ9YUgVIbTpsxHQKGy76T5lMlWX\n4LCOmEVomBJg1SqF6yi9Vu8TeNThaDqT4/DddYInd0OO69s0kGIXalVgGYiW2HOD\nx2q5R1VGCoJxXomz+EbOXY+HpKPOHAjU0DB9MxbU3S248LQ69nIB5uxysy0PSco1\nsdZ8sxRNQ9Dw6on0Nowx5m6Thefzs5iK3dnPGBqHTT43DHbnWc2scjQFG+eZhe98\nEll/kb6vpBoY4bG9/wCG9qu7jj9Z+BceCNKeHllbezVLCU/Hswivr7h2dnaEFvPD\nO4GqiWiwOF06XaBMVgxA8p2HRw0KtXqOpZk+o+sUvdPjsBw42BB96A1yFX4jgFNA\nPyZYnEUdP6OOv9HSjnl7k/iEkvHq/jGYMMojixlvXpGXhnt5jNyc4GSUJQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChhdXhpbGlhcnkga2V5KSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjkEEwECACMFAlsy23UCGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIX\ngAAKCRD3b2bD1AgnknqOD/9fB2ASuG2aJIiap4kK58R+RmOVM4qgclAnaG57+vjI\nnKvyfV3NH/keplGNRxwqHekfPCqvkpABwhdGEXIE8ILqnPewIMr6PZNZWNJynZ9i\neSMzVuCG7jDoGyQ5/6B0f6xeBtTeBDiRl7+Alehet1twuGL1BJUYG0QuLgcEzkaE\n/gkuumeVcazLzz7L12D22nMk66GxmgXfqS5zcbqOAuZwaA6VgSEgFdV2X2JU79zS\nBQJXv7NKc+nDXFG7M7EHjY3Rma3HXkDbkT8bzh9tJV7Z7TlpT829pStWQyoxKCVq\nsEX8WsSapTKA3P9YkYCwLShgZu4HKRFvHMaIasSIZWzLu+RZH/4yyHOhj0QB7XMY\neHQ6fGSbtJ+K6SrpHOOsKQNAJ0hVbSrnA1cr5+2SDfel1RfYt0W9FA6DoH/S5gAR\ndzT1u44QVwwp3U+eFpHphFy//uzxNMtCjjdkpzhYYhOCLNkDrlRPb+bcoL/6ePSr\n016PA7eEnuC305YU1Ml2WcCn7wQV8x90o33klJmEkWtXh3X39vYtI4nCPIvZn1eP\nVy+F+wWt4vN2b8oOdlzc2paOembbCo2B+Wapv5Y9peBvlbsDSgqtJABfK8KQq/jK\nYl3h5elIa1I3uNfczeHOnf1enLOUOlq630yeM/yHizz99G1g+z/guMh5+x/OHraW\niA==\n=+Gxh\n-----END PGP PUBLIC KEY BLOCK-----\n"
)

func TestValidateComposeRequest(t *testing.T) {
	// note: any url will work, it'll only try to contact the osbuild-composer
	// instance when calling /compose or /compose/$uuid
	srv, tokenSrv := startServer(t, &testServerClientsConf{}, nil)
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	t.Run("ErrorsForZeroImageRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests:  []ImageRequest{},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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
			Distribution:   "centos-9",
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
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Contains(t, body, `Error at \"/image_requests\": maximum number of items is 1`)
	})

	t.Run("ErrorsForEmptyAccountsAndSources", func(t *testing.T) {
		var uo UploadRequest_Options
		require.NoError(t, uo.FromAWSUploadRequestOptions(AWSUploadRequestOptions{}))

		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
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
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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
				Distribution:   "centos-9",
				ImageRequests:  []ImageRequest{tc.request},
			}
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, "Request must contain either (1) a source id, and no tenant or subscription ids or (2) tenant and subscription ids, and no source id.")
		})
	}

	t.Run("ErrorsForZeroUploadRequests", func(t *testing.T) {
		payload := ComposeRequest{
			Customizations: nil,
			Distribution:   "centos-9",
			ImageRequests: []ImageRequest{
				{
					Architecture:  "x86_64",
					ImageType:     ImageTypesAzure,
					UploadRequest: UploadRequest{},
				},
			},
		}
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusBadRequest, respStatusCode)
		require.Regexp(t, "image_requests/0/upload_request/options|image_requests/0/upload_request/type", body)
		require.Regexp(t, "Value is not nullable|value is not one of the allowed values|doesn't match any schema from", body)
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
			Distribution:   "centos-9",
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
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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
			Distribution:   "centos-9",
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
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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
			Distribution: "centos-9",
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
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total AWS image size cannot exceed %d bytes", FSMaxSize))
		}

		for _, it := range []ImageTypes{ImageTypesAzure, ImageTypesVhd} {
			payload.ImageRequests[0].ImageType = it
			payload.ImageRequests[0].UploadRequest = azureUr
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
			require.Equal(t, http.StatusBadRequest, respStatusCode)
			require.Contains(t, body, fmt.Sprintf("Total Azure image size cannot exceed %d bytes", FSMaxSize))
		}
	})

	t.Run("ValidateFSSizes", func(t *testing.T) {
		buildComposeRequest := func(fsSize *uint64, imgSize *uint64, imgType ImageTypes) *ComposeRequest {
			cr := &ComposeRequest{
				Distribution: "centos-9",
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
	ctx := context.Background()
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
	err = dbase.InsertCompose(ctx, id, "600000", "user@test.test", "000001", &imageName, json.RawMessage("{}"), &clientId, nil)
	require.NoError(t, err)

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s",
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

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
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
		Distribution:   "centos-9",
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
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
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
		Distribution: "centos-9",
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
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
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
		Distribution:   "centos-9",
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
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusInternalServerError, respStatusCode)
	require.Contains(t, body, "Internal Server Error")
}

// This test case queries the image-builder for a non existing type of the os distribution
// osbuild-composer is not being mock here as the error should be intercepted by image-builder
func TestComposeImageErrorsWhenDistributionNotExists(t *testing.T) {
	srv, tokenSrv := startServer(t, &testServerClientsConf{}, nil)
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
		Distribution:   "fedoros",
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
	respStatusCode, _ := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
	require.Equal(t, http.StatusBadRequest, respStatusCode)
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

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, nil)
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
		Distribution:   "centos-9",
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
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
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

		srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        allowFile,
		})
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("centos-9")

		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusCreated, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, id, result.Id)
	})

	t.Run("restricted distribution, forbidden", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        allowFile,
		})
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("rhel-8")

		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil, result.Id)
	})

	t.Run("restricted distribution, forbidden (no allowFile)", func(t *testing.T) {
		apiSrv := createApiSrv()
		defer apiSrv.Close()

		srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &ServerConfig{
			DistributionsDir: distsDir,
			AllowFile:        "",
		})
		defer func() {
			err := srv.Shutdown(context.Background())
			require.NoError(t, err)
		}()
		defer tokenSrv.Close()

		payload := createPayload("rhel-8")

		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload)
		require.Equal(t, http.StatusForbidden, respStatusCode)

		var result ComposeResponse
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, uuid.Nil, result.Id)
	})
}

func TestComposeWithSnapshots(t *testing.T) {
	var composeId uuid.UUID
	var composerRequest composer.ComposeRequest
	repoBaseId := uuid.New()
	repoAppstrId := uuid.New()
	repoPayloadId := uuid.New()
	repoPayloadId2 := uuid.New()
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
		composeId = uuid.New()
		result := composer.ComposeId{
			Id: composeId,
		}
		err = json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()
	csSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, tutils.AuthString0, r.Header.Get("x-rh-identity"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/repositories/" {
			urlForm := r.URL.Query().Get("url")
			urls := strings.Split(urlForm, ",")
			if slices.Equal(urls, []string{
				"https://cdn.redhat.com/content/dist/rhel9/9/x86_64/baseos/os",
				"https://cdn.redhat.com/content/dist/rhel9/9/x86_64/appstream/os",
			}) {
				require.Equal(t, "red_hat", r.URL.Query().Get("origin"))
				result := content_sources.ApiRepositoryCollectionResponse{
					Data: &[]content_sources.ApiRepositoryResponse{
						{
							GpgKey: common.ToPtr(rhelGpg),
							Uuid:   common.ToPtr(repoBaseId.String()),
						},
						{
							GpgKey: common.ToPtr(rhelGpg),
							Uuid:   common.ToPtr(repoAppstrId.String()),
						},
					},
				}
				err := json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			} else if slices.Equal(urls, []string{"https://some-repo-base-url.org"}) {
				require.Equal(t, "external", r.URL.Query().Get("origin"))
				result := content_sources.ApiRepositoryCollectionResponse{
					Data: &[]content_sources.ApiRepositoryResponse{
						{
							GpgKey: common.ToPtr("some-gpg-key"),
							Uuid:   common.ToPtr(repoPayloadId.String()),
						},
					},
				}
				err := json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			} else if slices.Equal(urls, []string{
				"https://some-repo-base-url.org",
				"https://some-repo-base-url2.org",
			}) {
				require.Equal(t, "external", r.URL.Query().Get("origin"))
				result := content_sources.ApiRepositoryCollectionResponse{
					Data: &[]content_sources.ApiRepositoryResponse{
						{
							GpgKey: common.ToPtr("some-gpg-key"),
							Uuid:   common.ToPtr(repoPayloadId.String()),
						},
						{
							Uuid: common.ToPtr(repoPayloadId2.String()),
						},
					},
				}
				err := json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			}
		}

		if r.URL.Path == "/snapshots/for_date/" {
			require.Equal(t, "application/json", r.Header.Get("content-type"))
			var body content_sources.ApiListSnapshotByDateRequest
			err := json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			require.Equal(t, "1999-01-30T00:00:00Z", *body.Date)

			if slices.Equal(*body.RepositoryUuids, []string{
				repoBaseId.String(),
				repoAppstrId.String(),
			}) {
				result := content_sources.ApiListSnapshotByDateResponse{
					Data: &[]content_sources.ApiSnapshotForDate{
						{
							IsAfter: common.ToPtr(false),
							Match: &content_sources.ApiSnapshotResponse{
								CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
								RepositoryPath: common.ToPtr("/snappy/baseos"),
							},
							RepositoryUuid: common.ToPtr(repoBaseId.String()),
						},
						{
							IsAfter: common.ToPtr(false),
							Match: &content_sources.ApiSnapshotResponse{
								CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
								RepositoryPath: common.ToPtr("/snappy/appstream"),
							},
							RepositoryUuid: common.ToPtr(repoAppstrId.String()),
						},
					},
				}
				err = json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			} else if slices.Equal(*body.RepositoryUuids, []string{repoPayloadId.String()}) {
				result := content_sources.ApiListSnapshotByDateResponse{
					Data: &[]content_sources.ApiSnapshotForDate{
						{
							IsAfter: common.ToPtr(false),
							Match: &content_sources.ApiSnapshotResponse{
								CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
								RepositoryPath: common.ToPtr("/snappy/payload"),
							},
							RepositoryUuid: common.ToPtr(repoPayloadId.String()),
						},
					},
				}
				err = json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			} else if slices.Equal(*body.RepositoryUuids, []string{
				repoPayloadId.String(),
				repoPayloadId2.String(),
			}) {
				result := content_sources.ApiListSnapshotByDateResponse{
					Data: &[]content_sources.ApiSnapshotForDate{
						{
							IsAfter: common.ToPtr(false),
							Match: &content_sources.ApiSnapshotResponse{
								CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
								RepositoryPath: common.ToPtr("/snappy/payload"),
							},
							RepositoryUuid: common.ToPtr(repoPayloadId.String()),
						},
						{
							IsAfter: common.ToPtr(false),
							Match: &content_sources.ApiSnapshotResponse{
								CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
								RepositoryPath: common.ToPtr("/snappy/payload2"),
							},
							RepositoryUuid: common.ToPtr(repoPayloadId2.String()),
						},
					},
				}
				err = json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			}
		}
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL, CSURL: csSrv.URL}, &ServerConfig{
		CSReposURL: "https://content-sources.org",
	})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var uo UploadRequest_Options
	require.NoError(t, uo.FromAWSS3UploadRequestOptions(AWSS3UploadRequestOptions{}))
	payloads := []struct {
		imageBuilderRequest ComposeRequest
		composerRequest     composer.ComposeRequest
	}{
		// basic without payload or custom repositories
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "rhel-94",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30"),
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-94",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(rhelGpg),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(rhelGpg),
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
			imageBuilderRequest: ComposeRequest{
				Distribution: "rhel-94",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30"),
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
				Customizations: &Customizations{
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
					CustomRepositories: &[]CustomRepository{
						{
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							CheckGpg: common.ToPtr(true),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       repoPayloadId.String(),
						},
						{
							Baseurl: &[]string{"https://some-repo-base-url2.org"},
							Id:      repoPayloadId2.String(),
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-94",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(rhelGpg),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(rhelGpg),
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
							Baseurl:  &[]string{"https://content-sources.org/snappy/payload"},
							CheckGpg: common.ToPtr(true),
							Enabled:  common.ToPtr(false),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       repoPayloadId.String(),
						},
						{
							Baseurl: &[]string{"https://content-sources.org/snappy/payload2"},
							Enabled: common.ToPtr(false),
							Id:      repoPayloadId2.String(),
						},
					},
				},
			},
		},
		// 2 payload 1 custom repository
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "rhel-94",
				ImageRequests: []ImageRequest{
					{
						Architecture: "x86_64",
						ImageType:    ImageTypesGuestImage,
						SnapshotDate: common.ToPtr("1999-01-30"),
						UploadRequest: UploadRequest{
							Type:    UploadTypesAwsS3,
							Options: uo,
						},
					},
				},
				Customizations: &Customizations{
					PayloadRepositories: &[]Repository{
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
					CustomRepositories: &[]CustomRepository{
						{
							Baseurl:  &[]string{"https://some-repo-base-url.org"},
							CheckGpg: common.ToPtr(true),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       repoPayloadId.String(),
						},
					},
				},
			},
			composerRequest: composer.ComposeRequest{
				Distribution: "rhel-94",
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/baseos"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(rhelGpg),
							CheckGpg:    common.ToPtr(true),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
						},
						{
							Baseurl:     common.ToPtr("https://content-sources.org/snappy/appstream"),
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(rhelGpg),
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
							Baseurl:  &[]string{"https://content-sources.org/snappy/payload"},
							CheckGpg: common.ToPtr(true),
							Enabled:  common.ToPtr(false),
							Gpgkey:   &[]string{"some-gpg-key"},
							Id:       repoPayloadId.String(),
						},
					},
				},
			},
		},
	}

	for idx, payload := range payloads {
		fmt.Printf("TT payload %d\n", idx)
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload.imageBuilderRequest)
		require.Equal(t, http.StatusCreated, respStatusCode)
		var result ComposeResponse
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

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL, ProvURL: provSrv.URL}, nil)
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
		imageBuilderRequest         ComposeRequest
		composerRequest             composer.ComposeRequest
		passwordsPresentAndRedacted bool // if False then passwords are redacted thus can't compare for equality
	}{
		// Customizations
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Users: &[]User{
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
					Installer: &Installer{
						Unattended:   common.ToPtr(true),
						SudoNopasswd: &[]string{"admin", "%wheel"},
					},
				},
				Distribution: "centos-9",
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
							Gpgkey:      common.ToPtr(rhelGpg),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(rhelGpg),
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
				Distribution: "centos-9",
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-9",
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
		// Test Azure with SourceId
		{
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-9",
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-9",
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
			imageBuilderRequest: ComposeRequest{
				Distribution: "centos-9",
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
							Gpgkey:      common.ToPtr(centosGpg),
							CheckGpg:    common.ToPtr(true),
						},
						{
							Baseurl:     common.ToPtr("http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(false),
							Gpgkey:      common.ToPtr(centosGpg),
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
							Gpgkey:      common.ToPtr(rhelGpg),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(rhelGpg),
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
							Gpgkey:      common.ToPtr(rhelGpg),
						},
						{
							Baseurl:     common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							IgnoreSsl:   nil,
							Metalink:    nil,
							Mirrorlist:  nil,
							PackageSets: nil,
							Rhsm:        common.ToPtr(true),
							CheckGpg:    common.ToPtr(true),
							Gpgkey:      common.ToPtr(rhelGpg),
						},
					},
					UploadOptions: makeUploadOptions(t, composer.OCIUploadOptions{}),
				},
			},
			passwordsPresentAndRedacted: false,
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
							Gpgkey:   common.ToPtr(rhelGpg),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(rhelGpg),
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
							Gpgkey:   common.ToPtr(rhelGpg),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(rhelGpg),
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
		// firewall, services, locale, tz, containers
		{
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Firewall: &FirewallCustomization{
						Ports: common.ToPtr([]string{"1"}),
					},
					Services: &Services{
						Disabled: common.ToPtr([]string{"service"}),
						Masked:   common.ToPtr([]string{"service2"}),
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
				},
				ImageRequest: &composer.ImageRequest{
					Architecture: "x86_64",
					ImageType:    composer.ImageTypesGuestImage,
					Repositories: []composer.Repository{
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(rhelGpg),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(rhelGpg),
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
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Subscription: &Subscription{
						Insights: true,
					},
					Openscap: &OpenSCAP{
						ProfileId: "test",
					},
					Services: &Services{
						Enabled: &[]string{"test_service"},
						Masked:  &[]string{"test_service2"},
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
						ProfileId: "test",
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
							Gpgkey:   common.ToPtr(rhelGpg),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(rhelGpg),
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
			imageBuilderRequest: ComposeRequest{
				Customizations: &Customizations{
					Subscription: &Subscription{
						Insights: true,
					},
					Openscap: &OpenSCAP{
						ProfileId: "test",
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
						ProfileId: "test",
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
							Gpgkey:   common.ToPtr(rhelGpg),
							CheckGpg: common.ToPtr(true),
						},
						{
							Baseurl:  common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"),
							Rhsm:     common.ToPtr(true),
							Gpgkey:   common.ToPtr(rhelGpg),
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
	}

	for idx, payload := range payloads {
		fmt.Printf("TT payload %d\n", idx)
		respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/compose", payload.imageBuilderRequest)
		require.Equal(t, http.StatusCreated, respStatusCode)

		var result ComposeResponse
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
				require.True(t, u.IsRedacted())
			}
		}
		composerRequest = composer.ComposeRequest{}
	}
}
