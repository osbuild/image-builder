package depsolvednf

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixtures for V2 API parsing tests.
// These define the expected rpmmd types after parsing the corresponding JSON.

// testExpectedPackage is the expected rpmmd.Package after parsing testPackageJSON.
var testExpectedPackage = rpmmd.Package{
	Name:            "bash",
	Epoch:           1,
	Version:         "5.1.8",
	Release:         "9.el9",
	Arch:            "x86_64",
	RepoID:          "baseos",
	Location:        "Packages/bash-5.1.8-9.el9.x86_64.rpm",
	RemoteLocations: []string{"https://example.com/baseos/Packages/bash-5.1.8-9.el9.x86_64.rpm"},
	Checksum:        rpmmd.Checksum{Type: "sha256", Value: "abc123"},
	HeaderChecksum:  rpmmd.Checksum{Type: "sha256", Value: "def456"},
	License:         "GPLv3+",
	Summary:         "The GNU Bourne Again shell",
	Description:     "Bash is a shell",
	URL:             "https://www.gnu.org/software/bash",
	Vendor:          "Red Hat",
	Packager:        "Red Hat, Inc.",
	BuildTime:       time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
	DownloadSize:    1234567,
	InstallSize:     2345678,
	Group:           "System Environment/Shells",
	SourceRpm:       "bash-5.1.8-9.el9.src.rpm",
	Reason:          "user",
	Provides: rpmmd.RelDepList{
		{Name: "bash", Relationship: "=", Version: "5.1.8-9.el9"},
		{Name: "/bin/bash"},
	},
	Requires: rpmmd.RelDepList{
		{Name: "libc.so.6()(64bit)"},
	},
	Files:     []string{"/bin/bash", "/usr/bin/bash"},
	CheckGPG:  true, // from repo.GPGCheck=true
	IgnoreSSL: true, // from repo.SSLVerify=false
	Repo:      &testExpectedRepo,
}

// testExpectedRepo is the expected rpmmd.RepoConfig after parsing testRepoJSON.
var testExpectedRepo = rpmmd.RepoConfig{
	Id:             "baseos",
	Name:           "BaseOS",
	BaseURLs:       []string{"https://example.com/baseos"},
	GPGKeys:        []string{"file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release"},
	CheckGPG:       common.ToPtr(true),
	CheckRepoGPG:   common.ToPtr(true),
	IgnoreSSL:      common.ToPtr(true), // SSLVerify=false -> IgnoreSSL=true
	SSLCACert:      "/etc/pki/tls/certs/ca-bundle.crt",
	MetadataExpire: "86400",
	ModuleHotfixes: common.ToPtr(true),
	Enabled:        common.ToPtr(true),
}

// testParseDetailsInput is a complete V2 API response JSON containing
// one package and one repo, matching testExpectedPackage and testExpectedRepo.
const testParseDetailsInput = `{
	"solver": "dnf5",
	"transactions": [
		[
			{
				"name": "bash",
				"epoch": 1,
				"version": "5.1.8",
				"release": "9.el9",
				"arch": "x86_64",
				"repo_id": "baseos",
				"location": "Packages/bash-5.1.8-9.el9.x86_64.rpm",
				"remote_locations": ["https://example.com/baseos/Packages/bash-5.1.8-9.el9.x86_64.rpm"],
				"checksum": {"algorithm": "sha256", "value": "abc123"},
				"header_checksum": {"algorithm": "sha256", "value": "def456"},
				"license": "GPLv3+",
				"summary": "The GNU Bourne Again shell",
				"description": "Bash is a shell",
				"url": "https://www.gnu.org/software/bash",
				"vendor": "Red Hat",
				"packager": "Red Hat, Inc.",
				"build_time": "2023-01-15T10:30:00Z",
				"download_size": 1234567,
				"install_size": 2345678,
				"group": "System Environment/Shells",
				"source_rpm": "bash-5.1.8-9.el9.src.rpm",
				"reason": "user",
				"provides": [
					{"name": "bash", "relation": "=", "version": "5.1.8-9.el9"},
					{"name": "/bin/bash"}
				],
				"requires": [{"name": "libc.so.6()(64bit)"}],
				"requires_pre": [],
				"conflicts": [],
				"obsoletes": [],
				"regular_requires": [],
				"recommends": [],
				"suggests": [],
				"enhances": [],
				"supplements": [],
				"files": ["/bin/bash", "/usr/bin/bash"]
			}
		]
	],
	"repos": {
		"baseos": {
			"id": "baseos",
			"name": "BaseOS",
			"baseurl": ["https://example.com/baseos"],
			"metalink": "",
			"mirrorlist": "",
			"gpgcheck": true,
			"repo_gpgcheck": true,
			"gpgkey": ["file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release"],
			"sslverify": false,
			"sslcacert": "/etc/pki/tls/certs/ca-bundle.crt",
			"sslclientkey": "",
			"sslclientcert": "",
			"metadata_expire": "86400",
			"module_hotfixes": true,
			"rhsm": false
		}
	},
	"modules": {}
}`

// testParseDetailsDumpSearchInput is a V2 API dump/search response JSON
// containing one package and one repo, matching testExpectedPackage and testExpectedRepo.
const testParseDetailsDumpSearchInput = `{
	"solver": "dnf5",
	"packages": [
		{
			"name": "bash",
			"epoch": 1,
			"version": "5.1.8",
			"release": "9.el9",
			"arch": "x86_64",
			"repo_id": "baseos",
			"location": "Packages/bash-5.1.8-9.el9.x86_64.rpm",
			"remote_locations": ["https://example.com/baseos/Packages/bash-5.1.8-9.el9.x86_64.rpm"],
			"checksum": {"algorithm": "sha256", "value": "abc123"},
			"header_checksum": {"algorithm": "sha256", "value": "def456"},
			"license": "GPLv3+",
			"summary": "The GNU Bourne Again shell",
			"description": "Bash is a shell",
			"url": "https://www.gnu.org/software/bash",
			"vendor": "Red Hat",
			"packager": "Red Hat, Inc.",
			"build_time": "2023-01-15T10:30:00Z",
			"download_size": 1234567,
			"install_size": 2345678,
			"group": "System Environment/Shells",
			"source_rpm": "bash-5.1.8-9.el9.src.rpm",
			"reason": "user",
			"provides": [
				{"name": "bash", "relation": "=", "version": "5.1.8-9.el9"},
				{"name": "/bin/bash"}
			],
			"requires": [{"name": "libc.so.6()(64bit)"}],
			"requires_pre": [],
			"conflicts": [],
			"obsoletes": [],
			"regular_requires": [],
			"recommends": [],
			"suggests": [],
			"enhances": [],
			"supplements": [],
			"files": ["/bin/bash", "/usr/bin/bash"]
		}
	],
	"repos": {
		"baseos": {
			"id": "baseos",
			"name": "BaseOS",
			"baseurl": ["https://example.com/baseos"],
			"metalink": "",
			"mirrorlist": "",
			"gpgcheck": true,
			"repo_gpgcheck": true,
			"gpgkey": ["file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release"],
			"sslverify": false,
			"sslcacert": "/etc/pki/tls/certs/ca-bundle.crt",
			"sslclientkey": "",
			"sslclientcert": "",
			"metadata_expire": "86400",
			"module_hotfixes": true,
			"rhsm": false
		}
	}
}`

func TestV2HandlerMakeDepsolveRequest(t *testing.T) {
	baseOS := rpmmd.RepoConfig{
		Name:     "baseos",
		BaseURLs: []string{"https://example.org/baseos"},
	}
	appstream := rpmmd.RepoConfig{
		Name:     "appstream",
		BaseURLs: []string{"https://example.org/appstream"},
	}
	userRepo := rpmmd.RepoConfig{
		Name:     "user-repo",
		BaseURLs: []string{"https://example.org/user-repo"},
	}
	userRepo2 := rpmmd.RepoConfig{
		Name:     "user-repo-2",
		BaseURLs: []string{"https://example.org/user-repo-2"},
	}
	moduleHotfixRepo := rpmmd.RepoConfig{
		Name:           "module-hotfixes",
		BaseURLs:       []string{"https://example.org/nginx"},
		ModuleHotfixes: common.ToPtr(true),
	}
	mtlsRepo := rpmmd.RepoConfig{
		Name:          "mtls",
		BaseURLs:      []string{"https://example.org/mtls"},
		SSLCACert:     "/cacert",
		SSLClientCert: "/cert",
		SSLClientKey:  "/key",
	}

	testCases := []struct {
		name        string
		packageSets []rpmmd.PackageSet
		withSbom    bool
		wantJSON    string
	}{
		{
			name: "single transaction",
			packageSets: []rpmmd.PackageSet{
				{
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{
						baseOS,
						appstream,
					},
					InstallWeakDeps: true,
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "exclude-specs": ["pkg2"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": true}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash()),
		},
		{
			name: "2 transactions + package set specific repo",
			packageSets: []rpmmd.PackageSet{
				{
					Include:         []string{"pkg1"},
					Exclude:         []string{"pkg2"},
					Repositories:    []rpmmd.RepoConfig{baseOS, appstream},
					InstallWeakDeps: true,
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]},
						{"id": %[3]q, "name": "user-repo", "baseurl": ["https://example.org/user-repo"]}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "exclude-specs": ["pkg2"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": true},
						{"package-specs": ["pkg3"], "repo-ids": [%[1]q, %[2]q, %[3]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash(), userRepo.Hash()),
		},
		{
			name: "2 transactions + no package set specific repos",
			packageSets: []rpmmd.PackageSet{
				{
					Include:         []string{"pkg1"},
					Exclude:         []string{"pkg2"},
					Repositories:    []rpmmd.RepoConfig{baseOS, appstream},
					InstallWeakDeps: true,
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "exclude-specs": ["pkg2"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": true},
						{"package-specs": ["pkg3"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash()),
		},
		{
			name: "3 transactions + package set specific repo used by 2nd and 3rd transaction",
			packageSets: []rpmmd.PackageSet{
				{
					Include:         []string{"pkg1"},
					Exclude:         []string{"pkg2"},
					Repositories:    []rpmmd.RepoConfig{baseOS, appstream},
					InstallWeakDeps: true,
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
				{
					Include:      []string{"pkg4"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]},
						{"id": %[3]q, "name": "user-repo", "baseurl": ["https://example.org/user-repo"]}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "exclude-specs": ["pkg2"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": true},
						{"package-specs": ["pkg3"], "repo-ids": [%[1]q, %[2]q, %[3]q], "install_weak_deps": false},
						{"package-specs": ["pkg4"], "repo-ids": [%[1]q, %[2]q, %[3]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash(), userRepo.Hash()),
		},
		{
			name: "3 transactions + 3rd transaction using another repo",
			packageSets: []rpmmd.PackageSet{
				{
					Include:         []string{"pkg1"},
					Exclude:         []string{"pkg2"},
					Repositories:    []rpmmd.RepoConfig{baseOS, appstream},
					InstallWeakDeps: true,
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
				{
					Include:      []string{"pkg4"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo, userRepo2},
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]},
						{"id": %[3]q, "name": "user-repo", "baseurl": ["https://example.org/user-repo"]},
						{"id": %[4]q, "name": "user-repo-2", "baseurl": ["https://example.org/user-repo-2"]}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "exclude-specs": ["pkg2"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": true},
						{"package-specs": ["pkg3"], "repo-ids": [%[1]q, %[2]q, %[3]q], "install_weak_deps": false},
						{"package-specs": ["pkg4"], "repo-ids": [%[1]q, %[2]q, %[3]q, %[4]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash(), userRepo.Hash(), userRepo2.Hash()),
		},
		{
			name: "module hotfixes flag passed",
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, moduleHotfixRepo},
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]},
						{"id": %[3]q, "name": "module-hotfixes", "baseurl": ["https://example.org/nginx"], "module_hotfixes": true}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "repo-ids": [%[1]q, %[2]q, %[3]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash(), moduleHotfixRepo.Hash()),
		},
		{
			name: "mtls certs passed",
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, mtlsRepo},
				},
			},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]},
						{"id": %[3]q, "name": "mtls", "baseurl": ["https://example.org/mtls"], "sslcacert": "/cacert", "sslclientkey": "/key", "sslclientcert": "/cert"}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "repo-ids": [%[1]q, %[2]q, %[3]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"]
				}
			}`, baseOS.Hash(), appstream.Hash(), mtlsRepo.Hash()),
		},
		{
			name: "2 transactions + withSbom flag",
			packageSets: []rpmmd.PackageSet{
				{
					Include:         []string{"pkg1"},
					Exclude:         []string{"pkg2"},
					Repositories:    []rpmmd.RepoConfig{baseOS, appstream},
					InstallWeakDeps: true,
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
			},
			withSbom: true,
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "depsolve",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %[1]q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %[2]q, "name": "appstream", "baseurl": ["https://example.org/appstream"]}
					],
					"transactions": [
						{"package-specs": ["pkg1"], "exclude-specs": ["pkg2"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": true},
						{"package-specs": ["pkg3"], "repo-ids": [%[1]q, %[2]q], "install_weak_deps": false}
					],
					"root_dir": "/root",
					"optional-metadata": ["filelists"],
					"sbom": {"type": "spdx"}
				}
			}`, baseOS.Hash(), appstream.Hash()),
		},
	}

	cfg := &solverConfig{
		modulePlatformID: "platform:el8",
		arch:             "x86_64",
		releaseVer:       "8",
		cacheDir:         "/cache",
		rootDir:          "/root",
	}
	v2Handler := newV2Handler()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var sbomType sbom.StandardType
			if tt.withSbom {
				sbomType = sbom.StandardTypeSpdx
			}

			rawReq, err := v2Handler.makeDepsolveRequest(cfg, tt.packageSets, sbomType)
			require.NoError(t, err)
			require.NotEmpty(t, rawReq)
			assert.JSONEq(t, tt.wantJSON, string(rawReq))
		})
	}
}

func TestV2HandlerMakeDumpRequest(t *testing.T) {
	baseOS := rpmmd.RepoConfig{
		Name:     "baseos",
		BaseURLs: []string{"https://example.org/baseos"},
	}
	appstream := rpmmd.RepoConfig{
		Name:     "appstream",
		BaseURLs: []string{"https://example.org/appstream"},
	}
	mtlsRepo := rpmmd.RepoConfig{
		Name:          "mtls",
		BaseURLs:      []string{"https://example.org/mtls"},
		SSLCACert:     "/cacert",
		SSLClientCert: "/cert",
		SSLClientKey:  "/key",
	}

	testCases := []struct {
		name     string
		repos    []rpmmd.RepoConfig
		wantJSON string
	}{
		{
			name:  "single repo",
			repos: []rpmmd.RepoConfig{baseOS},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "dump",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %q, "name": "baseos", "baseurl": ["https://example.org/baseos"]}
					]
				}
			}`, baseOS.Hash()),
		},
		{
			name:  "multiple repos",
			repos: []rpmmd.RepoConfig{baseOS, appstream},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "dump",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %q, "name": "appstream", "baseurl": ["https://example.org/appstream"]}
					]
				}
			}`, baseOS.Hash(), appstream.Hash()),
		},
		{
			name:  "mtls certs passed",
			repos: []rpmmd.RepoConfig{baseOS, mtlsRepo},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "dump",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %q, "name": "mtls", "baseurl": ["https://example.org/mtls"], "sslcacert": "/cacert", "sslclientkey": "/key", "sslclientcert": "/cert"}
					]
				}
			}`, baseOS.Hash(), mtlsRepo.Hash()),
		},
	}

	cfg := &solverConfig{
		modulePlatformID: "platform:el8",
		arch:             "x86_64",
		releaseVer:       "8",
		cacheDir:         "/cache",
	}
	v2Handler := newV2Handler()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			rawReq, err := v2Handler.makeDumpRequest(cfg, tt.repos)
			require.NoError(t, err)
			require.NotEmpty(t, rawReq)
			assert.JSONEq(t, tt.wantJSON, string(rawReq))
		})
	}
}

func TestV2HandlerMakeSearchRequest(t *testing.T) {
	baseOS := rpmmd.RepoConfig{
		Name:     "baseos",
		BaseURLs: []string{"https://example.org/baseos"},
	}
	appstream := rpmmd.RepoConfig{
		Name:     "appstream",
		BaseURLs: []string{"https://example.org/appstream"},
	}

	testCases := []struct {
		name     string
		repos    []rpmmd.RepoConfig
		packages []string
		wantJSON string
	}{
		{
			name:     "single package search",
			repos:    []rpmmd.RepoConfig{baseOS, appstream},
			packages: []string{"vim"},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "search",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %q, "name": "baseos", "baseurl": ["https://example.org/baseos"]},
						{"id": %q, "name": "appstream", "baseurl": ["https://example.org/appstream"]}
					],
					"search": {"latest": false, "packages": ["vim"]}
				}
			}`, baseOS.Hash(), appstream.Hash()),
		},
		{
			name:     "glob pattern search",
			repos:    []rpmmd.RepoConfig{baseOS},
			packages: []string{"python3*", "kernel-*"},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "search",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %q, "name": "baseos", "baseurl": ["https://example.org/baseos"]}
					],
					"search": {"latest": false, "packages": ["python3*", "kernel-*"]}
				}
			}`, baseOS.Hash()),
		},
		{
			name:     "empty packages list",
			repos:    []rpmmd.RepoConfig{baseOS},
			packages: []string{},
			wantJSON: fmt.Sprintf(`{
				"api_version": 2,
				"command": "search",
				"module_platform_id": "platform:el8",
				"releasever": "8",
				"arch": "x86_64",
				"cachedir": "/cache",
				"arguments": {
					"repos": [
						{"id": %q, "name": "baseos", "baseurl": ["https://example.org/baseos"]}
					],
					"search": {"latest": false, "packages": []}
				}
			}`, baseOS.Hash()),
		},
	}

	cfg := &solverConfig{
		modulePlatformID: "platform:el8",
		arch:             "x86_64",
		releaseVer:       "8",
		cacheDir:         "/cache",
	}
	v2Handler := newV2Handler()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			rawReq, err := v2Handler.makeSearchRequest(cfg, tt.repos, tt.packages)
			require.NoError(t, err)
			require.NotEmpty(t, rawReq)
			assert.JSONEq(t, tt.wantJSON, string(rawReq))
		})
	}
}

func TestV2HandlerParseDepsolveResult(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		wantTransactions []int // len = number of transactions, values = packages per transaction
		wantRepos        int
		wantSolver       string
		wantModules      int
		wantErr          bool
	}{
		{
			name: "single transaction with packages",
			input: `{
				"solver": "dnf5",
				"transactions": [
					[
						{
							"name": "bash",
							"epoch": 0,
							"version": "5.1.8",
							"release": "9.el9",
							"arch": "x86_64",
							"repo_id": "baseos",
							"location": "Packages/bash-5.1.8-9.el9.x86_64.rpm",
							"remote_locations": ["https://example.com/baseos/Packages/bash-5.1.8-9.el9.x86_64.rpm"],
							"checksum": {"algorithm": "sha256", "value": "aaaa"},
							"header_checksum": null,
							"license": "GPLv3+",
							"summary": "The GNU Bourne Again shell",
							"description": "The GNU Bourne Again shell",
							"url": "https://www.gnu.org/software/bash",
							"vendor": "Red Hat",
							"packager": "Red Hat",
							"build_time": "2023-01-15T10:30:00Z",
							"download_size": 1234567,
							"install_size": 2345678,
							"group": "System Environment/Shells",
							"source_rpm": "bash-5.1.8-9.el9.src.rpm",
							"reason": "user",
							"provides": [{"name": "bash", "relation": "=", "version": "5.1.8-9.el9"}],
							"requires": [{"name": "libc.so.6()(64bit)"}],
							"requires_pre": [],
							"conflicts": [],
							"obsoletes": [],
							"regular_requires": [],
							"recommends": [],
							"suggests": [],
							"enhances": [],
							"supplements": [],
							"files": ["/bin/bash", "/usr/bin/bash"]
						}
					]
				],
				"repos": {
					"baseos": {
						"id": "baseos",
						"name": "BaseOS",
						"baseurl": ["https://example.com/baseos"],
						"metalink": null,
						"mirrorlist": null,
						"gpgcheck": true,
						"repo_gpgcheck": false,
						"gpgkey": ["file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release"],
						"sslverify": true,
						"sslcacert": null,
						"sslclientkey": null,
						"sslclientcert": null,
						"metadata_expire": "",
						"module_hotfixes": null,
						"rhsm": false
					}
				},
				"modules": {}
			}`,
			wantTransactions: []int{1},
			wantRepos:        1,
			wantSolver:       "dnf5",
			wantModules:      0,
		},
		{
			name: "multiple transactions",
			input: `{
				"solver": "dnf5",
				"transactions": [
					[
						{
							"name": "pkg1",
							"epoch": 0,
							"version": "1.0",
							"release": "1.el9",
							"arch": "x86_64",
							"repo_id": "baseos",
							"location": "Packages/pkg1.rpm",
							"remote_locations": ["https://example.com/pkg1.rpm"],
							"checksum": {"algorithm": "sha256", "value": "aaaa"},
							"header_checksum": null,
							"license": "MIT",
							"summary": "Package 1",
							"description": "Package 1 description",
							"url": "",
							"vendor": "",
							"packager": "",
							"build_time": null,
							"download_size": null,
							"install_size": null,
							"group": "",
							"source_rpm": "",
							"reason": "",
							"provides": [],
							"requires": [],
							"requires_pre": [],
							"conflicts": [],
							"obsoletes": [],
							"regular_requires": [],
							"recommends": [],
							"suggests": [],
							"enhances": [],
							"supplements": [],
							"files": []
						}
					],
					[
						{
							"name": "pkg2",
							"epoch": 0,
							"version": "2.0",
							"release": "1.el9",
							"arch": "x86_64",
							"repo_id": "baseos",
							"location": "Packages/pkg2.rpm",
							"remote_locations": ["https://example.com/pkg2.rpm"],
							"checksum": {"algorithm": "sha256", "value": "bbbb"},
							"header_checksum": null,
							"license": "MIT",
							"summary": "Package 2",
							"description": "Package 2 description",
							"url": "",
							"vendor": "",
							"packager": "",
							"build_time": null,
							"download_size": null,
							"install_size": null,
							"group": "",
							"source_rpm": "",
							"reason": "",
							"provides": [],
							"requires": [],
							"requires_pre": [],
							"conflicts": [],
							"obsoletes": [],
							"regular_requires": [],
							"recommends": [],
							"suggests": [],
							"enhances": [],
							"supplements": [],
							"files": []
						}
					]
				],
				"repos": {
					"baseos": {
						"id": "baseos",
						"name": "BaseOS",
						"baseurl": ["https://example.com/baseos"],
						"metalink": null,
						"mirrorlist": null,
						"gpgcheck": false,
						"repo_gpgcheck": false,
						"gpgkey": null,
						"sslverify": null,
						"sslcacert": null,
						"sslclientkey": null,
						"sslclientcert": null,
						"metadata_expire": "",
						"module_hotfixes": null,
						"rhsm": false
					}
				},
				"modules": {}
			}`,
			wantTransactions: []int{1, 1},
			wantRepos:        1,
			wantSolver:       "dnf5",
			wantModules:      0,
		},
		{
			name: "multiple repos",
			input: `{
				"solver": "dnf5",
				"transactions": [
					[
						{
							"name": "pkg1",
							"epoch": 0,
							"version": "1.0",
							"release": "1.el9",
							"arch": "x86_64",
							"repo_id": "baseos",
							"location": "Packages/pkg1.rpm",
							"remote_locations": ["https://example.com/pkg1.rpm"],
							"checksum": {"algorithm": "sha256", "value": "aaaa"},
							"header_checksum": null,
							"license": "MIT",
							"summary": "Package 1",
							"description": "Package 1 description",
							"url": "",
							"vendor": "",
							"packager": "",
							"build_time": null,
							"download_size": null,
							"install_size": null,
							"group": "",
							"source_rpm": "",
							"reason": "",
							"provides": [],
							"requires": [],
							"requires_pre": [],
							"conflicts": [],
							"obsoletes": [],
							"regular_requires": [],
							"recommends": [],
							"suggests": [],
							"enhances": [],
							"supplements": [],
							"files": []
						},
						{
							"name": "pkg2",
							"epoch": 0,
							"version": "2.0",
							"release": "1.el9",
							"arch": "x86_64",
							"repo_id": "appstream",
							"location": "Packages/pkg2.rpm",
							"remote_locations": ["https://example.com/pkg2.rpm"],
							"checksum": {"algorithm": "sha256", "value": "bbbb"},
							"header_checksum": null,
							"license": "MIT",
							"summary": "Package 2",
							"description": "Package 2 description",
							"url": "",
							"vendor": "",
							"packager": "",
							"build_time": null,
							"download_size": null,
							"install_size": null,
							"group": "",
							"source_rpm": "",
							"reason": "",
							"provides": [],
							"requires": [],
							"requires_pre": [],
							"conflicts": [],
							"obsoletes": [],
							"regular_requires": [],
							"recommends": [],
							"suggests": [],
							"enhances": [],
							"supplements": [],
							"files": []
						}
					]
				],
				"repos": {
					"baseos": {
						"id": "baseos",
						"name": "BaseOS",
						"baseurl": ["https://example.com/baseos"],
						"metalink": null,
						"mirrorlist": null,
						"gpgcheck": true,
						"repo_gpgcheck": false,
						"gpgkey": null,
						"sslverify": null,
						"sslcacert": null,
						"sslclientkey": null,
						"sslclientcert": null,
						"metadata_expire": "",
						"module_hotfixes": null,
						"rhsm": false
					},
					"appstream": {
						"id": "appstream",
						"name": "AppStream",
						"baseurl": ["https://example.com/appstream"],
						"metalink": null,
						"mirrorlist": null,
						"gpgcheck": false,
						"repo_gpgcheck": false,
						"gpgkey": null,
						"sslverify": null,
						"sslcacert": null,
						"sslclientkey": null,
						"sslclientcert": null,
						"metadata_expire": "",
						"module_hotfixes": null,
						"rhsm": false
					}
				},
				"modules": {}
			}`,
			wantTransactions: []int{2},
			wantRepos:        2,
			wantSolver:       "dnf5",
			wantModules:      0,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json`,
			wantErr: true,
		},
		{
			name: "repo not found error",
			input: `{
				"solver": "dnf5",
				"transactions": [
					[
						{
							"name": "pkg1",
							"epoch": 0,
							"version": "1.0",
							"release": "1.el9",
							"arch": "x86_64",
							"repo_id": "unknown-repo",
							"location": "Packages/pkg1.rpm",
							"remote_locations": ["https://example.com/pkg1.rpm"],
							"checksum": {"algorithm": "sha256", "value": "aaaa"},
							"header_checksum": null,
							"license": "MIT",
							"summary": "Package 1",
							"description": "Package 1 description",
							"url": "",
							"vendor": "",
							"packager": "",
							"build_time": null,
							"download_size": null,
							"install_size": null,
							"group": "",
							"source_rpm": "",
							"reason": "",
							"provides": [],
							"requires": [],
							"requires_pre": [],
							"conflicts": [],
							"obsoletes": [],
							"regular_requires": [],
							"recommends": [],
							"suggests": [],
							"enhances": [],
							"supplements": [],
							"files": []
						}
					]
				],
				"repos": {},
				"modules": {}
			}`,
			wantErr: true,
		},
	}

	v2Handler := newV2Handler()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v2Handler.parseDepsolveResult([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)

			// Check transactions
			require.Len(t, result.Transactions, len(tt.wantTransactions), "transaction count mismatch")
			for i, wantPkgCount := range tt.wantTransactions {
				assert.Len(t, result.Transactions[i], wantPkgCount, "transaction %d package count", i)
			}

			// Check other fields
			assert.Len(t, result.Repos, tt.wantRepos)
			assert.Equal(t, tt.wantSolver, result.Solver)
			assert.Len(t, result.Modules, tt.wantModules)
		})
	}
}

// TestV2HandlerParseDumpSearchResult tests both parseDumpResult and parseSearchResult
// since they share the same underlying parsing logic via parsePackageListResult.
func TestV2HandlerParseDumpSearchResult(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		wantPkgLen  int
		wantRepoLen int
		wantSolver  string
		wantErrMsg  string
	}{
		{
			name: "successful result with multiple packages",
			input: `{
				"solver": "dnf5",
				"packages": [
					{
						"name": "vim",
						"epoch": 0,
						"version": "8.2",
						"release": "1.el9",
						"arch": "x86_64",
						"repo_id": "appstream",
						"location": "Packages/vim-8.2-1.el9.x86_64.rpm",
						"remote_locations": ["https://example.com/vim.rpm"],
						"checksum": {"algorithm": "sha256", "value": "cccc"},
						"header_checksum": null,
						"license": "Vim",
						"summary": "The VIM editor",
						"description": "VIM (VIsual editor iMproved)",
						"url": "https://www.vim.org/",
						"vendor": "",
						"packager": "",
						"build_time": "2023-06-01T12:00:00Z",
						"download_size": 1500000,
						"install_size": 3000000,
						"group": "",
						"source_rpm": "vim-8.2-1.el9.src.rpm",
						"reason": "",
						"provides": [],
						"requires": [],
						"requires_pre": [],
						"conflicts": [],
						"obsoletes": [],
						"regular_requires": [],
						"recommends": [],
						"suggests": [],
						"enhances": [],
						"supplements": [],
						"files": []
					},
					{
						"name": "emacs",
						"epoch": 0,
						"version": "27.2",
						"release": "1.el9",
						"arch": "x86_64",
						"repo_id": "appstream",
						"location": "Packages/emacs-27.2-1.el9.x86_64.rpm",
						"remote_locations": ["https://example.com/emacs.rpm"],
						"checksum": {"algorithm": "sha256", "value": "dddd"},
						"header_checksum": null,
						"license": "GPLv3+",
						"summary": "GNU Emacs",
						"description": "GNU Emacs editor",
						"url": "https://www.gnu.org/software/emacs/",
						"vendor": "",
						"packager": "",
						"build_time": "2023-06-01T12:00:00Z",
						"download_size": 2500000,
						"install_size": 5000000,
						"group": "",
						"source_rpm": "emacs-27.2-1.el9.src.rpm",
						"reason": "",
						"provides": [],
						"requires": [],
						"requires_pre": [],
						"conflicts": [],
						"obsoletes": [],
						"regular_requires": [],
						"recommends": [],
						"suggests": [],
						"enhances": [],
						"supplements": [],
						"files": []
					}
				],
				"repos": {
					"appstream": {
						"id": "appstream",
						"name": "AppStream",
						"baseurl": ["https://example.com/appstream"],
						"metalink": null,
						"mirrorlist": null,
						"gpgcheck": false,
						"repo_gpgcheck": false,
						"gpgkey": null,
						"sslverify": null,
						"sslcacert": null,
						"sslclientkey": null,
						"sslclientcert": null,
						"metadata_expire": "",
						"module_hotfixes": null,
						"rhsm": false
					}
				}
			}`,
			wantPkgLen:  2,
			wantRepoLen: 1,
			wantSolver:  "dnf5",
		},
		{
			name: "empty packages",
			input: `{
				"solver": "dnf5",
				"packages": [],
				"repos": {}
			}`,
			wantPkgLen:  0,
			wantRepoLen: 0,
			wantSolver:  "dnf5",
		},
		{
			name:       "invalid JSON",
			input:      `{invalid json`,
			wantErrMsg: "decoding",
		},
		{
			name: "repo not found",
			input: `{
				"solver": "dnf5",
				"packages": [
					{
						"name": "foo",
						"epoch": 0,
						"version": "1.0",
						"release": "1.el9",
						"arch": "x86_64",
						"repo_id": "missing_repo",
						"location": "Packages/foo-1.0-1.el9.x86_64.rpm",
						"remote_locations": [],
						"checksum": null,
						"header_checksum": null,
						"license": "",
						"summary": "",
						"description": "",
						"url": "",
						"vendor": "",
						"packager": "",
						"build_time": "",
						"download_size": 0,
						"install_size": 0,
						"group": "",
						"source_rpm": "",
						"reason": "",
						"provides": [],
						"requires": [],
						"requires_pre": [],
						"conflicts": [],
						"obsoletes": [],
						"regular_requires": [],
						"recommends": [],
						"suggests": [],
						"enhances": [],
						"supplements": [],
						"files": []
					}
				],
				"repos": {}
			}`,
			wantErrMsg: "repo ID not found",
		},
	}

	v2Handler := newV2Handler()

	// Test both parsers with the same test cases
	parsers := []struct {
		name  string
		parse func([]byte) (pkgLen, repoLen int, solver string, err error)
	}{
		{
			name: "parseDumpResult",
			parse: func(b []byte) (int, int, string, error) {
				r, err := v2Handler.parseDumpResult(b)
				if err != nil {
					return 0, 0, "", err
				}
				return len(r.Packages), len(r.Repos), r.Solver, nil
			},
		},
		{
			name: "parseSearchResult",
			parse: func(b []byte) (int, int, string, error) {
				r, err := v2Handler.parseSearchResult(b)
				if err != nil {
					return 0, 0, "", err
				}
				return len(r.Packages), len(r.Repos), r.Solver, nil
			},
		},
	}

	for _, parser := range parsers {
		t.Run(parser.name, func(t *testing.T) {
			for _, tt := range testCases {
				t.Run(tt.name, func(t *testing.T) {
					pkgLen, repoLen, solver, err := parser.parse([]byte(tt.input))
					if tt.wantErrMsg != "" {
						require.Error(t, err)
						assert.Contains(t, err.Error(), tt.wantErrMsg)
						return
					}
					require.NoError(t, err)
					assert.Equal(t, tt.wantPkgLen, pkgLen)
					assert.Equal(t, tt.wantRepoLen, repoLen)
					assert.Equal(t, tt.wantSolver, solver)
				})
			}
		})
	}
}

func TestV2HandlerToRPMMDPackageWithMTLS(t *testing.T) {
	v2Handler := newV2Handler()

	repo := v2Repository{
		ID:            "mtls-repo",
		Name:          "MTLS Repo",
		SSLClientKey:  "/path/to/key",
		SSLClientCert: "/path/to/cert",
	}

	pkg := v2Package{
		Name:            "test-pkg",
		Epoch:           0,
		Version:         "1.0",
		Release:         "1.el9",
		Arch:            "x86_64",
		RepoID:          "mtls-repo",
		Location:        "Packages/test-pkg.rpm",
		RemoteLocations: []string{"https://example.com/test-pkg.rpm"},
		Checksum:        &v2Checksum{Algorithm: "sha256", Value: "aaaa"},
		Provides:        []v2Dependency{},
		Requires:        []v2Dependency{},
		RequiresPre:     []v2Dependency{},
		Conflicts:       []v2Dependency{},
		Obsoletes:       []v2Dependency{},
		RegularRequires: []v2Dependency{},
		Recommends:      []v2Dependency{},
		Suggests:        []v2Dependency{},
		Enhances:        []v2Dependency{},
		Supplements:     []v2Dependency{},
		Files:           []string{},
	}
	rpmmdRepo := v2Handler.toRPMMDRepoConfig(repo)
	rpmPkg, err := v2Handler.toRPMMDPackage(pkg, &rpmmdRepo)
	require.NoError(t, err)

	assert.Equal(t, "org.osbuild.mtls", rpmPkg.Secrets)
}

func TestV2HandlerToRPMMDRelDepList(t *testing.T) {
	v2Handler := newV2Handler()

	testCases := []struct {
		name     string
		deps     []v2Dependency
		expected rpmmd.RelDepList
	}{
		{
			name:     "empty list",
			deps:     []v2Dependency{},
			expected: nil,
		},
		{
			name:     "nil list",
			deps:     nil,
			expected: nil,
		},
		{
			name: "dependencies with relations",
			deps: []v2Dependency{
				{Name: "bash", Relation: ">=", Version: "5.0"},
				{Name: "glibc", Relation: "=", Version: "2.34"},
			},
			expected: rpmmd.RelDepList{
				{Name: "bash", Relationship: ">=", Version: "5.0"},
				{Name: "glibc", Relationship: "=", Version: "2.34"},
			},
		},
		{
			name: "dependencies without relations",
			deps: []v2Dependency{
				{Name: "libc.so.6()(64bit)"},
				{Name: "/bin/sh"},
			},
			expected: rpmmd.RelDepList{
				{Name: "libc.so.6()(64bit)"},
				{Name: "/bin/sh"},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result := v2Handler.toRPMMDRelDepList(tt.deps)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestV2HandlerParseDepsolveResultWithSBOM(t *testing.T) {
	v2Handler := newV2Handler()

	input := `{
		"solver": "dnf5",
		"transactions": [],
		"repos": {},
		"modules": {},
		"sbom": {"spdxVersion": "SPDX-2.3", "name": "test"}
	}`

	result, err := v2Handler.parseDepsolveResult([]byte(input))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.SBOMRaw)

	// Verify SBOM is preserved as raw JSON
	var sbomData map[string]interface{}
	err = json.Unmarshal(result.SBOMRaw, &sbomData)
	require.NoError(t, err)
	assert.Equal(t, "SPDX-2.3", sbomData["spdxVersion"])
	assert.Equal(t, "test", sbomData["name"])
}

func TestV2HandlerParseDepsolveResultWithModules(t *testing.T) {
	v2Handler := newV2Handler()

	input := `{
		"solver": "dnf5",
		"transactions": [],
		"repos": {},
		"modules": {
			"nodejs": {
				"module-file": {
					"path": "/etc/dnf/modules.d/nodejs.module",
					"data": {
						"name": "nodejs",
						"stream": "18",
						"profiles": ["default"],
						"state": "enabled"
					}
				},
				"failsafe-file": {
					"path": "/etc/dnf/modules.d/nodejs.failsafe",
					"data": "failsafe data"
				}
			}
		}
	}`

	result, err := v2Handler.parseDepsolveResult([]byte(input))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Modules, 1)

	mod := result.Modules[0]
	assert.Equal(t, "/etc/dnf/modules.d/nodejs.module", mod.ModuleConfigFile.Path)
	assert.Equal(t, "nodejs", mod.ModuleConfigFile.Data.Name)
	assert.Equal(t, "18", mod.ModuleConfigFile.Data.Stream)
	assert.Equal(t, []string{"default"}, mod.ModuleConfigFile.Data.Profiles)
	assert.Equal(t, "enabled", mod.ModuleConfigFile.Data.State)
	assert.Equal(t, "/etc/dnf/modules.d/nodejs.failsafe", mod.FailsafeFile.Path)
	assert.Equal(t, "failsafe data", mod.FailsafeFile.Data)
}

// TestV2HandlerParseDepsolveResultDetails verifies that parseDepsolveResult
// correctly parses all package and repository fields into rpmmd types.
func TestV2HandlerParseDepsolveResultDetails(t *testing.T) {
	v2Handler := newV2Handler()

	result, err := v2Handler.parseDepsolveResult([]byte(testParseDetailsInput))
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify structure
	require.Len(t, result.Transactions, 1)
	require.Len(t, result.Transactions[0], 1)
	require.Len(t, result.Repos, 1)
	assert.Equal(t, "dnf5", result.Solver)

	// Verify package - single assert compares all fields
	assert.Equal(t, testExpectedPackage, result.Transactions[0][0])

	// Verify repo - single assert compares all fields
	assert.Equal(t, testExpectedRepo, result.Repos[0])
}

// TestV2HandlerParseDumpSearchResultDetails verifies that parseDumpResult and
// parseSearchResult correctly parse all package and repository fields into rpmmd types.
func TestV2HandlerParseDumpSearchResultDetails(t *testing.T) {
	v2Handler := newV2Handler()

	// Test parseDumpResult
	t.Run("parseDumpResult", func(t *testing.T) {
		result, err := v2Handler.parseDumpResult([]byte(testParseDetailsDumpSearchInput))
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Packages, 1)
		require.Len(t, result.Repos, 1)
		assert.Equal(t, "dnf5", result.Solver)

		// Verify package - single assert compares all fields
		assert.Equal(t, testExpectedPackage, result.Packages[0])

		// Verify repo - single assert compares all fields
		assert.Equal(t, testExpectedRepo, result.Repos[0])
	})

	// Test parseSearchResult
	t.Run("parseSearchResult", func(t *testing.T) {
		result, err := v2Handler.parseSearchResult([]byte(testParseDetailsDumpSearchInput))
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Packages, 1)
		require.Len(t, result.Repos, 1)
		assert.Equal(t, "dnf5", result.Solver)

		// Verify package - single assert compares all fields
		assert.Equal(t, testExpectedPackage, result.Packages[0])

		// Verify repo - single assert compares all fields
		assert.Equal(t, testExpectedRepo, result.Repos[0])
	})
}

func TestV2HandlerToRPMMDRepoConfigs(t *testing.T) {
	h := newV2Handler()
	v2Repos := map[string]v2Repository{
		"repo-b": {ID: "repo-b", Name: "Repo B"},
		"repo-a": {ID: "repo-a", Name: "Repo A"},
	}

	repos, repoMap := h.toRPMMDRepoConfigs(v2Repos)

	// Check slice is sorted by ID
	require.Len(t, repos, 2)
	assert.Equal(t, "repo-a", repos[0].Id)
	assert.Equal(t, "repo-b", repos[1].Id)

	// Check map points to correct repos
	require.Len(t, repoMap, 2)
	assert.Equal(t, "Repo A", repoMap["repo-a"].Name)
	assert.Equal(t, "Repo B", repoMap["repo-b"].Name)
}
