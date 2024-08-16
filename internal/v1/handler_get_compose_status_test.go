package v1

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
)

func TestComposeStatus(t *testing.T) {
	ctx := context.Background()
	composeId := uuid.New()
	var composerStatus composer.ComposeStatus
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
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

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	cr := ComposeRequest{
		Distribution: "rhel-9",
		Customizations: &Customizations{
			// Since we are not calling handleCommonCompose but inserting directly to DB
			// there is no point in using plaintext passwords
			// If there is plaintext password in DB there is problem elsewhere (eg. CreateBlueprint)
			Users: &[]User{
				{
					Name:     "user",
					SshKey:   common.ToPtr("ssh-rsa AAAAB3NzaC2"),
					Password: common.ToPtr("$6$password123"),
				},
			},
		}}

	crRaw, err := json.Marshal(cr)
	require.NoError(t, err)
	err = dbase.InsertCompose(ctx, composeId, "000000", "user000000@test.test", "000000", cr.ImageName, crRaw, (*string)(cr.ClientId), nil)
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

	var awsUS composer.UploadStatus_Options
	require.NoError(t, awsUS.FromAWSEC2UploadStatus(composer.AWSEC2UploadStatus{
		Ami:    "ami-fakeami",
		Region: "us-east-1",
	}))
	var ibAwsUS UploadStatus_Options
	require.NoError(t, ibAwsUS.FromAWSUploadStatus(AWSUploadStatus{
		Ami:    "ami-fakeami",
		Region: "us-east-1",
	}))

	var awsS3US composer.UploadStatus_Options
	require.NoError(t, awsS3US.FromAWSS3UploadStatus(composer.AWSS3UploadStatus{
		Url: "url",
	}))
	var ibAwsS3US UploadStatus_Options
	require.NoError(t, ibAwsS3US.FromAWSS3UploadStatus(AWSS3UploadStatus{
		Url: "url",
	}))

	var azureUS composer.UploadStatus_Options
	require.NoError(t, azureUS.FromAzureUploadStatus(composer.AzureUploadStatus{
		ImageName: "image-name",
	}))
	var ibAzureUS UploadStatus_Options
	require.NoError(t, ibAzureUS.FromAzureUploadStatus(AzureUploadStatus{
		ImageName: "image-name",
	}))

	var gcpUS composer.UploadStatus_Options
	require.NoError(t, gcpUS.FromGCPUploadStatus(composer.GCPUploadStatus{
		ImageName: "image-name",
		ProjectId: "project-id",
	}))
	var ibGcpUS UploadStatus_Options
	require.NoError(t, ibGcpUS.FromGCPUploadStatus(GCPUploadStatus{
		ImageName: "image-name",
		ProjectId: "project-id",
	}))

	var ociUS composer.UploadStatus_Options
	require.NoError(t, ociUS.FromOCIUploadStatus(composer.OCIUploadStatus{
		Url: "url",
	}))
	var ibOciUS UploadStatus_Options
	require.NoError(t, ibOciUS.FromOCIUploadStatus(OCIUploadStatus{
		Url: "url",
	}))

	payloads := []struct {
		composerStatus composer.ComposeStatus
		imageStatus    ImageStatus
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
			imageStatus: ImageStatus{
				Status: ImageStatusStatusSuccess,
				UploadStatus: &UploadStatus{
					Status:  Success,
					Type:    UploadTypesAws,
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
			imageStatus: ImageStatus{
				Status: ImageStatusStatusSuccess,
				UploadStatus: &UploadStatus{
					Status:  Success,
					Type:    UploadTypesAwsS3,
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
			imageStatus: ImageStatus{
				Status: ImageStatusStatusSuccess,
				UploadStatus: &UploadStatus{
					Status:  Success,
					Type:    UploadTypesAzure,
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
			imageStatus: ImageStatus{
				Status: ImageStatusStatusSuccess,
				UploadStatus: &UploadStatus{
					Status:  Success,
					Type:    UploadTypesGcp,
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
			imageStatus: ImageStatus{
				Status: ImageStatusStatusSuccess,
				UploadStatus: &UploadStatus{
					Status:  Success,
					Type:    UploadTypesOciObjectstorage,
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

		var result ComposeStatus
		err := json.Unmarshal([]byte(body), &result)
		require.NoError(t, err)
		require.Equal(t, payload.imageStatus, result.ImageStatus)
		require.Equal(t, cr.Distribution, result.Request.Distribution)
		require.Equal(t, cr.Distribution, result.Request.Distribution)
		require.True(t, (*result.Request.Customizations.Users)[0].IsRedacted())
	}
}
