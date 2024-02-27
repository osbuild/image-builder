package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/composer"
	"github.com/osbuild/image-builder/internal/db"
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
	srv, tokenSrv := startServer(t, apiSrv.URL, "", &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
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
	name := "blueprint"
	description := "desc"
	blueprint := BlueprintBody{
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
	message, err = json.Marshal(blueprint)
	require.NoError(t, err)
	err = dbase.InsertBlueprint(id, versionId, "000000", "000000", name, description, message)
	require.NoError(t, err)

	respStatusCode, body := tutils.PostResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s/compose", id.String()), map[string]string{})
	require.Equal(t, http.StatusCreated, respStatusCode)

	var result []ComposeResponse
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, ids[0], result[0].Id)
	require.Equal(t, ids[1], result[1].Id)
}

func TestHandlers_GetBlueprintComposes(t *testing.T) {
	blueprintId := uuid.New()
	versionId := uuid.New()
	version2Id := uuid.New()
	imageName := "MyImageName"
	clientId := "ui"

	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, "", "", &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var result ComposesResponse

	err = dbase.InsertBlueprint(blueprintId, versionId, "000000", "500000", "blueprint", "blueprint desc", json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`))
	require.NoError(t, err)
	id1 := uuid.New()
	err = dbase.InsertCompose(id1, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "edge-installer"}]}`), &clientId, &versionId)
	require.NoError(t, err)
	id2 := uuid.New()
	err = dbase.InsertCompose(id2, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &versionId)
	require.NoError(t, err)

	err = dbase.UpdateBlueprint(version2Id, blueprintId, "000000", "blueprint", "desc2", json.RawMessage(`{"image_requests": [{"image_type": "aws"}, {"image_type": "gcp"}]}`))
	require.NoError(t, err)
	id3 := uuid.New()
	err = dbase.InsertCompose(id3, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &version2Id)
	require.NoError(t, err)
	id4 := uuid.New()
	err = dbase.InsertCompose(id4, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "gcp"}]}`), &clientId, &version2Id)
	require.NoError(t, err)

	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s/composes", blueprintId.String()), &tutils.AuthString0)
	require.NoError(t, err)

	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, blueprintId, *result.Data[0].BlueprintId)
	require.Equal(t, 2, *result.Data[0].BlueprintVersion)
	require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/composes?blueprint_id=%s&limit=100&offset=0", blueprintId.String()), result.Links.First)
	require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/composes?blueprint_id=%s&limit=100&offset=3", blueprintId.String()), result.Links.Last)
	require.Equal(t, 4, len(result.Data))
	require.Equal(t, 4, result.Meta.Count)

	// get composes for specific version
	respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s/composes?blueprint_version=2", blueprintId.String()), &tutils.AuthString0)
	require.NoError(t, err)

	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, blueprintId, *result.Data[0].BlueprintId)
	require.Equal(t, 2, *result.Data[0].BlueprintVersion)
	require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/composes?blueprint_id=%s&blueprint_version=2&limit=100&offset=0", blueprintId.String()), result.Links.First)
	require.Equal(t, fmt.Sprintf("/api/image-builder/v1.0/composes?blueprint_id=%s&blueprint_version=2&limit=100&offset=1", blueprintId.String()), result.Links.Last)
	require.Equal(t, 2, len(result.Data))
	require.Equal(t, 2, result.Meta.Count)

	// get composes for latest version
	respStatusCode, body = tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s/composes?blueprint_version=-1", blueprintId.String()), &tutils.AuthString0)
	require.NoError(t, err)

	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, blueprintId, *result.Data[0].BlueprintId)
	require.Equal(t, 2, *result.Data[0].BlueprintVersion)
}

func TestHandlers_GetBlueprint(t *testing.T) {
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, "", "", &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	id := uuid.New()
	versionId := uuid.New()

	uploadOptions := UploadRequest_Options{}
	err = uploadOptions.FromAWSUploadRequestOptions(AWSUploadRequestOptions{
		ShareWithAccounts: common.ToPtr([]string{"test-account"}),
	})
	require.NoError(t, err)
	name := "blueprint"
	description := "desc"
	blueprint := BlueprintBody{
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
	message, err = json.Marshal(blueprint)
	require.NoError(t, err)
	err = dbase.InsertBlueprint(id, versionId, "000000", "000000", name, description, message)
	require.NoError(t, err)

	respStatusCode, body := tutils.GetResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s", id.String()), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)

	var result BlueprintResponse
	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, id, result.Id)
	require.Equal(t, description, result.Description)
	require.Equal(t, name, result.Name)
	require.Equal(t, blueprint.ImageRequests, result.ImageRequests)
	require.Equal(t, blueprint.Distribution, result.Distribution)
	require.Equal(t, blueprint.Customizations, result.Customizations)
}

func TestHandlers_DeleteBlueprint(t *testing.T) {
	blueprintId := uuid.New()
	versionId := uuid.New()
	version2Id := uuid.New()
	clientId := "ui"
	imageName := "MyImageName"

	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, "", "", &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	err = dbase.InsertBlueprint(blueprintId, versionId, "000000", "000000", "blueprint", "blueprint desc", json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`))
	require.NoError(t, err)
	id1 := uuid.New()
	err = dbase.InsertCompose(id1, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "edge-installer"}]}`), &clientId, &versionId)
	require.NoError(t, err)
	id2 := uuid.New()
	err = dbase.InsertCompose(id2, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &versionId)
	require.NoError(t, err)

	err = dbase.UpdateBlueprint(version2Id, blueprintId, "000000", "blueprint", "desc2", json.RawMessage(`{"image_requests": [{"image_type": "aws"}, {"image_type": "gcp"}]}`))
	require.NoError(t, err)
	id3 := uuid.New()
	err = dbase.InsertCompose(id3, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &version2Id)
	require.NoError(t, err)
	id4 := uuid.New()
	err = dbase.InsertCompose(id4, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "gcp"}]}`), &clientId, &version2Id)
	require.NoError(t, err)

	respStatusCode, body := tutils.DeleteResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s", blueprintId.String()))
	require.Equal(t, 204, respStatusCode)
	require.Equal(t, "", body)

	var errorResponse HTTPErrorList
	notFoundCode, body := tutils.DeleteResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/experimental/blueprints/%s", blueprintId.String()))
	require.Equal(t, 404, notFoundCode)
	err = json.Unmarshal([]byte(body), &errorResponse)
	require.NoError(t, err)
	require.Equal(t, "Not Found", errorResponse.Errors[0].Detail)

	_, err = dbase.GetBlueprint(blueprintId, "000000", "000000")
	require.ErrorIs(t, err, db.BlueprintNotFoundError)

	// Composes should not be assigned to the blueprint anymore
	bpComposes, err := dbase.GetBlueprintComposes("000000", blueprintId, nil, (time.Hour * 24 * 14), 10, 0, nil)
	require.NoError(t, err)
	require.Len(t, bpComposes, 0)
}
