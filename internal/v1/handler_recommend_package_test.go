package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/tutils"
	"github.com/stretchr/testify/require"
)

func TestRecommendPackage_Success_with_StatusForbidden(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &ServerConfig{})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()
	payload := RecommendPackageRequest{
		Packages: []string{
			"some",
			"packages",
		},
		RecommendedPackages: func() int32 {
			recommendedPackages := int32(3)
			return recommendedPackages
		}(),
	}
	respStatusCode, body := tutils.PostResponseBody(t, apiSrv.URL+"/", payload)
	require.Equal(t, http.StatusCreated, respStatusCode)
	var result RecommendationsResponse
	expectedResult := RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_Success_with_StatusUnauthorized(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &ServerConfig{})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()
	payload := RecommendPackageRequest{
		Packages: []string{
			"some",
			"packages",
		},
		RecommendedPackages: func() int32 {
			recommendedPackages := int32(3)
			return recommendedPackages
		}(),
	}

	respStatusCode, body := tutils.PostResponseBody(t, apiSrv.URL+"/", payload)
	require.Equal(t, http.StatusCreated, respStatusCode)
	var result RecommendationsResponse
	expectedResult := RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_Success_with_no_packages(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		var result RecommendationsResponse
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &ServerConfig{})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()
	payload := RecommendPackageRequest{
		Packages: []string{
			"some",
			"packages",
		},
		RecommendedPackages: func() int32 {
			recommendedPackages := int32(3)
			return recommendedPackages
		}(),
	}
	respStatusCode, body := tutils.PostResponseBody(t, apiSrv.URL+"/", payload)
	require.Equal(t, http.StatusCreated, respStatusCode)
	var result RecommendationsResponse
	expectedResult := RecommendationsResponse{
		Packages: nil,
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_Success_with_packages(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer" == r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &ServerConfig{})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()
	payload := RecommendPackageRequest{
		Packages: []string{
			"some",
			"packages",
		},
		RecommendedPackages: func() int32 {
			recommendedPackages := int32(3)
			return recommendedPackages
		}(),
	}
	respStatusCode, body := tutils.PostResponseBody(t, apiSrv.URL+"/", payload)
	require.Equal(t, http.StatusCreated, respStatusCode)
	var result RecommendationsResponse
	expectedResult := RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_with_authenticationServer(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
		require.Equal(t, "Bearer dog_token", r.Header.Get("Authorization"))
	}))
	defer apiSrv.Close()

	oauthMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "client_credentials", r.PostForm.Get("grant_type"))
		require.Equal(t, "secret", r.PostForm.Get("client_secret"))
		require.Equal(t, "id", r.PostForm.Get("client_id"))

		err := json.NewEncoder(w).Encode(struct {
			AccessToken string    `json:"access_token"`
			Expiration  time.Time `json:"expiration"`
		}{
			AccessToken: "dog_token",
			Expiration:  time.Now().Add(time.Hour),
		})

		require.NoError(t, err)
	}))
	defer oauthMock.Close()

	srv, tokenSrv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL, OAuthURL: oauthMock.URL}, &ServerConfig{})
	defer func() {
		err := srv.Shutdown(context.Background())
		require.NoError(t, err)
	}()
	defer tokenSrv.Close()
	payload := RecommendPackageRequest{
		Packages: []string{
			"some",
			"packages",
		},
		RecommendedPackages: func() int32 {
			recommendedPackages := int32(3)
			return recommendedPackages
		}(),
	}
	respStatusCode, body := tutils.PostResponseBody(t, srv.URL+"/api/image-builder/v1/experimental/recommendations", payload)
	require.Equal(t, http.StatusOK, respStatusCode)
	var result RecommendationsResponse
	expectedResult := RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}
