package content_sources_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder-crc/internal/clients/content_sources"
	"github.com/osbuild/image-builder-crc/internal/common"
)

func TestGetBaseURL(t *testing.T) {
	url, err := content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Url:    common.ToPtr("someurl"),
		Origin: common.ToPtr("external"),
	})
	require.NoError(t, err)
	require.Equal(t, "someurl", url)

	url, err = content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Url:    common.ToPtr("someurl"),
		Origin: common.ToPtr("red_hat"),
	})
	require.NoError(t, err)
	require.Equal(t, "someurl", url)

	url, err = content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Url:               common.ToPtr("someurl"),
		LatestSnapshotUrl: common.ToPtr("realurl"),
		Origin:            common.ToPtr("upload"),
	})
	require.NoError(t, err)
	require.Equal(t, "realurl", url)

	_, err = content_sources.GetBaseURL(content_sources.ApiRepositoryResponse{
		Uuid: common.ToPtr("d15a0c50-1549-4d04-bcb8-b3553576acd4"),
	})
	require.Error(t, err)
}
