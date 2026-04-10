package manifestmock_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/manifestgen/manifestmock"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

func TestResolveContainers_EmptyInpu(t *testing.T) {
	result := manifestmock.ResolveContainers(nil)
	assert.Equal(t, map[string][]container.Spec{}, result)

	result = manifestmock.ResolveContainers(map[string][]container.SourceSpec{})
	assert.Equal(t, map[string][]container.Spec{}, result)
}

func TestResolveContainers_Smoke(t *testing.T) {
	input := map[string][]container.SourceSpec{
		"build": {
			{
				Name:      "Build container",
				Source:    "ghcr.io/ondrejbudai/booc:fedora",
				TLSVerify: common.ToPtr(true),
			},
		},
	}
	result := manifestmock.ResolveContainers(input)
	assert.Equal(t, map[string][]container.Spec{
		"build": []container.Spec{
			{
				Source:     "ghcr.io/ondrejbudai/booc:fedora",
				Digest:     "sha256:df023f283afc154c1374e2335ea4a54a210f1cf0f8fe2af812c239a576577efa",
				ListDigest: "sha256:26c7349a68c3e90dd897c8ca1fff7097b274efc33fed56d858331de3bd01c9d8",
				TLSVerify:  common.ToPtr(true),
				ImageID:    "sha256:2c380abcfa442874be885a28e4f909600c24e5457374cd6671a4dbd74e28ffe7",
				LocalName:  "Build container",
			},
		},
	}, result)
}

func TestResolveCommits_EmptyInput(t *testing.T) {
	result := manifestmock.ResolveCommits(nil)
	assert.Equal(t, map[string][]ostree.CommitSpec{}, result)

	result = manifestmock.ResolveCommits(map[string][]ostree.SourceSpec{})
	assert.Equal(t, map[string][]ostree.CommitSpec{}, result)
}

func TestResolveCommits_Smoke(t *testing.T) {
	input := map[string][]ostree.SourceSpec{
		"pipeline1": {
			{
				Ref: "test/ref",
				URL: "https://example.com/repo",
			},
		},
	}
	result := manifestmock.ResolveCommits(input)
	assert.Equal(t, map[string][]ostree.CommitSpec{
		"pipeline1": []ostree.CommitSpec{
			{
				Ref:      "test/ref",
				URL:      "https://example.com/repo",
				Checksum: "b9b3034a43bf9c404fce8c7713f7e115a10a429d67afc55076b911878ca92615",
			},
		},
	}, result)
}

func TestDepsolve_EmptyInput(t *testing.T) {
	result, err := manifestmock.Depsolve(nil, "x86_64", nil, false)
	assert.NoError(t, err)
	assert.Equal(t, map[string]depsolvednf.DepsolveResult{}, result)

	result, err = manifestmock.Depsolve(map[string][]rpmmd.PackageSet{}, "x86_64", nil, false)
	assert.NoError(t, err)
	assert.Equal(t, map[string]depsolvednf.DepsolveResult{}, result)
}

func TestDepsolve_Smoke(t *testing.T) {
	baseosRepo := rpmmd.RepoConfig{
		Id:       "baseos",
		Name:     "baseos",
		BaseURLs: []string{"https://example.com/baseos"},
	}
	appstreamRepo := rpmmd.RepoConfig{
		Id:       "appstream",
		Name:     "appstream",
		BaseURLs: []string{"https://example.com/appstream"},
	}
	userRepo := rpmmd.RepoConfig{
		Id:       "user",
		Name:     "user",
		BaseURLs: []string{"https://example.com/user"},
	}
	baseRepos := []rpmmd.RepoConfig{appstreamRepo, baseosRepo}
	allRepos := slices.Concat(baseRepos, []rpmmd.RepoConfig{userRepo})

	packageSets := map[string][]rpmmd.PackageSet{
		"build": {
			{
				Include:         []string{"build-inc1", "dnf"},
				Exclude:         []string{"build-exc1"},
				Repositories:    baseRepos,
				InstallWeakDeps: true,
			},
		},
		"os": {
			{
				Include:         []string{"os-inc1", "dnf"},
				Exclude:         []string{"os-exc1"},
				Repositories:    baseRepos,
				InstallWeakDeps: true,
			},
			{
				Include:         []string{"os-inc2", "dnf"},
				Exclude:         []string{"os-exc2"},
				Repositories:    allRepos,
				InstallWeakDeps: false,
			},
		},
	}

	arch := "x86_64"
	result, err := manifestmock.Depsolve(packageSets, arch, nil, false)
	assert.NoError(t, err)
	assert.Equal(t, map[string]depsolvednf.DepsolveResult{
		"build": depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name:            "build-inc1",
						Version:         "1",
						Release:         "3.pkgset~build^trans~0",
						Arch:            "x86_64",
						Location:        "packages/build-inc1-0:1-3.pkgset~build^trans~0.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/baseos/packages/build-inc1-0:1-3.pkgset~build^trans~0.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "efd4f3331ab734995ccbd2fa9d16ee989b17b8160e7f79baf23118b19849c027"},
						RepoID:          baseosRepo.Id,
						Repo:            &baseosRepo,
					},
					{
						Name:            "dnf",
						Version:         "3",
						Release:         "7.pkgset~build^trans~0",
						Arch:            "x86_64",
						Location:        "packages/dnf-0:3-7.pkgset~build^trans~0.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/dnf-0:3-7.pkgset~build^trans~0.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "ba61ee16bedef613c9779ecc5d40be8304ece49473923c828082ab5c48dcf8ee"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
					{
						Name:            "exclude:build-exc1",
						Version:         "1",
						Release:         "8.pkgset~build^trans~0",
						Arch:            "x86_64",
						Location:        "packages/exclude:build-exc1-0:1-8.pkgset~build^trans~0.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/baseos/packages/exclude:build-exc1-0:1-8.pkgset~build^trans~0.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "75dacb0c543a006c4ba5f339dbbd2e53006a7f918079b4f5e5fdb683aa395ab7"},
						RepoID:          baseosRepo.Id,
						Repo:            &baseosRepo,
					},
					{
						Name:            "https://example.com/passed-arch:x86_64/passed-repo:/appstream",
						Version:         "2",
						Release:         "1.fk1",
						Arch:            "x86_64",
						Location:        "passed-arch:x86_64/passed-repo:/appstream",
						RemoteLocations: []string{"https://example.com/passed-arch:x86_64/passed-repo:/appstream"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "e795634c003b026414b3299574c7b847d9168280e8d2bbdbe3e881cbed194c9d"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
					{
						Name:            "https://example.com/passed-arch:x86_64/passed-repo:/baseos",
						Version:         "6",
						Release:         "0.fk1",
						Arch:            "x86_64",
						Location:        "passed-arch:x86_64/passed-repo:/baseos",
						RemoteLocations: []string{"https://example.com/passed-arch:x86_64/passed-repo:/baseos"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "3cdb35e9d0c01e60bc3362af2544e8b46429c3b1d5177579a0bec588ac65e707"},
						RepoID:          baseosRepo.Id,
						Repo:            &baseosRepo,
					},
					{
						Name:            "pkgset:build_trans:0_repos:appstream+baseos+weak",
						Version:         "2",
						Release:         "8.fk1",
						Arch:            "x86_64",
						Location:        "packages/pkgset:build_trans:0_repos:appstream+baseos+weak-0:2-8.fk1.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/pkgset:build_trans:0_repos:appstream+baseos+weak-0:2-8.fk1.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "856ccc87e01cd2c6ad3a72c6168f8f0b149e28e8bc1b9eb5e52e7d7a3e57e7d8"},
						RepoID:          baseRepos[0].Id,
						Repo:            &baseRepos[0],
						Files:           []string{"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release", "/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release"},
					},
				},
			},
			Repos: baseRepos,
		},
		"os": depsolvednf.DepsolveResult{
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name:            "dnf",
						Version:         "0",
						Release:         "5.pkgset~os^trans~0",
						Arch:            "x86_64",
						Location:        "packages/dnf-0:0-5.pkgset~os^trans~0.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/dnf-0:0-5.pkgset~os^trans~0.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "22c603fbf9285e567b76ea505f365da7d013cc48174b494e40a5032e153291ac"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
					{
						Name:            "exclude:os-exc1",
						Version:         "1",
						Release:         "5.pkgset~os^trans~0",
						Arch:            "x86_64",
						Location:        "packages/exclude:os-exc1-0:1-5.pkgset~os^trans~0.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/exclude:os-exc1-0:1-5.pkgset~os^trans~0.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "d292792edfe3ffd70bd96aae58368cf64607fd8ca5d989df23138bacba271a8d"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
					{
						Name:            "https://example.com/passed-arch:x86_64/passed-repo:/appstream",
						Version:         "2",
						Release:         "1.fk1",
						Arch:            "x86_64",
						Location:        "passed-arch:x86_64/passed-repo:/appstream",
						RemoteLocations: []string{"https://example.com/passed-arch:x86_64/passed-repo:/appstream"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "e795634c003b026414b3299574c7b847d9168280e8d2bbdbe3e881cbed194c9d"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
					{
						Name:            "https://example.com/passed-arch:x86_64/passed-repo:/baseos",
						Version:         "6",
						Release:         "0.fk1",
						Arch:            "x86_64",
						Location:        "passed-arch:x86_64/passed-repo:/baseos",
						RemoteLocations: []string{"https://example.com/passed-arch:x86_64/passed-repo:/baseos"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "3cdb35e9d0c01e60bc3362af2544e8b46429c3b1d5177579a0bec588ac65e707"},
						RepoID:          baseosRepo.Id,
						Repo:            &baseosRepo,
					},
					{
						Name:            "os-inc1",
						Version:         "3",
						Release:         "7.pkgset~os^trans~0",
						Arch:            "x86_64",
						Location:        "packages/os-inc1-0:3-7.pkgset~os^trans~0.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/os-inc1-0:3-7.pkgset~os^trans~0.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "540bb70a104b25dce75c51363eadbdb9c0ed0387467fa2ee28fec5c3103b7f0a"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
					{
						Name:            "pkgset:os_trans:0_repos:appstream+baseos+weak",
						Version:         "1",
						Release:         "2.fk1",
						Arch:            "x86_64",
						Location:        "packages/pkgset:os_trans:0_repos:appstream+baseos+weak-0:1-2.fk1.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/pkgset:os_trans:0_repos:appstream+baseos+weak-0:1-2.fk1.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "d87ebe584488771c23c9304d4c7f11f33eb67ab9aa0494b416e010dab3213bf9"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
						Files:           []string{"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release", "/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release"},
					},
				},
				{
					{
						Name:            "dnf",
						Version:         "3",
						Release:         "1.pkgset~os^trans~1",
						Arch:            "x86_64",
						Location:        "packages/dnf-0:3-1.pkgset~os^trans~1.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/baseos/packages/dnf-0:3-1.pkgset~os^trans~1.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "5d182a7a8683bbc8e3f64a2d9d9547de758e7ff9d98cdebdcac97c62d4912602"},
						RepoID:          baseosRepo.Id,
						Repo:            &baseosRepo,
					},
					{
						Name:            "exclude:os-exc2",
						Version:         "1",
						Release:         "2.pkgset~os^trans~1",
						Arch:            "x86_64",
						Location:        "packages/exclude:os-exc2-0:1-2.pkgset~os^trans~1.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/user/packages/exclude:os-exc2-0:1-2.pkgset~os^trans~1.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "780cae4f0a0ccba5ba6ecb647e03d7dd1d38cd1c5843e67b00432d5c478d6018"},
						RepoID:          userRepo.Id,
						Repo:            &userRepo,
					},
					{
						Name:            "https://example.com/passed-arch:x86_64/passed-repo:/user",
						Version:         "3",
						Release:         "0.fk1",
						Arch:            "x86_64",
						Location:        "passed-arch:x86_64/passed-repo:/user",
						RemoteLocations: []string{"https://example.com/passed-arch:x86_64/passed-repo:/user"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "0c676fb4e895762dc80cd8abadec73e49970317f281ad5b94b93df9afdcad4e9"},
						RepoID:          userRepo.Id,
						Repo:            &userRepo,
					},
					{
						Name:            "os-inc2",
						Version:         "2",
						Release:         "0.pkgset~os^trans~1",
						Arch:            "x86_64",
						Location:        "packages/os-inc2-0:2-0.pkgset~os^trans~1.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/user/packages/os-inc2-0:2-0.pkgset~os^trans~1.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "fc429287a10941ecc73ad362fa2e0cf97226613de5206025f92f6fd5da24fd73"},
						RepoID:          userRepo.Id,
						Repo:            &userRepo,
					},
					{
						Name:            "pkgset:os_trans:1_repos:appstream+baseos+user",
						Version:         "2",
						Release:         "7.fk1",
						Arch:            "x86_64",
						Location:        "packages/pkgset:os_trans:1_repos:appstream+baseos+user-0:2-7.fk1.x86_64.rpm",
						RemoteLocations: []string{"https://example.com/appstream/packages/pkgset:os_trans:1_repos:appstream+baseos+user-0:2-7.fk1.x86_64.rpm"},
						Checksum:        rpmmd.Checksum{Type: "sha256", Value: "84f3c5f75b9d53fbb9bec083bf3b149daae1daa750ccf3b75d984f9f32510218"},
						RepoID:          appstreamRepo.Id,
						Repo:            &appstreamRepo,
					},
				},
			},
			Repos: allRepos,
		},
	}, result)
}
