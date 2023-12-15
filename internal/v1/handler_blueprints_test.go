package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/tutils"
)

func TestHandlers_ComposeBlueprint(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	idx := 0
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := composer.ComposeId{
			Id: ids[idx],
		}
		idx += 1
		encodeErr := json.NewEncoder(w).Encode(result)
		require.NoError(t, encodeErr)
	}))
	defer apiSrv.Close()

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	srv, tokenSrv := startServerWithCustomDB(t, apiSrv.URL, "", dbase, "../../distributions", "")
	defer func() {
		shutdownErr := srv.Shutdown(context.Background())
		require.NoError(t, shutdownErr)
	}()
	defer tokenSrv.Close()

	id := uuid.New()
	versionId := uuid.New()

	uploadOptions := UploadRequest_Options{}
	err = uploadOptions.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: common.ToPtr([]string{"test-account"}),
	})
	require.NoError(t, err)
	blueprintBody := BlueprintV1{
		Version:     1,
		Name:        "blueprint",
		Description: "desc",
		Customizations: Customizations{
			Packages: common.ToPtr([]string{"nginx"}),
		},
		Distribution: "centos-8",
		ImageRequests: []ImageRequest{
			{
				Architecture: ImageRequestArchitectureX8664,
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type:    UploadTypesAws,
					Options: uploadOptions,
				},
			},
			{
				Architecture: ImageRequestArchitectureAarch64,
				ImageType:    ImageTypesAws,
				UploadRequest: UploadRequest{
					Type:    UploadTypesAws,
					Options: uploadOptions,
				},
			},
		},
	}

	var message []byte
	message, err = json.Marshal(blueprintBody)
	require.NoError(t, err)
	err = dbase.InsertBlueprint(id, versionId, "000000", "000000", "blueprint", "desc", message)
	require.NoError(t, err)

	respStatusCode, body := tutils.PostResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprint/%s/compose", id.String()), map[string]string{})
	require.Equal(t, http.StatusCreated, respStatusCode)

	var result []ComposeResponse
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, ids[0], result[0].Id)
	require.Equal(t, ids[1], result[1].Id)
}
