package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestParseCoprURL(t *testing.T) {
	tests := []struct {
		spec    string
		owner   string
		project string
		err     string
	}{
		{"copr://@osbuild/osbuild", "@osbuild", "osbuild", ""},
		{"copr://daan/myproject", "daan", "myproject", ""},
		{"copr://@group/my-project", "@group", "my-project", ""},
		{"copr://", "", "", "invalid copr URL"},
		{"copr://noslash", "", "", "invalid copr URL"},
		{"copr:///project", "", "", "invalid copr URL"},
		{"copr://owner/", "", "", "invalid copr URL"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			u := mustParseURL(t, tt.spec)
			owner, project, err := parseCoprURL(u)
			if tt.err != "" {
				assert.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.owner, owner)
				assert.Equal(t, tt.project, project)
			}
		})
	}
}

func TestDistroToCoprChroots(t *testing.T) {
	tests := []struct {
		distro   string
		expected []string
	}{
		{"fedora-44", []string{"fedora-44"}},
		{"fedora-43", []string{"fedora-43"}},
		{"centos-10", []string{"centos-stream-10", "centos-10"}},
		{"centos-9", []string{"centos-stream-9", "centos-9"}},
		{"rhel-10.2", []string{"rhel-10.2", "rhel-10", "epel-10.2", "epel-10"}},
		{"rhel-10", []string{"rhel-10", "epel-10"}},
		{"rhel-9.6", []string{"rhel-9.6", "rhel-9", "epel-9.6", "epel-9"}},
		{"almalinux-9.4", []string{"almalinux-9.4"}},
	}

	for _, tt := range tests {
		t.Run(tt.distro, func(t *testing.T) {
			assert.Equal(t, tt.expected, distroToCoprChroots(tt.distro))
		})
	}
}

func mockCoprServer(t *testing.T, response coprAPIResponse, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
}

func TestCoprProjectRepoConfig(t *testing.T) {
	cp := &coprProject{
		FullName: "@osbuild/osbuild",
		Owner:    "@osbuild",
		Name:     "osbuild",
		ChrootRepos: map[string]string{
			"fedora-44-x86_64":        "https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/fedora-44-x86_64/",
			"fedora-44-aarch64":       "https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/fedora-44-aarch64/",
			"centos-stream-10-x86_64": "https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/centos-stream-10-x86_64/",
		},
		GPGKeyURL: "https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/pubkey.gpg",
	}

	t.Run("matching chroot", func(t *testing.T) {
		rc := cp.repoConfig("fedora-44-x86_64", "extra", 0)
		require.NotNil(t, rc)
		assert.Equal(t, "extra-copr-0", rc.Id)
		assert.Equal(t, []string{"https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/fedora-44-x86_64/"}, rc.BaseURLs)
		assert.Equal(t, []string{"https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/pubkey.gpg"}, rc.GPGKeys)
		assert.True(t, *rc.CheckGPG)
		assert.False(t, *rc.CheckRepoGPG)
	})

	t.Run("non-matching chroot", func(t *testing.T) {
		rc := cp.repoConfig("fedora-99-x86_64", "extra", 0)
		assert.Nil(t, rc)
	})
}

func TestFetchCoprProjectNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origClient := coprHTTPClient
	origURL := coprBaseURL
	t.Cleanup(func() {
		coprHTTPClient = origClient
		coprBaseURL = origURL
	})
	coprHTTPClient = srv.Client()
	coprBaseURL = srv.URL

	_, err := fetchCoprProject("@noone", "noproject")
	assert.ErrorContains(t, err, "not found")
}

func TestFetchCoprProjectHappy(t *testing.T) {
	response := coprAPIResponse{
		FullName:  "@test/testproject",
		Ownername: "@test",
		Name:      "testproject",
		ChrootRepos: map[string]string{
			"fedora-44-x86_64": "https://download.copr.fedorainfracloud.org/results/@test/testproject/fedora-44-x86_64/",
		},
	}
	srv := mockCoprServer(t, response, http.StatusOK)
	defer srv.Close()

	origClient := coprHTTPClient
	origURL := coprBaseURL
	t.Cleanup(func() {
		coprHTTPClient = origClient
		coprBaseURL = origURL
	})
	coprHTTPClient = srv.Client()
	coprBaseURL = srv.URL

	cp, err := fetchCoprProject("@test", "testproject")
	require.NoError(t, err)
	assert.Equal(t, "@test/testproject", cp.FullName)
	assert.Equal(t, "@test", cp.Owner)
	assert.Equal(t, "testproject", cp.Name)
	assert.Contains(t, cp.ChrootRepos, "fedora-44-x86_64")
	assert.Contains(t, cp.GPGKeyURL, "pubkey.gpg")
}

func TestResolveCoprRepo(t *testing.T) {
	response := coprAPIResponse{
		FullName:  "@osbuild/osbuild",
		Ownername: "@osbuild",
		Name:      "osbuild",
		ChrootRepos: map[string]string{
			"fedora-44-x86_64":        "https://example.com/fedora-44-x86_64/",
			"centos-stream-10-x86_64": "https://example.com/centos-stream-10-x86_64/",
			"epel-9-x86_64":           "https://example.com/epel-9-x86_64/",
		},
	}
	srv := mockCoprServer(t, response, http.StatusOK)
	defer srv.Close()

	origClient := coprHTTPClient
	origURL := coprBaseURL
	t.Cleanup(func() {
		coprHTTPClient = origClient
		coprBaseURL = origURL
	})
	coprHTTPClient = srv.Client()
	coprBaseURL = srv.URL

	t.Run("fedora direct match", func(t *testing.T) {
		rc, err := resolveCoprRepo(mustParseURL(t, "copr://@osbuild/osbuild"), "extra", 0, "fedora-44", "x86_64")
		require.NoError(t, err)
		require.NotNil(t, rc)
		assert.Equal(t, []string{"https://example.com/fedora-44-x86_64/"}, rc.BaseURLs)
	})

	t.Run("centos maps to centos-stream", func(t *testing.T) {
		rc, err := resolveCoprRepo(mustParseURL(t, "copr://@osbuild/osbuild"), "extra", 0, "centos-10", "x86_64")
		require.NoError(t, err)
		require.NotNil(t, rc)
		assert.Equal(t, []string{"https://example.com/centos-stream-10-x86_64/"}, rc.BaseURLs)
	})

	t.Run("rhel falls back to epel", func(t *testing.T) {
		rc, err := resolveCoprRepo(mustParseURL(t, "copr://@osbuild/osbuild"), "extra", 0, "rhel-9.6", "x86_64")
		require.NoError(t, err)
		require.NotNil(t, rc)
		assert.Equal(t, []string{"https://example.com/epel-9-x86_64/"}, rc.BaseURLs)
	})

	t.Run("no matching chroot returns nil", func(t *testing.T) {
		rc, err := resolveCoprRepo(mustParseURL(t, "copr://@osbuild/osbuild"), "extra", 0, "fedora-99", "x86_64")
		require.NoError(t, err)
		assert.Nil(t, rc)
	})
}
