package flatpak

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOciImageRefFromIndexComponents(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		repo     string
		digest   string
		want     string
	}{
		{
			name:     "https_host_and_leading_slash_on_repo",
			registry: "https://registry.example.com",
			repo:     "/flatpak/repo",
			digest:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			want:     "registry.example.com/flatpak/repo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			name:     "http_host_trailing_slash",
			registry: "http://localhost:5000/",
			repo:     "library/foo",
			digest:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			want:     "localhost:5000/library/foo@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ociImageRefFromIndexComponents(tt.registry, tt.repo, tt.digest)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestFindFlatpakInIndex(t *testing.T) {
	const wantRef = "app/org.test/x86_64/stable"
	const dig = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tests := []struct {
		name        string
		root        *ResponseRoot
		ref         string
		wantRepo    string
		wantDigest  string
		errContains string
	}{
		{
			name: "match_skips_noise_rows",
			root: &ResponseRoot{
				Registry: "https://registry.example.com",
				Results: []ResponseRepository{
					{
						Name: "/other",
						Images: []*ResponseImage{
							{Labels: map[string]string{"org.flatpak.ref": "other"}, Digest: "sha256:1111"},
						},
					},
					{
						Name: "/flatpak/want",
						Images: []*ResponseImage{
							{Labels: map[string]string{"org.flatpak.ref": wantRef}, Digest: dig},
						},
					},
				},
			},
			ref:        wantRef,
			wantRepo:   "/flatpak/want",
			wantDigest: dig,
		},
		{
			name: "not_found",
			root: &ResponseRoot{
				Registry: "https://r.example",
				Results: []ResponseRepository{
					{Name: "/x", Images: []*ResponseImage{{Labels: map[string]string{"org.flatpak.ref": "other"}}}},
				},
			},
			ref:         wantRef,
			errContains: "did not find",
		},
		{
			name: "missing_registry",
			root: &ResponseRoot{
				Registry: "",
				Results: []ResponseRepository{
					{Name: "/x", Images: []*ResponseImage{{Labels: map[string]string{"org.flatpak.ref": wantRef}, Digest: dig}}},
				},
			},
			ref:         wantRef,
			errContains: "Registry field was missing",
		},
		{
			name: "nil_image_skipped",
			root: &ResponseRoot{
				Registry: "https://r.example",
				Results: []ResponseRepository{
					{Name: "/x", Images: []*ResponseImage{nil, {Digest: dig, Labels: map[string]string{"org.flatpak.ref": wantRef}}}},
				},
			},
			ref:        wantRef,
			wantRepo:   "/x",
			wantDigest: dig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, gotDig, err := findFlatpakInIndex(tt.root, tt.ref)
			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("err=%v want substring %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if repo != tt.wantRepo || gotDig != tt.wantDigest {
				t.Fatalf("repo=%q digest=%q want repo=%q digest=%q", repo, gotDig, tt.wantRepo, tt.wantDigest)
			}
		})
	}
}

func TestFetchRegistryIndex(t *testing.T) {
	const wantRef = "app/org.test/x86_64/stable"
	const wantDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tests := []struct {
		name         string
		status       int
		body         string
		os           string
		tag          string
		checkQuery   bool
		wantRef      string
		wantDigest   string
		wantRegistry string
		wantRepo     string
		wantErr      bool
		errContains  string
	}{
		{
			name:   "query_params_and_decode_registry_first",
			status: http.StatusOK,
			body: fmt.Sprintf(
				`{"Registry":"https://registry.example.com","Results":[{"Name":"/flatpak/test","Images":[{"Digest":%q,"Labels":{"org.flatpak.ref":%q}}]}]}`,
				wantDigest, wantRef,
			),
			os:           "linux",
			tag:          "stable",
			checkQuery:   true,
			wantRef:      wantRef,
			wantDigest:   wantDigest,
			wantRegistry: "https://registry.example.com",
			wantRepo:     "/flatpak/test",
		},
		{
			name:   "decode_results_before_registry",
			status: http.StatusOK,
			body: fmt.Sprintf(
				`{"Results":[{"Name":"/r","Images":[{"Digest":%q,"Labels":{"org.flatpak.ref":%q}}]}],"Registry":"https://r.example"}`,
				"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "app/x",
			),
			os:           "linux",
			tag:          "latest",
			checkQuery:   false,
			wantRef:      "app/x",
			wantDigest:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			wantRegistry: "https://r.example",
			wantRepo:     "/r",
		},
		{
			name:        "http_not_ok",
			status:      http.StatusGone,
			body:        "",
			os:          "linux",
			tag:         "latest",
			wantErr:     true,
			errContains: "410",
		},
		{
			name:    "invalid_json",
			status:  http.StatusOK,
			body:    `{`,
			os:      "linux",
			tag:     "latest",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.status != http.StatusOK {
					http.Error(w, "gone", tt.status)
					return
				}
				if tt.checkQuery {
					if !strings.Contains(r.URL.Path, "/index/static") {
						t.Errorf("path %q", r.URL.Path)
					}
					q := r.URL.Query()
					if q.Get("label:org.flatpak.ref:exists") != "1" {
						t.Errorf("label:org.flatpak.ref:exists: got %q", q.Get("label:org.flatpak.ref:exists"))
					}
					if q.Get("os") != tt.os {
						t.Errorf("os: got %q want %q", q.Get("os"), tt.os)
					}
					if q.Get("tag") != tt.tag {
						t.Errorf("tag: got %q want %q", q.Get("tag"), tt.tag)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			idx, err := NewOCIRegistryIndex(ts.URL, tt.os, tt.tag)
			if err != nil {
				t.Fatal(err)
			}
			defer idx.Close()

			root, err := idx.getResponseRoot()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("err=%v want substring %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if root.Registry != tt.wantRegistry {
				t.Errorf("Registry: got %q want %q", root.Registry, tt.wantRegistry)
			}
			repo, dig, err := findFlatpakInIndex(root, tt.wantRef)
			if err != nil {
				t.Fatal(err)
			}
			if repo != tt.wantRepo || dig != tt.wantDigest {
				t.Fatalf("repo=%q digest=%q want repo=%q digest=%q", repo, dig, tt.wantRepo, tt.wantDigest)
			}
		})
	}
}

func TestOCIRegistryIndex_cacheReusesGET(t *testing.T) {
	const body = `{"Registry":"https://registry.example.com","Results":[]}`
	var indexGets atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/index/static") {
			indexGets.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	idx, err := NewOCIRegistryIndex(ts.URL, "linux", "latest")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if _, err := idx.getResponseRoot(); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.getResponseRoot(); err != nil {
		t.Fatal(err)
	}
	if indexGets.Load() != 1 {
		t.Fatalf("expected 1 index GET, got %d", indexGets.Load())
	}

	idx.Close()
	if _, err := idx.getResponseRoot(); err != nil {
		t.Fatal(err)
	}
	if indexGets.Load() != 2 {
		t.Fatalf("after Close expected 2 index GETs, got %d", indexGets.Load())
	}
}
