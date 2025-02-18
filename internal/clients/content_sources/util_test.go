package content_sources_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder-crc/internal/clients/content_sources"
	"github.com/osbuild/image-builder-crc/internal/common"
)

func TestGetBaseURL(t *testing.T) {
	baseURL, err := content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Url:    common.ToPtr("someurl"),
		Origin: common.ToPtr("external"),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "someurl", baseURL)

	baseURL, err = content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Url:    common.ToPtr("someurl"),
		Origin: common.ToPtr("red_hat"),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "someurl", baseURL)

	csURL, err := url.Parse("https://realurl.com")
	require.NoError(t, err)
	baseURL, err = content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Url:               common.ToPtr("someurl"),
		LatestSnapshotUrl: common.ToPtr("realurl"),
		Origin:            common.ToPtr("upload"),
	}, csURL)
	require.NoError(t, err)
	require.Equal(t, "https://realurl.com/realurl", baseURL)

	_, err = content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Uuid: common.ToPtr("d15a0c50-1549-4d04-bcb8-b3553576acd4"),
	}, nil)
	require.Error(t, err)
}
