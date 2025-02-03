package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/tutils"
	v1 "github.com/osbuild/image-builder/internal/v1"
)

func TestComposeStatus(t *testing.T) {
	ctx := context.Background()
	composeId := uuid.New()
	var composerStatus composer.ComposeStatus
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		// TODO make sure compose id matches
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
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

	var awsUS composer.UploadStatus_Options
	require.NoError(t, awsUS.FromAWSEC2UploadStatus(composer.AWSEC2UploadStatus{
		Ami:    "ami-fakeami",
		Region: "us-east-1",
	}))
	var ibAwsUS v1.UploadStatus_Options
	require.NoError(t, ibAwsUS.FromAWSUploadStatus(v1.AWSUploadStatus{
		Ami:    "ami-fakeami",
		Region: "us-east-1",
	}))

	var awsS3US composer.UploadStatus_Options
	require.NoError(t, awsS3US.FromAWSS3UploadStatus(composer.AWSS3UploadStatus{
		Url: "url",
	}))
	var ibAwsS3US v1.UploadStatus_Options
	require.NoError(t, ibAwsS3US.FromAWSS3UploadStatus(v1.AWSS3UploadStatus{
		Url: "url",
	}))

	var azureUS composer.UploadStatus_Options
	require.NoError(t, azureUS.FromAzureUploadStatus(composer.AzureUploadStatus{
		ImageName: "image-name",
	}))
	var ibAzureUS v1.UploadStatus_Options
	require.NoError(t, ibAzureUS.FromAzureUploadStatus(v1.AzureUploadStatus{
		ImageName: "image-name",
	}))

	var gcpUS composer.UploadStatus_Options
	require.NoError(t, gcpUS.FromGCPUploadStatus(composer.GCPUploadStatus{
		ImageName: "image-name",
		ProjectId: "project-id",
	}))
	var ibGcpUS v1.UploadStatus_Options
	require.NoError(t, ibGcpUS.FromGCPUploadStatus(v1.GCPUploadStatus{
		ImageName: "image-name",
		ProjectId: "project-id",
	}))

	var ociUS composer.UploadStatus_Options
	require.NoError(t, ociUS.FromOCIUploadStatus(composer.OCIUploadStatus{
		Url: "url",
	}))
	var ibOciUS v1.UploadStatus_Options
	require.NoError(t, ibOciUS.FromOCIUploadStatus(v1.OCIUploadStatus{
		Url: "url",
	}))

	payloads := []struct {
		composerStatus composer.ComposeStatus
		imageStatus    v1.ImageStatus
	}{
		{
			composerStatus: composer.ComposeStatus{
				ImageStatus: composer.ImageStatus{
					Status: composer.ImageStatusValueSuccess,
					UploadStatus: &composer.UploadStatus{
						Status:  composer.UploadStatusValue("success"),
						Type:    composer.UploadTypesAws,
						Options: awsUS,
					},
				},
				Status: composer.ComposeStatusValueSuccess,
			},
			imageStatus: v1.ImageStatus{
				Status: v1.ImageStatusStatusSuccess,
				UploadStatus: &v1.UploadStatus{
					Status:  v1.Success,
					Type:    v1.UploadTypesAws,
					Options: ibAwsUS,
				},
			},
		},
		{
			composerStatus: composer.ComposeStatus{
				ImageStatus: composer.ImageStatus{
					Status: composer.ImageStatusValueSuccess,
					UploadStatus: &composer.UploadStatus{
						Status:  composer.UploadStatusValue("success"),
						Type:    composer.UploadTypesAwsS3,
						Options: awsS3US,
					},
				},
				Status: composer.ComposeStatusValueSuccess,
			},
			imageStatus: v1.ImageStatus{
				Status: v1.ImageStatusStatusSuccess,
				UploadStatus: &v1.UploadStatus{
					Status:  v1.Success,
					Type:    v1.UploadTypesAwsS3,
					Options: ibAwsS3US,
				},
			},
		},
		{
			composerStatus: composer.ComposeStatus{
				ImageStatus: composer.ImageStatus{
					Status: composer.ImageStatusValueSuccess,
					UploadStatus: &composer.UploadStatus{
						Status:  composer.UploadStatusValue("success"),
						Type:    composer.UploadTypesAzure,
						Options: azureUS,
					},
				},
				Status: composer.ComposeStatusValueSuccess,
			},
			imageStatus: v1.ImageStatus{
				Status: v1.ImageStatusStatusSuccess,
				UploadStatus: &v1.UploadStatus{
					Status:  v1.Success,
					Type:    v1.UploadTypesAzure,
					Options: ibAzureUS,
				},
			},
		},
		{
			composerStatus: composer.ComposeStatus{
				ImageStatus: composer.ImageStatus{
					Status: composer.ImageStatusValueSuccess,
					UploadStatus: &composer.UploadStatus{
						Status:  composer.UploadStatusValue("success"),
						Type:    composer.UploadTypesGcp,
						Options: gcpUS,
					},
				},
				Status: composer.ComposeStatusValueSuccess,
			},
			imageStatus: v1.ImageStatus{
				Status: v1.ImageStatusStatusSuccess,
				UploadStatus: &v1.UploadStatus{
					Status:  v1.Success,
					Type:    v1.UploadTypesGcp,
					Options: ibGcpUS,
				},
			},
		},
		{
			composerStatus: composer.ComposeStatus{
				ImageStatus: composer.ImageStatus{
					Status: composer.ImageStatusValueSuccess,
					UploadStatus: &composer.UploadStatus{
						Status:  composer.UploadStatusValue("success"),
						Type:    composer.UploadTypesOciObjectstorage,
						Options: ociUS,
					},
				},
				Status: composer.ComposeStatusValueSuccess,
			},
			imageStatus: v1.ImageStatus{
				Status: v1.ImageStatusStatusSuccess,
				UploadStatus: &v1.UploadStatus{
					Status:  v1.Success,
					Type:    v1.UploadTypesOciObjectstorage,
					Options: ibOciUS,
				},
			},
		},
	}

	for idx, payload := range payloads {
		fmt.Printf("TT payload %d\n", idx)
		composerStatus = payload.composerStatus
		respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/composes/%s", composeId), &tutils.AuthString0)
		require.Equal(t, http.StatusOK, respStatusCode)

		var result v1.ComposeStatus
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, payload.imageStatus, result.ImageStatus)
		require.Equal(t, cr.Distribution, result.Request.Distribution)
		require.Equal(t, cr.Distribution, result.Request.Distribution)
		user := (*result.Request.Customizations.Users)[0]
		require.Nil(t, user.Password)
	}
}
