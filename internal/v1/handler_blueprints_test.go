package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/clients/content_sources"

	"github.com/google/uuid"
	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/db"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/tutils"
)

func TestHandlers_CreateBlueprint(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("crypt() not supported on darwin")
	}

	var jsonResp HTTPErrorList
	ctx := context.Background()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
		CSReposURL:       "https://content-sources.org",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	body := map[string]interface{}{
		"name":        "Blueprint",
		"description": "desc",
		"customizations": map[string]interface{}{
			"packages": []string{"nginx"},
			"users": []map[string]interface{}{
				{"name": "user", "password": "test"},
				{"name": "user2", "ssh_key": "ssh-rsa AAAAB3NzaC1"},
			},
		},
		"distribution": "centos-9",
		"image_requests": []map[string]interface{}{
			{
				"architecture":   "x86_64",
				"image_type":     "aws",
				"upload_request": map[string]interface{}{"type": "aws", "options": map[string]interface{}{"share_with_accounts": []string{"test-account"}}},
			},
		},
	}
	statusCodePost, respPost := tutils.PostResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints", body)
	require.Equal(t, http.StatusCreated, statusCodePost)

	var result CreateBlueprintResponse
	err = json.Unmarshal([]byte(respPost), &result)
	require.NoError(t, err)

	be, err := dbase.GetBlueprint(ctx, result.Id, "000000", nil)
	require.NoError(t, err)
	require.Nil(t, be.Metadata)

	// Test unique name constraint
	statusCode, resp := tutils.PostResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints", body)
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)
	err = json.Unmarshal([]byte(resp), &jsonResp)
	require.NoError(t, err)
	require.Equal(t, "Name not unique", jsonResp.Errors[0].Title)

	// Test non empty name constraint
	body["name"] = ""
	statusCode, resp = tutils.PostResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints", body)
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)
	err = json.Unmarshal([]byte(resp), &jsonResp)
	require.NoError(t, err)
	require.Equal(t, "Invalid blueprint name", jsonResp.Errors[0].Title)

	// Test customization users, user without password and key is invalid
	body["name"] = "Blueprint with invalid user"
	body["customizations"] = map[string]interface{}{"users": []map[string]interface{}{{"name": "test"}}}
	statusCode, resp = tutils.PostResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints", body)
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)
	err = json.Unmarshal([]byte(resp), &jsonResp)
	require.NoError(t, err)
	require.Equal(t, "Invalid user", jsonResp.Errors[0].Title)
}

func TestUser_MergeForUpdate(t *testing.T) {
	tests := []struct {
		name          string
		newUser       User
		existingUsers []User
		wantPass      *string
		wantSsh       *string
		wantErr       bool
	}{
		{
			name: "Both password and ssh_key are provided, no need to fetch user from DB",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr("password"),
				SshKey:   common.ToPtr("ssh key"),
			},
			existingUsers: []User{},
			wantPass:      common.ToPtr("password"),
			wantSsh:       common.ToPtr("ssh key"),
			wantErr:       false,
		},
		{
			name: "User found in DB, merge should keep new values",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr("password"),
				SshKey:   common.ToPtr("ssh key"),
			},
			existingUsers: []User{
				{
					Name:     "test",
					Password: common.ToPtr("old password"),
					SshKey:   common.ToPtr("old ssh key"),
				},
			},
			wantPass: common.ToPtr("password"),
			wantSsh:  common.ToPtr("ssh key"),
			wantErr:  false,
		},
		{
			name: "New user, empty password set to nil",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr(""),
				SshKey:   common.ToPtr("ssh key"),
			},
			existingUsers: []User{},
			wantPass:      nil,
			wantSsh:       common.ToPtr("ssh key"),
			wantErr:       false,
		},
		{
			name: "Existing user, empty password set to nil = change to only 'ssh key' user",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr(""),
				SshKey:   common.ToPtr("ssh key"),
			},
			existingUsers: []User{
				{
					Name:     "test",
					Password: common.ToPtr("old password"),
					SshKey:   nil,
				},
			},
			wantPass: nil,
			wantSsh:  common.ToPtr("ssh key"),
			wantErr:  false,
		},
		{
			name: "New user, empty ssh_key set to nil",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr("password"),
				SshKey:   common.ToPtr(""),
			},
			existingUsers: []User{},
			wantPass:      common.ToPtr("password"),
			wantSsh:       nil,
			wantErr:       false,
		},
		{
			name: "Existing user, empty ssh key set to nil = change to 'password' user",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr("password"),
				SshKey:   common.ToPtr(""),
			},
			existingUsers: []User{
				{
					Name:     "test",
					Password: nil,
					SshKey:   common.ToPtr("old ssh key"),
				},
			},
			wantPass: common.ToPtr("password"),
			wantSsh:  nil,
			wantErr:  false,
		},
		{
			name: "Both password and ssh_key are empty, invalid",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr(""),
				SshKey:   common.ToPtr(""),
			},
			existingUsers: []User{},
			wantPass:      nil,
			wantSsh:       nil,
			wantErr:       true,
		},
		{
			name: "Both password and ssh_key are nil, no existing user, invalid",
			newUser: User{
				Name: "test",
			},
			existingUsers: []User{},
			wantPass:      nil,
			wantSsh:       nil,
			wantErr:       true,
		},
		{
			name: "Both password and ssh_key are nil, existing user, keep old values",
			newUser: User{
				Name: "test",
			},
			existingUsers: []User{
				{
					Name:     "test",
					Password: common.ToPtr("old password"),
					SshKey:   common.ToPtr("old ssh key"),
				},
			},
			wantPass: common.ToPtr("old password"),
			wantSsh:  common.ToPtr("old ssh key"),
			wantErr:  false,
		},
		{
			name: "Empty password, existing user only with password, fail",
			newUser: User{
				Name:     "test",
				Password: common.ToPtr(""),
				SshKey:   nil,
			},
			existingUsers: []User{
				{
					Name:     "test",
					Password: common.ToPtr("old password"),
					SshKey:   nil,
				},
			},
			wantPass: nil,
			wantSsh:  nil,
			wantErr:  true,
		},
		{
			name: "Empty ssh key, existing user only with ssh key, fail",
			newUser: User{
				Name:     "test",
				SshKey:   common.ToPtr(""),
				Password: nil,
			},
			existingUsers: []User{
				{
					Name:     "test",
					Password: nil,
					SshKey:   common.ToPtr("old ssh key"),
				},
			},
			wantPass: nil,
			wantSsh:  nil,
			wantErr:  true,
		},
		{
			name: "Add new user to one already existing user, fail no password or ssh key",
			newUser: User{
				Name:     "test2",
				SshKey:   nil,
				Password: nil,
			},
			existingUsers: []User{
				{
					Name:     "test",
					SshKey:   common.ToPtr("old password"),
					Password: nil,
				},
			},
			wantPass: nil,
			wantSsh:  nil,
			wantErr:  true,
		},
		{
			name: "Add new user to one already existing user",
			newUser: User{
				Name:     "test2",
				SshKey:   common.ToPtr("ssh key"),
				Password: nil,
			},
			existingUsers: []User{
				{
					Name:     "test",
					SshKey:   common.ToPtr("old password"),
					Password: nil,
				},
			},
			wantPass: nil,
			wantSsh:  common.ToPtr("ssh key"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.newUser.MergeForUpdate(tt.existingUsers)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantPass, tt.newUser.Password)
				require.Equal(t, tt.wantSsh, tt.newUser.SshKey)
			}
		})
	}
}

func TestHandlers_UpdateBlueprint_CustomizationUser(t *testing.T) {
	var jsonResp HTTPErrorList
	ctx := context.Background()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	body := map[string]interface{}{
		"name":        "Blueprint",
		"description": "desc",
		"customizations": map[string]interface{}{
			"users": []map[string]interface{}{},
		},
		"distribution": "centos-9",
		"image_requests": []map[string]interface{}{
			{
				"architecture":   "x86_64",
				"image_type":     "aws",
				"upload_request": map[string]interface{}{"type": "aws", "options": map[string]interface{}{"share_with_accounts": []string{"test-account"}}},
			},
		},
	}

	var result ComposeResponse

	// No users in the blueprint = SUCCESS
	statusCode, responseBody := tutils.PostResponseBody(t, "http://localhost:8086/api/image-builder/v1/blueprints", body)
	require.Equal(t, http.StatusCreated, statusCode)
	err = json.Unmarshal([]byte(responseBody), &result)
	require.NoError(t, err)

	// Add new user with password = SUCCESS
	body["customizations"] = map[string]interface{}{"users": []map[string]interface{}{{"name": "test", "password": "test"}}}
	statusCode, _ = tutils.PutResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusCreated, statusCode)

	blueprintEntry, err := dbase.GetBlueprint(ctx, result.Id, "000000", nil)
	require.NoError(t, err)
	updatedBlueprint, err := BlueprintFromEntry(blueprintEntry)
	require.NoError(t, err)
	require.NotEmpty(t, (*updatedBlueprint.Customizations.Users)[0].Password) // hashed, can't compare with plaintext value
	require.Nil(t, (*updatedBlueprint.Customizations.Users)[0].SshKey)

	// Update with hashed password = SUCCESS
	userHashedPassword := "$6$foo"
	body["customizations"] = map[string]interface{}{"users": []map[string]interface{}{{"name": "test", "password": userHashedPassword}}}
	statusCode, _ = tutils.PutResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusCreated, statusCode)

	blueprintEntry, err = dbase.GetBlueprint(ctx, result.Id, "000000", nil)
	require.NoError(t, err)
	updatedBlueprint, err = BlueprintFromEntry(blueprintEntry)
	require.NoError(t, err)

	existingPassword := (*updatedBlueprint.Customizations.Users)[0].Password
	require.NotNil(t, existingPassword)
	require.Equal(t, userHashedPassword, *existingPassword)
	require.Nil(t, (*updatedBlueprint.Customizations.Users)[0].SshKey)

	// keep ssh key and remove password = FAIL (previous ssh key still empty)
	body["customizations"] = map[string]interface{}{"users": []map[string]interface{}{{"name": "test", "password": ""}}}
	statusCode, responseBody = tutils.PutResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)
	err = json.Unmarshal([]byte(responseBody), &jsonResp)
	require.NoError(t, err)
	require.Equal(t, "Invalid user", jsonResp.Errors[0].Title)

	// add ssh key and remove password = SUCCESS
	body["customizations"] = map[string]interface{}{"users": []map[string]interface{}{{"name": "test", "password": "", "ssh_key": "ssh key"}}}
	statusCode, _ = tutils.PutResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusCreated, statusCode)

	blueprintEntry, err = dbase.GetBlueprint(ctx, result.Id, "000000", nil)
	require.NoError(t, err)

	updatedBlueprint, err = BlueprintFromEntryWithRedactedPasswords(blueprintEntry)
	require.NoError(t, err)
	require.Nil(t, (*updatedBlueprint.Customizations.Users)[0].Password)

	updatedBlueprint, err = BlueprintFromEntry(blueprintEntry)
	require.NoError(t, err)
	sshKey := (*updatedBlueprint.Customizations.Users)[0].SshKey
	require.NotNil(t, sshKey)
	require.Equal(t, "ssh key", *sshKey)
	require.Nil(t, (*updatedBlueprint.Customizations.Users)[0].Password)

	// add new user without password or ssh_key = FAIL
	users := []map[string]interface{}{
		{"name": "test"},  // keep old values
		{"name": "test2"}, // FAIL
	}
	body["customizations"] = map[string]interface{}{"users": users}
	statusCode, responseBody = tutils.PutResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)
	err = json.Unmarshal([]byte(responseBody), &jsonResp)
	require.NoError(t, err)
	require.Equal(t, "Invalid user", jsonResp.Errors[0].Title)

	// add new user with password and ssh_key = SUCCESS
	users = []map[string]interface{}{
		{"name": "test"}, // keep old values
		{"name": "test2", "password": "test", "ssh_key": "ssh key"},
	}
	body["customizations"] = map[string]interface{}{"users": users}
	statusCode, _ = tutils.PutResponseBody(t, fmt.Sprintf("http://localhost:8086/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusCreated, statusCode)

	blueprintEntry, err = dbase.GetBlueprint(ctx, result.Id, "000000", nil)
	require.NoError(t, err)
	updatedBlueprint, err = BlueprintFromEntry(blueprintEntry)
	require.NoError(t, err)
	require.Len(t, *updatedBlueprint.Customizations.Users, 2)
	user1 := (*updatedBlueprint.Customizations.Users)[0]
	require.NotNil(t, user1.SshKey)
	require.Equal(t, "ssh key", *user1.SshKey)
	require.Nil(t, user1.Password)

	user2 := (*updatedBlueprint.Customizations.Users)[1]
	require.Equal(t, "test2", user2.Name)
	require.NotNil(t, user2.Password)
	require.NotNil(t, user2.SshKey)
}

func TestHandlers_UpdateBlueprint(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("crypt() not supported on darwin")
	}

	var jsonResp HTTPErrorList
	ctx := context.Background()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	body := map[string]interface{}{
		"name":           "Blueprint",
		"description":    "desc",
		"customizations": map[string]interface{}{"packages": []string{"nginx"}},
		"distribution":   "centos-9",
		"image_requests": []map[string]interface{}{
			{
				"architecture":   "x86_64",
				"image_type":     "aws",
				"upload_request": map[string]interface{}{"type": "aws", "options": map[string]interface{}{"share_with_accounts": []string{"test-account"}}},
			},
		},
	}
	statusCode, resp := tutils.PostResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints", body)
	require.Equal(t, http.StatusCreated, statusCode)
	var result ComposeResponse
	err = json.Unmarshal([]byte(resp), &result)
	require.NoError(t, err)

	// Test non empty name constraint
	body["name"] = ""
	statusCode, resp = tutils.PutResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", result.Id), body)
	require.Equal(t, http.StatusUnprocessableEntity, statusCode)
	err = json.Unmarshal([]byte(resp), &jsonResp)
	require.NoError(t, err)
	require.Equal(t, "Invalid blueprint name", jsonResp.Errors[0].Title)

	// Test non-existing blueprint
	body["name"] = "Changing to correct body"
	respStatusCodeNotFound, _ := tutils.PutResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", uuid.New()), body)
	require.Equal(t, http.StatusNotFound, respStatusCodeNotFound)

	body["customizations"] = map[string]interface{}{"users": []map[string]interface{}{{"name": "test", "password": "test"}}}
	statusCode, _ = tutils.PutResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", uuid.New()), body)
	require.Equal(t, http.StatusNotFound, statusCode)
}

func TestHandlers_ComposeBlueprint(t *testing.T) {
	ctx := context.Background()

	ids := []uuid.UUID{}
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newId := uuid.New()
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accesstoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := composer.ComposeId{
			Id: newId,
		}
		ids = append(ids, newId)
		encodeErr := json.NewEncoder(w).Encode(result)
		require.NoError(t, encodeErr)
	}))
	defer apiSrv.Close()

	dbase, err := dbc.NewDB()
	require.NoError(t, err)
	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		shutdownErr := srv.Shutdown(ctx)
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
	name := "Blueprint Human Name"
	description := "desc"
	blueprint := BlueprintBody{
		Customizations: Customizations{
			Packages: common.ToPtr([]string{"nginx"}),
			Users: common.ToPtr([]User{
				{
					Name:     "user1",
					Password: common.ToPtr("$6$password123"),
				},
				{
					Name:   "user2",
					SshKey: common.ToPtr("ssh-rsa AAAAB3NzaC1"),
				},
			}),
		},
		Distribution: "centos-9",
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
			{
				Architecture: ImageRequestArchitectureAarch64,
				ImageType:    ImageTypesGuestImage,
				UploadRequest: UploadRequest{
					Type:    UploadTypesAwsS3,
					Options: uploadOptions,
				},
			},
		},
	}
	var message []byte
	message, err = json.Marshal(blueprint)
	require.NoError(t, err)
	err = dbase.InsertBlueprint(ctx, id, versionId, "000000", "000000", name, description, message, nil)
	require.NoError(t, err)

	tests := map[string]struct {
		payload any
		expect  int
	}{
		"empty targets":    {payload: strings.NewReader(""), expect: 3},
		"multiple targets": {payload: ComposeBlueprintJSONBody{ImageTypes: &[]ImageTypes{"aws", "guest-image", "gcp"}}, expect: 3},
		"one target":       {payload: ComposeBlueprintJSONBody{ImageTypes: &[]ImageTypes{"aws"}}, expect: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			respStatusCode, body := tutils.PostResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/compose", id.String()), tc.payload)
			require.Equal(t, http.StatusCreated, respStatusCode)

			var result []ComposeResponse
			err = json.Unmarshal([]byte(body), &result)
			require.NoError(t, err)
			require.Len(t, result, tc.expect)
			for i := 0; i < tc.expect; i++ {
				require.Equal(t, ids[len(ids)-tc.expect+i], result[i].Id)
			}
		})
	}
	t.Run("non-existing blueprint", func(t *testing.T) {
		respStatusCode, _ := tutils.PostResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/compose", uuid.New()), ComposeBlueprintJSONBody{})
		require.Equal(t, http.StatusNotFound, respStatusCode)
	})
}

func TestHandlers_GetBlueprintComposes(t *testing.T) {
	ctx := context.Background()
	blueprintId := uuid.New()
	versionId := uuid.New()
	version2Id := uuid.New()
	imageName := "MyImageName"
	clientId := "ui"

	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	var result ComposesResponse

	err = dbase.InsertBlueprint(ctx, blueprintId, versionId, "000000", "500000", "blueprint", "blueprint desc", json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), nil)
	require.NoError(t, err)
	id1 := uuid.New()
	err = dbase.InsertCompose(ctx, id1, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "edge-installer"}]}`), &clientId, &versionId)
	require.NoError(t, err)
	id2 := uuid.New()
	err = dbase.InsertCompose(ctx, id2, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &versionId)
	require.NoError(t, err)

	err = dbase.UpdateBlueprint(ctx, version2Id, blueprintId, "000000", "blueprint", "desc2", json.RawMessage(`{"image_requests": [{"image_type": "aws"}, {"image_type": "gcp"}]}`))
	require.NoError(t, err)
	id3 := uuid.New()
	err = dbase.InsertCompose(ctx, id3, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &version2Id)
	require.NoError(t, err)
	id4 := uuid.New()
	err = dbase.InsertCompose(ctx, id4, "500000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "gcp"}]}`), &clientId, &version2Id)
	require.NoError(t, err)

	respStatusCode, body := tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/composes", blueprintId.String()), &tutils.AuthString0)
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
	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/composes?blueprint_version=2", blueprintId.String()), &tutils.AuthString0)
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
	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/composes?blueprint_version=-1", blueprintId.String()), &tutils.AuthString0)
	require.NoError(t, err)

	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, blueprintId, *result.Data[0].BlueprintId)
	require.Equal(t, 2, *result.Data[0].BlueprintVersion)

	// get composes for non-existing blueprint
	respStatusCode, _ = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/composes?blueprint_version=1", uuid.New().String()), &tutils.AuthString0)
	require.Equal(t, 404, respStatusCode)

	// get composes for a blueprint that does not have any composes
	id5 := uuid.New()
	versionId2 := uuid.New()
	err = dbase.InsertBlueprint(ctx, id5, versionId2, "000000", "500000", "newBlueprint", "blueprint desc", json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), nil)
	require.NoError(t, err)
	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/composes?blueprint_version=1", id5), &tutils.AuthString0)
	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, 0, len(result.Data))
	require.Equal(t, 0, result.Meta.Count)
}

func TestHandlers_BlueprintFromEntryWithRedactedPasswords(t *testing.T) {
	t.Run("plain password", func(t *testing.T) {
		body := []byte(`{"name": "Blueprint", "description": "desc", "customizations": {"users": [{"name": "user", "password": "foo"}]}, "distribution": "centos-9"}`)
		be := &db.BlueprintEntry{
			Body: body,
		}
		result, err := BlueprintFromEntryWithRedactedPasswords(be)
		require.NoError(t, err)
		require.NotEqual(t, common.ToPtr("foo"), (*result.Customizations.Users)[0].Password)
	})
	t.Run("already hashed password", func(t *testing.T) {
		body := []byte(`{"name": "Blueprint", "description": "desc", "customizations": {"users": [{"name": "user", "password": "$6$foo"}]}, "distribution": "centos-9"}`)
		be := &db.BlueprintEntry{
			Body: body,
		}
		result, err := BlueprintFromEntryWithRedactedPasswords(be)
		require.NoError(t, err)

		require.Nil(t, (*result.Customizations.Users)[0].Password)
	})
}

func TestHandlers_GetBlueprint(t *testing.T) {
	ctx := context.Background()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
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
			Users: common.ToPtr([]User{
				{
					Name:     "user",
					Password: common.ToPtr("password123"),
				},
			}),
		},
		Distribution: "centos-9",
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
	err = dbase.InsertBlueprint(ctx, id, versionId, "000000", "000000", name, description, message, nil)
	require.NoError(t, err)

	be, err := dbase.GetBlueprint(ctx, id, "000000", nil)
	require.NoError(t, err)
	require.Nil(t, be.Metadata)

	respStatusCode, body := tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", id.String()), &tutils.AuthString0)
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
	require.Equal(t, blueprint.Customizations.Packages, result.Customizations.Packages)
	// Check that the password returned is redacted
	for _, u := range *result.Customizations.Users {
		require.Nil(t, u.Password)
	}

	respStatusCodeNotFound, _ := tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", uuid.New()), &tutils.AuthString0)
	require.Equal(t, http.StatusNotFound, respStatusCodeNotFound)

	// fetch specific version
	version2Id := uuid.New()
	version2Body := BlueprintBody{}
	err = json.Unmarshal(message, &version2Body)
	require.NoError(t, err)
	version2Body.Customizations.Packages = common.ToPtr([]string{"nginx", "httpd"})
	var message2 []byte
	message2, err = json.Marshal(version2Body)
	require.NoError(t, err)
	err = dbase.UpdateBlueprint(ctx, version2Id, id, "000000", name, description, message2)
	require.NoError(t, err)

	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s?version=%d", id.String(), -1), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, version2Body.Customizations.Packages, result.Customizations.Packages)
	for _, u := range *result.Customizations.Users {
		require.Nil(t, u.Password)
	}

	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s?version=%d", id.String(), 2), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, version2Body.Customizations.Packages, result.Customizations.Packages)
	for _, u := range *result.Customizations.Users {
		require.Nil(t, u.Password)
	}

	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s?version=%d", id.String(), 1), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, blueprint.Customizations.Packages, result.Customizations.Packages)
	for _, u := range *result.Customizations.Users {
		require.Nil(t, u.Password)
	}
}

func TestHandlers_ExportBlueprint(t *testing.T) {
	ctx := context.Background()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	var composeId uuid.UUID
	var composerRequest composer.ComposeRequest
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
		if r.URL.Path == "/repositories/bulk_export/" {
			require.Equal(t, "application/json", r.Header.Get("content-type"))
			var body content_sources.ApiRepositoryExportRequest
			err := json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			gpgKey := "some-gpg-key"
			if slices.Equal(*body.RepositoryUuids, []string{
				repoPayloadId.String(),
			}) {
				result := []content_sources.ApiRepositoryExportResponse{
					{
						GpgKey: &gpgKey,
						Name:   common.ToPtr("payload"),
						Url:    common.ToPtr("http://snappy-url/snappy/baseos"),
					},
				}
				err = json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			} else if slices.Equal(*body.RepositoryUuids, []string{repoPayloadId.String(), repoPayloadId2.String()}) {
				result := []content_sources.ApiRepositoryExportResponse{
					{
						GpgKey: &gpgKey,
						Name:   common.ToPtr("payload"),
						Url:    common.ToPtr("http://snappy-url/snappy/baseos"),
					},
					{
						GpgKey: &gpgKey,
						Name:   common.ToPtr("payload2"),
						Url:    common.ToPtr("http://snappy-url/snappy/appstream"),
					},
				}
				err = json.NewEncoder(w).Encode(result)
				require.NoError(t, err)
			}
		}
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{ComposerURL: apiSrv.URL, CSURL: csSrv.URL}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
		CSReposURL:       "https://content-sources.org",
	})
	defer func() {
		err := srv.Shutdown(context.Background())
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
			Subscription: &Subscription{
				ActivationKey: "aaa",
			},
			Users: common.ToPtr([]User{
				{
					Name:     "user",
					Password: common.ToPtr("password123"),
				},
			}),
			CustomRepositories: &[]CustomRepository{
				{
					Baseurl: &[]string{"http://snappy-url/snappy/baseos"},
					Name:    common.ToPtr("payload"),
					Gpgkey:  &[]string{"some-gpg-key"},
					Id:      repoPayloadId.String(),
				},
			},
		},
		Distribution: "centos-9",
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

	parentId := uuid.New()
	exportedAt := time.RFC3339
	metadata := BlueprintMetadata{
		ParentId:   &parentId,
		ExportedAt: exportedAt,
	}
	var metadataMessage []byte
	metadataMessage, err = json.Marshal(metadata)
	require.NoError(t, err)

	err = dbase.InsertBlueprint(ctx, id, versionId, "000000", "000000", name, description, message, metadataMessage)
	require.NoError(t, err)

	respStatusCode, body := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/export", id.String()), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)

	var result BlueprintExportResponse
	require.Equal(t, 200, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	var customRepositories []content_sources.ApiRepositoryExportResponse
	err = json.Unmarshal([]byte(*result.CustomRepositoriesDetails), &customRepositories)
	require.NoError(t, err)
	require.Equal(t, description, result.Description)
	require.Equal(t, name, result.Name)
	require.Equal(t, blueprint.Distribution, result.Distribution)
	require.Equal(t, blueprint.Customizations.Packages, result.Customizations.Packages)
	require.Equal(t, "payload", *customRepositories[0].Name)
	require.Equal(t, "http://snappy-url/snappy/baseos", *customRepositories[0].Url)
	require.Equal(t, "some-gpg-key", *customRepositories[0].GpgKey)
	require.Len(t, customRepositories, 1)
	// Check that the password returned is redacted
	for _, u := range *result.Customizations.Users {
		require.Nil(t, u.Password)
	}
	require.Nil(t, result.Customizations.Subscription)
	require.Equal(t, &id, result.Metadata.ParentId)
	require.NotEqual(t, metadata.ExportedAt, result.Metadata.ExportedAt)

	nameMeta := "blueprint with metadata"
	parentIdMeta := "be75e486-7f2b-4b0d-a0f2-de152dcd344a"
	bodyToImport := map[string]interface{}{
		"name":           nameMeta,
		"description":    "desc",
		"customizations": map[string]interface{}{"packages": []string{"nginx"}},
		"distribution":   "centos-9",
		"image_requests": []map[string]interface{}{
			{
				"architecture":   "x86_64",
				"image_type":     "aws",
				"upload_request": map[string]interface{}{"type": "aws", "options": map[string]interface{}{"share_with_accounts": []string{"test-account"}}},
			},
		},
		"metadata": map[string]interface{}{
			"parent_id":   parentIdMeta,
			"exported_at": exportedAt,
		},
	}

	statusPost, respPost := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/blueprints", bodyToImport)
	require.Equal(t, http.StatusCreated, statusPost)

	var resultPost CreateBlueprintResponse
	err = json.Unmarshal([]byte(respPost), &resultPost)
	require.NoError(t, err)

	be, err := dbase.GetBlueprint(ctx, resultPost.Id, "000000", nil)
	require.NoError(t, err)

	var resultMeta BlueprintMetadata
	require.NotNil(t, be.Metadata)
	err = json.Unmarshal(be.Metadata, &resultMeta)
	require.NoError(t, err)

	require.Equal(t, parentIdMeta, resultMeta.ParentId.String())
	require.Equal(t, exportedAt, resultMeta.ExportedAt)

	respStatusCodeNoCustomRepos, _ := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/export", id.String()), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCodeNoCustomRepos)

	id2 := uuid.New()
	versionId2 := uuid.New()
	moreRepos := BlueprintBody{
		Customizations: Customizations{
			Packages: common.ToPtr([]string{"nginx"}),
			Subscription: &Subscription{
				ActivationKey: "aaa",
			},
			Users: common.ToPtr([]User{
				{
					Name:     "user",
					Password: common.ToPtr("password123"),
				},
			}),
			CustomRepositories: &[]CustomRepository{
				{
					Baseurl: &[]string{"http://snappy-url/snappy/baseos"},
					Name:    common.ToPtr("payload"),
					Gpgkey:  &[]string{"some-gpg-key"},
					Id:      repoPayloadId.String(),
				},
				{
					Baseurl: &[]string{"http://snappy-url/snappy/appstream"},
					Name:    common.ToPtr("payload2"),
					Gpgkey:  &[]string{"some-gpg-key"},
					Id:      repoPayloadId2.String(),
				},
			},
		},
		Distribution: "centos-9",
	}

	var message2 []byte
	message2, err = json.Marshal(moreRepos)
	require.NoError(t, err)

	err = dbase.InsertBlueprint(ctx, id2, versionId2, "000000", "000000", "blueprint2", "", message2, metadataMessage)
	require.NoError(t, err)

	respStatusCodeMoreRepos, bodyMoreRepos := tutils.GetResponseBody(t, srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/export", id2.String()), &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCodeMoreRepos)

	var result2 BlueprintExportResponse
	err = json.Unmarshal([]byte(bodyMoreRepos), &result2)
	require.NoError(t, err)
	var customRepositories2 []content_sources.ApiRepositoryExportResponse
	err = json.Unmarshal([]byte(*result2.CustomRepositoriesDetails), &customRepositories2)
	require.NoError(t, err)
	require.Len(t, customRepositories2, 2)
	require.Equal(t, "payload", *customRepositories2[0].Name)
	require.Equal(t, "http://snappy-url/snappy/baseos", *customRepositories2[0].Url)
	require.Equal(t, "some-gpg-key", *customRepositories2[0].GpgKey)
	require.Equal(t, "payload2", *customRepositories2[1].Name)
	require.Equal(t, "http://snappy-url/snappy/appstream", *customRepositories2[1].Url)
	require.Equal(t, "some-gpg-key", *customRepositories2[1].GpgKey)
}

func TestHandlers_GetBlueprints(t *testing.T) {
	ctx := context.Background()
	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	blueprintId := uuid.New()
	versionId := uuid.New()
	err = dbase.InsertBlueprint(ctx, blueprintId, versionId, "000000", "000000", "blueprint", "blueprint desc", json.RawMessage(`{}`), nil)
	require.NoError(t, err)
	blueprintId2 := uuid.New()
	versionId2 := uuid.New()
	err = dbase.InsertBlueprint(ctx, blueprintId2, versionId2, "000000", "000000", "Blueprint2", "blueprint desc", json.RawMessage(`{}`), nil)
	require.NoError(t, err)

	var result BlueprintsResponse
	respStatusCode, body := tutils.GetResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints?name=blueprint", &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	require.Equal(t, blueprintId, result.Data[0].Id)

	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints?name=Blueprint", &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Len(t, result.Data, 0)
}

func TestHandlers_DeleteBlueprint(t *testing.T) {
	ctx := context.Background()
	blueprintId := uuid.New()
	versionId := uuid.New()
	version2Id := uuid.New()
	clientId := "ui"
	imageName := "MyImageName"

	dbase, err := dbc.NewDB()
	require.NoError(t, err)

	db_srv, tokenSrv := startServer(t, &testServerClientsConf{}, &ServerConfig{
		DBase:            dbase,
		DistributionsDir: "../../distributions",
	})
	defer func() {
		err := db_srv.Shutdown(ctx)
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()

	blueprintName := "blueprint"
	err = dbase.InsertBlueprint(ctx, blueprintId, versionId, "000000", "000000", blueprintName, "blueprint desc", json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), nil)
	require.NoError(t, err)
	id1 := uuid.New()
	err = dbase.InsertCompose(ctx, id1, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "edge-installer"}]}`), &clientId, &versionId)
	require.NoError(t, err)

	id2 := uuid.New()
	err = dbase.InsertCompose(ctx, id2, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &versionId)
	require.NoError(t, err)

	err = dbase.UpdateBlueprint(ctx, version2Id, blueprintId, "000000", "blueprint", "desc2", json.RawMessage(`{"image_requests": [{"image_type": "aws"}, {"image_type": "gcp"}]}`))
	require.NoError(t, err)
	id3 := uuid.New()
	err = dbase.InsertCompose(ctx, id3, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), &clientId, &version2Id)
	require.NoError(t, err)
	id4 := uuid.New()
	err = dbase.InsertCompose(ctx, id4, "000000", "user100000@test.test", "000000", &imageName, json.RawMessage(`{"image_requests": [{"image_type": "gcp"}]}`), &clientId, &version2Id)
	require.NoError(t, err)

	respStatusCode, body := tutils.DeleteResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", blueprintId.String()))
	require.Equal(t, 204, respStatusCode)
	require.Equal(t, "", body)

	var errorResponse HTTPErrorList
	notFoundCode, body := tutils.DeleteResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s", blueprintId.String()))
	require.Equal(t, 404, notFoundCode)
	err = json.Unmarshal([]byte(body), &errorResponse)
	require.NoError(t, err)
	require.Equal(t, "Not Found", errorResponse.Errors[0].Detail)

	_, err = dbase.GetBlueprint(ctx, blueprintId, "000000", nil)
	require.ErrorIs(t, err, db.BlueprintNotFoundError)

	// We should not be able to list deleted blueprint
	var result BlueprintsResponse
	respStatusCode, body = tutils.GetResponseBody(t, db_srv.URL+"/api/image-builder/v1/blueprints?name=blueprint", &tutils.AuthString0)
	require.Equal(t, http.StatusOK, respStatusCode)
	err = json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Len(t, result.Data, 0)

	// We should not be able to update deleted blueprint
	id5 := uuid.New()
	err = dbase.UpdateBlueprint(ctx, id5, blueprintId, "000000", "newName", "desc2", json.RawMessage(`{"image_requests": [{"image_type": "aws"}, {"image_type": "gcp"}]}`))
	require.ErrorIs(t, err, db.BlueprintNotFoundError)

	// Composes should not be assigned to the blueprint anymore
	respStatusCode, _ = tutils.GetResponseBody(t, db_srv.URL+fmt.Sprintf("/api/image-builder/v1/blueprints/%s/composes", blueprintId.String()), &tutils.AuthString0)
	require.Equal(t, 404, respStatusCode)

	// We should be able to create a Blueprint with same name
	blueprintId2 := uuid.New()
	versionId2 := uuid.New()
	err = dbase.InsertBlueprint(ctx, blueprintId2, versionId2, "000000", "000000", blueprintName, "blueprint desc", json.RawMessage(`{"image_requests": [{"image_type": "aws"}]}`), nil)
	require.NoError(t, err)

	bpComposes, err := dbase.GetBlueprintComposes(ctx, "000000", blueprintId2, nil, (time.Hour * 24 * 14), 10, 0, nil)
	require.Len(t, bpComposes, 0)
	require.NoError(t, err)
}

func TestBlueprintBody_CryptPasswords(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("crypt() not supported on darwin")
	}

	// Create a sample blueprint body with users
	passwordToHash := "password123"
	blueprint := &BlueprintBody{
		Customizations: Customizations{
			Users: &[]User{
				{
					Name:     "user1",
					Password: common.ToPtr(passwordToHash),
				},
				{
					Name:   "user2",
					SshKey: common.ToPtr("ssh-key-string"),
				},
			},
		},
	}

	err := blueprint.CryptPasswords()
	require.NoError(t, err)

	// Password hashed
	require.NotEqual(t, (*blueprint.Customizations.Users)[0].Password, passwordToHash)
	// No change with no password
	require.Nil(t, (*blueprint.Customizations.Users)[1].Password)
}

func TestUser_RedactPassword(t *testing.T) {
	user := &User{
		Name:     "test",
		Password: common.ToPtr("password123"),
	}

	user.RedactPassword()
	require.Nil(t, user.Password)
}
