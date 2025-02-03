package v1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/tutils"
	v1 "github.com/osbuild/image-builder/internal/v1"
)

func TestRecommendPackage_Success_with_StatusForbidden(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := v1.RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &v1.ServerConfig{})
	defer srv.Shutdown(t)
	payload := v1.RecommendPackageRequest{
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
	var result v1.RecommendationsResponse
	expectedResult := v1.RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_Success_with_StatusUnauthorized(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := v1.RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &v1.ServerConfig{})
	defer srv.Shutdown(t)
	payload := v1.RecommendPackageRequest{
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
	var result v1.RecommendationsResponse
	expectedResult := v1.RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_Success_with_no_packages(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		var result v1.RecommendationsResponse
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &v1.ServerConfig{})
	defer srv.Shutdown(t)
	payload := v1.RecommendPackageRequest{
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
	var result v1.RecommendationsResponse
	expectedResult := v1.RecommendationsResponse{
		Packages: nil,
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_Success_with_packages(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		require.Equal(t, "", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		result := v1.RecommendationsResponse{
			Packages: []string{"vim", "python"},
		}
		err := json.NewEncoder(w).Encode(result)
		require.NoError(t, err)
	}))
	defer apiSrv.Close()

	srv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL}, &v1.ServerConfig{})
	defer srv.Shutdown(t)
	payload := v1.RecommendPackageRequest{
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
	var result v1.RecommendationsResponse
	expectedResult := v1.RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestRecommendPackage_with_authenticationServer(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := v1.RecommendationsResponse{
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

	srv := startServer(t, &testServerClientsConf{RecommendURL: apiSrv.URL, OAuthURL: oauthMock.URL}, &v1.ServerConfig{})
	defer srv.Shutdown(t)
	payload := v1.RecommendPackageRequest{
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
	var result v1.RecommendationsResponse
	expectedResult := v1.RecommendationsResponse{
		Packages: []string{"vim", "python"},
	}
	err := json.Unmarshal([]byte(body), &result)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}
