package osbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/rpmmd"
)

func TestSource_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Type   string
		Source Source
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "invalid json",
			args: args{
				data: []byte(`{"name":"org.osbuild.foo","options":{"bar":null}`),
			},
			wantErr: true,
		},
		{
			name: "unknown source",
			args: args{
				data: []byte(`{"name":"org.osbuild.foo","options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "missing options",
			args: args{
				data: []byte(`{"name":"org.osbuild.curl"}`),
			},
			wantErr: true,
		},
		{
			name: "missing name",
			args: args{
				data: []byte(`{"foo":null,"options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "curl-empty",
			fields: fields{
				Type:   "org.osbuild.curl",
				Source: &CurlSource{Items: map[string]CurlSourceItem{}},
			},
			args: args{
				data: []byte(`{"org.osbuild.curl":{"items":{}}}`),
			},
		},
		{
			name: "curl-with-secrets",
			fields: fields{
				Type: "org.osbuild.curl",
				Source: &CurlSource{
					Items: map[string]CurlSourceItem{
						"checksum1": CurlSourceOptions{URL: "url1", Secrets: &URLSecrets{Name: "org.osbuild.rhsm"}},
						"checksum2": CurlSourceOptions{URL: "url2", Secrets: &URLSecrets{Name: "whatever"}},
					}},
			},
			args: args{
				data: []byte(`{"org.osbuild.curl":{"items":{"checksum1":{"url":"url1","secrets":{"name":"org.osbuild.rhsm"}},"checksum2":{"url":"url2","secrets":{"name":"whatever"}}}}}`),
			},
		},
		{
			name: "curl-url-only",
			fields: fields{
				Type: "org.osbuild.curl",
				Source: &CurlSource{
					Items: map[string]CurlSourceItem{
						"checksum1": URL("url1"),
						"checksum2": URL("url2"),
					}},
			},
			args: args{
				data: []byte(`{"org.osbuild.curl":{"items":{"checksum1":"url1","checksum2":"url2"}}}`),
			},
		},
		{
			name: "librepo",
			fields: fields{
				Type: "org.osbuild.librepo",
				Source: &LibrepoSource{
					Items: map[string]*LibrepoSourceItem{
						"checksum1": &LibrepoSourceItem{Path: "path1", MirrorID: "mirror1"},
					},
					Options: &LibrepoSourceOptions{
						Mirrors: map[string]*LibrepoSourceMirror{
							"mirror1": &LibrepoSourceMirror{
								URL:  "http://example.com/metalink",
								Type: "metalink",
							},
						},
					},
				},
			},
			args: args{
				data: []byte(`{"org.osbuild.librepo":{"items":{"checksum1":{"path":"path1","mirror":"mirror1"}},"options":{"mirrors":{"mirror1":{"url":"http://example.com/metalink","type":"metalink"}}}}}`),
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := &Sources{
				tt.fields.Type: tt.fields.Source,
			}
			var gotSources Sources
			if err := gotSources.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Sources.UnmarshalJSON() error = %v, wantErr %v [idx: %d]", err, tt.wantErr, idx)
			}
			if tt.wantErr {
				return
			}
			gotBytes, err := json.Marshal(sources)
			if err != nil {
				t.Errorf("Could not marshal source: %v [idx: %d]", err, idx)
			}
			if !bytes.Equal(gotBytes, tt.args.data) {
				t.Errorf("Expected '%v', got '%v' [idx: %d]", string(tt.args.data), string(gotBytes), idx)
			}
			if !reflect.DeepEqual(&gotSources, sources) {
				t.Errorf("got '%v', expected '%v' [idx:%d]", &gotSources, sources, idx)
			}
		})
	}
}

func TestGenSourcesTrivial(t *testing.T) {
	sources, err := GenSources(SourceInputs{}, 0)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `{}`)
}

func TestGenSourcesContainerStorage(t *testing.T) {
	imageID := "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
	containers := []container.Spec{
		{
			ImageID:      imageID,
			LocalStorage: true,
		},
	}
	sources, err := GenSources(SourceInputs{Containers: containers}, 0)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `{
  "org.osbuild.containers-storage": {
    "items": {
      "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f": {}
    }
  }
}`)
}

func TestGenSourcesSkopeo(t *testing.T) {
	imageID := "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
	digest := "sha256:aabbcc5cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
	containers := []container.Spec{
		{
			Source:  "some-source",
			Digest:  digest,
			ImageID: imageID,
		},
	}
	sources, err := GenSources(SourceInputs{Containers: containers}, 0)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `{
  "org.osbuild.skopeo": {
    "items": {
      "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f": {
        "image": {
          "name": "some-source",
          "digest": "sha256:aabbcc5cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
        }
      }
    }
  }
}`)
}

func TestGenSourcesWithSkopeoIndex(t *testing.T) {
	imageID := "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
	digest := "sha256:aabbcc5cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
	listDigest := "sha256:ffeeaabbcc90e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
	containers := []container.Spec{
		{
			Source:     "some-source",
			Digest:     digest,
			ListDigest: listDigest,
			ImageID:    imageID,
		},
	}
	sources, err := GenSources(SourceInputs{Containers: containers}, 0)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `{
  "org.osbuild.skopeo": {
    "items": {
      "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f": {
        "image": {
          "name": "some-source",
          "digest": "sha256:aabbcc5cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"
        }
      }
    }
  },
  "org.osbuild.skopeo-index": {
    "items": {
      "sha256:ffeeaabbcc90e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f": {
        "image": {
          "name": "some-source"
        }
      }
    }
  }
}`)
}

// TODO: move into a common "rpmtest" package
var fakeRepo = rpmmd.RepoConfig{
	Id:       "repo_id_metalink",
	Metalink: "http://example.com/metalink",
}

var opensslPkg = rpmmd.Package{
	Name:            "openssl-libs",
	RemoteLocations: []string{"https://example.com/repo/Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm"},
	Checksum:        rpmmd.Checksum{Type: "sha256", Value: "fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666"},
	Location:        "Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm",
	RepoID:          fakeRepo.Id,
	Repo:            &fakeRepo,
}

func TestGenSourcesRpmDefaultRpmDownloaderIsCurl(t *testing.T) {
	inputs := SourceInputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{opensslPkg},
			},
			Repos: []rpmmd.RepoConfig{fakeRepo},
		},
	}
	var defaultRpmDownloader RpmDownloader
	sources, err := GenSources(inputs, defaultRpmDownloader)
	assert.NoError(t, err)

	assert.NotNil(t, sources["org.osbuild.curl"])
	assert.Nil(t, sources["org.osbuild.librepo"])
}

func TestGenSourcesRpmWithLibcurl(t *testing.T) {
	inputs := SourceInputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{opensslPkg},
			},
			Repos: []rpmmd.RepoConfig{fakeRepo},
		},
	}
	sources, err := GenSources(inputs, RpmDownloaderCurl)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `{
  "org.osbuild.curl": {
    "items": {
      "sha256:fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666": {
        "url": "https://example.com/repo/Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm"
      }
    }
  }
}`)
}

func TestGenSourcesRpmWithLibrepo(t *testing.T) {
	inputs := SourceInputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{opensslPkg},
			},
			Repos: []rpmmd.RepoConfig{fakeRepo},
		},
	}
	sources, err := GenSources(inputs, RpmDownloaderLibrepo)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `{
  "org.osbuild.librepo": {
    "items": {
      "sha256:fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666": {
        "path": "Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm",
        "mirror": "repo_id_metalink"
      }
    },
    "options": {
      "mirrors": {
        "repo_id_metalink": {
          "url": "http://example.com/metalink",
          "type": "metalink"
        }
      }
    }
  }
}`)
}

func TestGenSourcesRpmBad(t *testing.T) {
	inputs := SourceInputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{opensslPkg},
			},
			Repos: []rpmmd.RepoConfig{fakeRepo},
		},
	}
	_, err := GenSources(inputs, 99)
	assert.EqualError(t, err, "unknown rpm downloader 99")
}

func TestGenSourcesFileRefs(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "test1.txt")
	err := os.WriteFile(testFile, nil, 0644)
	assert.NoError(t, err)

	sources, err := GenSources(SourceInputs{
		FileRefs: []string{testFile},
	}, 0)
	assert.NoError(t, err)

	jsonOutput, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), fmt.Sprintf(`{
  "org.osbuild.curl": {
    "items": {
      "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855": {
        "url": "file:%s"
      }
    }
  }
}`, testFile))
}
