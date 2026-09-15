package rpmmd_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

func TestRepoConfigMarshalEmpty(t *testing.T) {
	repoCfg := &rpmmd.RepoConfig{}
	js, err := json.Marshal(repoCfg)
	assert.NoError(t, err)
	assert.Equal(t, string(js), `{}`)
}

func TestRepoConfigUnmarshalHappy(t *testing.T) {
	testCases := []struct {
		name string
		json string
		repo rpmmd.Repository
	}{
		{
			name: "single-baseurl",
			json: `{"baseurl":"http://example.com/repo"}`,
			repo: rpmmd.Repository{BaseURL: []string{"http://example.com/repo"}},
		},
		{
			name: "multiple-baseurls",
			json: `{"baseurl":["http://example.com/repo1", "http://example.com/repo2"]}`,
			repo: rpmmd.Repository{BaseURL: []string{"http://example.com/repo1", "http://example.com/repo2"}},
		},
		{
			name: "empty",
			json: `{}`,
			repo: rpmmd.Repository{},
		},
		{
			name: "ignore-ssl",
			json: `{"ignore_ssl":true}`,
			repo: rpmmd.Repository{IgnoreSSL: common.ToPtr(true)},
		},
		{
			name: "verify-ssl",
			json: `{"ignore_ssl":false}`,
			repo: rpmmd.Repository{IgnoreSSL: common.ToPtr(false)},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var repos rpmmd.Repository
			err := yaml.Unmarshal([]byte(tc.json), &repos)
			assert.NoError(t, err)
			assert.Equal(t, tc.repo, repos)

			configs, err := rpmmd.LoadRepositoriesFromReader(strings.NewReader(`{"aarch64":[` + tc.json + `]}`))
			require.NoError(t, err)
			require.Len(t, configs["aarch64"], 1)
			assert.Equal(t, tc.repo.IgnoreSSL, configs["aarch64"][0].IgnoreSSL)
		})
	}
}

func TestRepoConfigUnmarshalSad(t *testing.T) {
	testCases := []struct {
		name        string
		json        string
		expectedErr string
	}{
		{
			name:        "wrong type",
			json:        `{"baseurl":true}`,
			expectedErr: `unexpected type for baseurl: bool`,
		},
		{
			name:        "wrong baseurl list content",
			json:        `{"baseurl": ["url1", 2.71]}`,
			expectedErr: `unexpected non-string value 2.71 in baseurl list`,
		},
		{
			name:        "wrong json",
			json:        `all-wrong`,
			expectedErr: `cannot unmarshal string into Go value of type struct`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var repos rpmmd.Repository
			err := yaml.Unmarshal([]byte(tc.json), &repos)
			assert.ErrorContains(t, err, tc.expectedErr)
		})
	}
}
