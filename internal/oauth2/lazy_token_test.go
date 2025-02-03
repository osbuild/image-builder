package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/osbuild/logging/pkg/collect"
	"github.com/osbuild/logging/pkg/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLazyToken(t *testing.T) {
	nextTokenID := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextTokenID++
		w.Header().Set("Content-Type", "application/json")
		tokenResponse := tokenResponse{
			AccessToken: fmt.Sprintf("mock-token-%d", nextTokenID),
			ExpiresIn:   3600, // expires in 1 hour
		}
		err := json.NewEncoder(w).Encode(tokenResponse)
		require.NoError(t, err)
		clientID := r.FormValue("client_id")
		require.Equal(t, "test-client-id", clientID)

		clientSecret := r.FormValue("client_secret")
		require.Equal(t, "test-client-secret", clientSecret)

		grantType := r.FormValue("grant_type")
		require.Equal(t, "client_credentials", grantType)
	}))
	defer mockServer.Close()

	// this is not thread-safe but tests are not running in parallel
	collector := collect.NewTestHandler(slog.LevelDebug, false, false, false)
	old := slog.Default().Handler()
	slog.SetDefault(slog.New(collector))
	defer slog.SetDefault(slog.New(old))

	// temporary logrus proxy - will be removed
	oldP := logrus.Default()
	logrus.SetDefault(logrus.NewProxyFor(slog.Default()))
	defer logrus.SetDefault(oldP)

	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	lazyToken := &LazyToken{
		Url:          mockServer.URL,
		ClientId:     clientID,
		ClientSecret: clientSecret,
	}
	ctx := context.Background()
	token, err := lazyToken.Token(ctx)
	require.NoError(t, err)
	require.Equal(t, "mock-token-1", token)

	// ensure no token is not part of the logs
	field := collector.Last()["msg"]
	assert.Equal(t, 1, len(collector.All()))
	assert.Contains(t, field, "Acquired new token")
	assert.NotContains(t, field, "mock-token-1")
	collector.Reset()

	token, err = lazyToken.Token(ctx)
	require.NoError(t, err)
	require.Equal(t, "mock-token-1", token)

	field = collector.Last()["msg"]
	assert.Equal(t, 1, len(collector.All()))
	assert.Contains(t, field, "AccessToken reused")
	assert.NotContains(t, field, "mock-token-1")
	collector.Reset()

	// generates a new token when token expired
	lazyToken.Expiration = time.Now().Add(-time.Minute) // Expire the token
	token, err = lazyToken.Token(ctx)
	require.NoError(t, err)
	require.Equal(t, "mock-token-2", token)
	require.True(t, lazyToken.Expiration.After(time.Now()), "Expiration should be in the future")

	// Calling ForceRefresh generates a new token
	token, err = lazyToken.ForceRefresh(ctx)
	require.NoError(t, err)
	require.Equal(t, "mock-token-3", token)
}

func TestLazyTokenUnhappy(t *testing.T) {
	// Set up a mock token server that always returns an HTTP error
	mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Mock token server error", http.StatusInternalServerError)
	}))
	defer mockTokenServer.Close()
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	lazyToken := &LazyToken{
		Url:          mockTokenServer.URL,
		ClientId:     clientID,
		ClientSecret: clientSecret,
	}

	ctx := context.Background()

	// Ensure that NextToken() returns an error when the token server responds with an error
	_, err := lazyToken.Token(ctx)
	require.Error(t, err)
}
