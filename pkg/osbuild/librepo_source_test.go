package osbuild_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

var (
	opensslPkg = rpmmd.Package{
		Name:            "openssl-libs",
		Epoch:           1,
		Version:         "3.0.1",
		Release:         "5.el9",
		Arch:            "x86_64",
		RemoteLocations: []string{"https://example.com/repo/Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm"},
		Checksum:        rpmmd.Checksum{Type: "sha256", Value: "fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666"},
		Location:        "Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm",
		RepoID:          "repo_id_metalink",
	}

	pamPkg = rpmmd.Package{
		Name:            "pam",
		Epoch:           0,
		Version:         "1.5.1",
		Release:         "9.el9",
		Arch:            "x86_64",
		RemoteLocations: []string{"https://example.com/repo/Packages/pam-1.5.1-9.el9.x86_64.rpm"},
		Checksum:        rpmmd.Checksum{Type: "sha256", Value: "e64caedce811645ecdd78e7b4ae83c189aa884ff1ba6445374f39186c588c52c"},
		Location:        "Packages/pam-1.5.1-9.el9.x86_64.rpm",
		RepoID:          "repo_id_mirrorlist",
	}

	dbusPkg = rpmmd.Package{
		Name:            "dbus",
		Epoch:           1,
		Version:         "1.12.20",
		Release:         "5.el9",
		Arch:            "x86_64",
		RemoteLocations: []string{"https://example.com/repo/Packages/dbus-1.12.20-5.el9.x86_64.rpm"},
		Checksum:        rpmmd.Checksum{Type: "sha256", Value: "bb85bd28cc162e98da53b756b988ffd9350f4dbcc186f4c6962ae047e27f83d3"},
		Location:        "Packages/dbus-1.12.20-5.el9.x86_64.rpm",
		RepoID:          "repo_id_baseurls",
	}
)

var fakeRepos = []rpmmd.RepoConfig{
	{
		Id:       "repo_id_metalink",
		Name:     "repo1",
		Metalink: "http://example.com/metalink",
	},
	{
		Id:         "repo_id_mirrorlist",
		Name:       "repo1",
		MirrorList: "http://example.com/mirrorlist",
	},
	{
		Id:       "repo_id_baseurls",
		Name:     "repo1",
		BaseURLs: []string{"http://example.com/baseurl1"},
	},
}

func TestLibrepoAddPackage(t *testing.T) {
	sources := osbuild.NewLibrepoSource()
	err := sources.AddPackage(opensslPkg, fakeRepos)
	assert.NoError(t, err)
	err = sources.AddPackage(pamPkg, fakeRepos)
	assert.NoError(t, err)
	err = sources.AddPackage(dbusPkg, fakeRepos)
	assert.NoError(t, err)

	expectedJSON := `{
  "items": {
    "sha256:bb85bd28cc162e98da53b756b988ffd9350f4dbcc186f4c6962ae047e27f83d3": {
      "path": "Packages/dbus-1.12.20-5.el9.x86_64.rpm",
      "mirror": "repo_id_baseurls"
    },
    "sha256:e64caedce811645ecdd78e7b4ae83c189aa884ff1ba6445374f39186c588c52c": {
      "path": "Packages/pam-1.5.1-9.el9.x86_64.rpm",
      "mirror": "repo_id_mirrorlist"
    },
    "sha256:fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666": {
      "path": "Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm",
      "mirror": "repo_id_metalink"
    }
  },
  "options": {
    "mirrors": {
      "repo_id_baseurls": {
        "url": "http://example.com/baseurl1",
        "type": "baseurl"
      },
      "repo_id_metalink": {
        "url": "http://example.com/metalink",
        "type": "metalink"
      },
      "repo_id_mirrorlist": {
        "url": "http://example.com/mirrorlist",
        "type": "mirrorlist"
      }
    }
  }
}`
	b, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(b))
}

func TestLibrepoInsecure(t *testing.T) {
	pkg := opensslPkg
	pkg.IgnoreSSL = true

	sources := osbuild.NewLibrepoSource()
	err := sources.AddPackage(pkg, fakeRepos)
	assert.NoError(t, err)

	expectedJSON := `{
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
        "type": "metalink",
        "insecure": true
      }
    }
  }
}`
	b, err := json.MarshalIndent(sources, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(b))
}

func TestLibrepoSecrets(t *testing.T) {
	for _, secret := range []string{"org.osbuild.rhsm", "org.osbuild.mtls"} {
		pkg := opensslPkg
		pkg.Secrets = secret

		sources := osbuild.NewLibrepoSource()
		err := sources.AddPackage(pkg, fakeRepos)
		assert.NoError(t, err)

		expectedJSON := fmt.Sprintf(`{
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
        "type": "metalink",
        "secrets": {
          "name": "%s"
        }
      }
    }
  }
}`, secret)
		b, err := json.MarshalIndent(sources, "", "  ")
		assert.NoError(t, err)
		assert.Equal(t, expectedJSON, string(b))
	}
}

func TestLibrepoJsonMinimal(t *testing.T) {
	expectedJSON := `{
  "url": "http://example.com",
  "type": "metalink"
}`
	sourceMirror := osbuild.LibrepoSourceMirror{
		URL:  "http://example.com",
		Type: "metalink",
	}
	b, err := json.MarshalIndent(sourceMirror, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(b))
}

func TestLibrepoJsonFull(t *testing.T) {
	expectedJSON := `{
  "url": "http://example.com",
  "type": "metalink",
  "insecure": true,
  "secrets": {
    "name": "org.osbuild.mtls"
  },
  "max-parallels": 10,
  "fastest-mirror": true
}`
	sourceMirror := osbuild.LibrepoSourceMirror{
		URL:           "http://example.com",
		Type:          "metalink",
		Insecure:      true,
		Secrets:       &osbuild.URLSecrets{Name: "org.osbuild.mtls"},
		MaxParallels:  common.ToPtr(10),
		FastestMirror: true,
	}
	b, err := json.MarshalIndent(sourceMirror, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(b))
}

func TestLibrepoRepoIdNotFound(t *testing.T) {
	pkg := opensslPkg
	pkg.RepoID = "invalid_repo_id"

	sources := osbuild.NewLibrepoSource()
	err := sources.AddPackage(pkg, fakeRepos)
	assert.EqualError(t, err, `cannot find repo-id for pkg openssl-libs: cannot find repo-id invalid_repo_id in [{ID:repo_id_metalink Name:repo1} {ID:repo_id_mirrorlist Name:repo1} {ID:repo_id_baseurls Name:repo1}]`)
}

func TestLibrepoInconsistentSSLConfiguration(t *testing.T) {
	pkg := opensslPkg
	pkg.IgnoreSSL = true

	sources := osbuild.NewLibrepoSource()
	err := sources.AddPackage(pkg, fakeRepos)
	assert.NoError(t, err)
	pkg.IgnoreSSL = false
	err = sources.AddPackage(pkg, fakeRepos)
	assert.EqualError(t, err, `inconsistent SSL configuration: package openssl-libs requires SSL but mirror http://example.com/metalink is configured to ignore SSL`)
}
