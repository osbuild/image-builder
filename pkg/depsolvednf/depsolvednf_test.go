package depsolvednf

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/mocks/rpmrepo"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var forceDNF = flag.Bool("force-dnf", false, "force dnf testing, making them fail instead of skip if dnf isn't installed")

// testHandler holds an API handler with its name for test iteration.
type testHandler struct {
	name                 string
	handler              apiHandler
	assertDepsolveResult func(t *testing.T, pkgSets []rpmmd.PackageSet, actual DepsolveResult)
}

// getTestHandlers returns the list of handlers to test against.
func getTestHandlers() []testHandler {
	return []testHandler{
		{name: "V2", handler: newV2Handler(), assertDepsolveResult: assertDepsolveResultV2},
	}
}

// mockActiveHandler replaces the activeHandler with the given handler
// and returns a function to restore the original handler.
func mockActiveHandler(h apiHandler) func() {
	original := activeHandler
	activeHandler = h
	return func() {
		activeHandler = original
	}
}

// assertPackagesMatchCore compares two packages by the original core fields only.
// This is needed because newer versions of the depsolver API return much more
// metadata than the original core fields, but the newer API is a superset of
// the original core fields.
func assertPackagesMatchCore(t *testing.T, expected, actual rpmmd.Package) {
	t.Helper()
	// Values set by the original core fields
	assert.Equal(t, expected.Name, actual.Name, "Name mismatch")
	assert.Equal(t, expected.Epoch, actual.Epoch, "Epoch mismatch")
	assert.Equal(t, expected.Version, actual.Version, "Version mismatch")
	assert.Equal(t, expected.Release, actual.Release, "Release mismatch")
	assert.Equal(t, expected.Arch, actual.Arch, "Arch mismatch")
	assert.Equal(t, expected.RepoID, actual.RepoID, "RepoID mismatch")
	assert.Equal(t, expected.Location, actual.Location, "Location mismatch")
	assert.Equal(t, expected.RemoteLocations, actual.RemoteLocations, "RemoteLocations mismatch")
	assert.Equal(t, expected.Checksum, actual.Checksum, "Checksum mismatch")
	// Values set by the Solver from the Repository metadata
	assert.Equal(t, expected.CheckGPG, actual.CheckGPG, "CheckGPG mismatch")
	assert.Equal(t, expected.IgnoreSSL, actual.IgnoreSSL, "IgnoreSSL mismatch")
}

// assertDepsolveResultV2 checks the expected packages against the actual packages for the V2 API result.
func assertDepsolveResultV2(t *testing.T, pkgSets []rpmmd.PackageSet, actual DepsolveResult) {
	t.Helper()

	require.Equal(t, 1, len(actual.Repos), "expected exactly 1 repo")
	expectedPackages := expectedDepsolvedPackages(actual.Repos[0])

	// V2 specific tests below

	// Transaction count must match number of package sets
	require.Equal(t, len(pkgSets), len(actual.Transactions), "transaction count mismatch")

	// Each requested package must be in the correct transaction
	for i, pkgSet := range pkgSets {
		for _, reqPkg := range pkgSet.Include {
			assert.True(t, slices.ContainsFunc(actual.Transactions[i], func(p rpmmd.Package) bool {
				return p.Name == reqPkg
			}), "requested package %q not found in transaction %d", reqPkg, i)
		}
	}

	// Union of all Transactions must equal expected packages
	allTransactionPkgs := actual.Transactions.AllPackages()
	require.Equal(t, len(expectedPackages), len(allTransactionPkgs), "transaction packages count mismatch")

	// Check full metadata for all packages
	for i := range expectedPackages {
		// Check full metadata for bash package as a smoke test
		if expectedPackages[i].Name == "bash" {
			assert.Equal(t, expectedPackages[i], allTransactionPkgs[i], "full bash metadata mismatch")
			continue
		}

		// NOTE: We don't compare the full metadata here, because V2 returns more metadata than what we define in our
		// test packages metadata. We don't include the full metadata for all packages to keep this source file
		// reasonably small.
		assertPackagesMatchCore(t, expectedPackages[i], allTransactionPkgs[i])

		// Sanity test that additional fields that should never be empty for any package are not empty
		assert.NotEmpty(t, allTransactionPkgs[i].Group)
		assert.Greater(t, allTransactionPkgs[i].DownloadSize, uint64(0))
		assert.NotEmpty(t, allTransactionPkgs[i].License)
		assert.NotEmpty(t, allTransactionPkgs[i].SourceRpm)
		assert.NotEmpty(t, allTransactionPkgs[i].BuildTime)
		assert.NotEmpty(t, allTransactionPkgs[i].Packager)
		assert.NotEmpty(t, allTransactionPkgs[i].Vendor)
		assert.NotEmpty(t, allTransactionPkgs[i].Summary)
		assert.NotEmpty(t, allTransactionPkgs[i].Description)
		assert.NotEmpty(t, allTransactionPkgs[i].Provides)
	}
}

func requireDNF(t *testing.T) {
	t.Helper()
	if !*forceDNF {
		// dnf tests aren't forced: skip them if the dnf sniff check fails
		if findDepsolveDnf() == "" {
			t.Skip("Test needs an installed osbuild-depsolve-dnf")
		}
	}
}

func newTestSolver(t *testing.T) *Solver {
	t.Helper()
	tmpdir := t.TempDir()
	solver := NewSolver("platform:el9", "9", "x86_64", "centos-stream-9", tmpdir)
	return solver
}

func TestSolverDepsolve(t *testing.T) {
	requireDNF(t)

	s := rpmrepo.NewTestServer()
	defer s.Close()

	type testCase struct {
		packages [][]string
		repos    []rpmmd.RepoConfig
		rootDir  string
		sbomType sbom.StandardType
		err      bool
		expMsg   string
	}

	rootDir := t.TempDir()
	reposDir := filepath.Join(rootDir, "etc", "yum.repos.d")
	require.NoError(t, os.MkdirAll(reposDir, 0777))
	s.WriteConfig(filepath.Join(reposDir, "test.repo"))

	testCases := map[string]testCase{
		"flat": {
			packages: [][]string{{"kernel", "vim-minimal", "tmux", "zsh"}},
			repos:    []rpmmd.RepoConfig{s.RepoConfig},
			err:      false,
		},
		"chain": {
			// chain depsolve of the same packages in order should produce the same result (at least in this case)
			packages: [][]string{{"kernel"}, {"vim-minimal", "tmux", "zsh"}},
			repos:    []rpmmd.RepoConfig{s.RepoConfig},
			err:      false,
		},
		"bad-flat": {
			packages: [][]string{{"this-package-does-not-exist"}},
			repos:    []rpmmd.RepoConfig{s.RepoConfig},
			err:      true,
			expMsg:   "this-package-does-not-exist",
		},
		"bad-chain": {
			packages: [][]string{{"kernel"}, {"this-package-does-not-exist"}},
			repos:    []rpmmd.RepoConfig{s.RepoConfig},
			err:      true,
			expMsg:   "this-package-does-not-exist",
		},
		"bad-chain-part-deux": {
			packages: [][]string{{"this-package-does-not-exist"}, {"vim-minimal", "tmux", "zsh"}},
			repos:    []rpmmd.RepoConfig{s.RepoConfig},
			err:      true,
			expMsg:   "this-package-does-not-exist",
		},
		"flat+dir": {
			packages: [][]string{{"kernel", "vim-minimal", "tmux", "zsh"}},
			rootDir:  rootDir,
			err:      false,
		},
		"chain+dir": {
			packages: [][]string{{"kernel"}, {"vim-minimal", "tmux", "zsh"}},
			rootDir:  rootDir,
			err:      false,
		},
		"bad-flat+dir": {
			packages: [][]string{{"this-package-does-not-exist"}},
			rootDir:  rootDir,
			err:      true,
			expMsg:   "this-package-does-not-exist",
		},
		"bad-chain+dir": {
			packages: [][]string{{"kernel"}, {"this-package-does-not-exist"}},
			rootDir:  rootDir,
			err:      true,
			expMsg:   "this-package-does-not-exist",
		},
		"bad-chain-part-deux+dir": {
			packages: [][]string{{"this-package-does-not-exist"}, {"vim-minimal", "tmux", "zsh"}},
			rootDir:  rootDir,
			err:      true,
			expMsg:   "this-package-does-not-exist",
		},
		"chain-with-sbom": {
			// chain depsolve of the same packages in order should produce the same result (at least in this case)
			packages: [][]string{{"kernel"}, {"vim-minimal", "tmux", "zsh"}},
			repos:    []rpmmd.RepoConfig{s.RepoConfig},
			sbomType: sbom.StandardTypeSpdx,
			err:      false,
		},
	}

	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			for tcName, tc := range testCases {
				t.Run(tcName, func(t *testing.T) {
					assert := assert.New(t)
					pkgsets := make([]rpmmd.PackageSet, len(tc.packages))
					for idx := range tc.packages {
						pkgsets[idx] = rpmmd.PackageSet{Include: tc.packages[idx], Repositories: tc.repos, InstallWeakDeps: true}
					}

					solver := newTestSolver(t)
					solver.SetRootDir(tc.rootDir)
					actualResult, err := solver.Depsolve(pkgsets, tc.sbomType)
					if tc.err {
						assert.Error(err)
						assert.Contains(err.Error(), tc.expMsg)
						return
					} else {
						assert.Nil(err)
						require.NotNil(t, actualResult)
					}

					h.assertDepsolveResult(t, pkgsets, *actualResult)

					// NOTE: The SBOM document is not stable due to UUIDs, so we need to take it from the result
					if tc.sbomType != sbom.StandardTypeNone {
						require.NotNil(t, actualResult.SBOM)
						assert.Equal(sbom.StandardTypeSpdx, actualResult.SBOM.DocType)
						assert.NotEmpty(actualResult.SBOM.Document)
					} else {
						assert.Nil(actualResult.SBOM)
					}
				})
			}
		})
	}
}

func TestSolverDepsolveAll(t *testing.T) {
	requireDNF(t)

	s := rpmrepo.NewTestServer()
	defer s.Close()

	type testCase struct {
		packageSets map[string][]rpmmd.PackageSet
		sbomType    sbom.StandardType
		// slice of message fragments which are all expected to be in the error
		expErrs []string
	}

	tmpdir := t.TempDir()

	testCases := map[string]testCase{
		"flat": {
			packageSets: map[string][]rpmmd.PackageSet{
				"first": {
					{
						Include:         []string{"kernel", "vim-minimal", "tmux", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
			},
		},
		"chain": {
			// chain depsolve of the same packages in order should produce the same result (at least in this case)
			packageSets: map[string][]rpmmd.PackageSet{
				"first": {
					{
						Include:         []string{"kernel"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "tmux", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
			},
		},
		"multi-chain": {
			// two chain depsolves for different pipelines
			packageSets: map[string][]rpmmd.PackageSet{
				"first": {
					{
						Include:         []string{"kernel", "tmux"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
				"second": {
					{
						Include:         []string{"kernel"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "tmux", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
			},
		},
		"multi-chain-error-first": {
			// two chain depsolves for different pipelines, first one fails
			packageSets: map[string][]rpmmd.PackageSet{
				"first": {
					{
						Include:         []string{"does-not-exist"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
				"second": {
					{
						Include:         []string{"kernel", "tmux"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "tmux", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
			},
			expErrs: []string{"error depsolving package sets for \"first\"", "missing packages: does-not-exist"},
		},
		"multi-chain-error-second": {
			// two chain depsolves for different pipelines, second one fails
			packageSets: map[string][]rpmmd.PackageSet{
				"first": {
					{
						Include:         []string{"kernel", "tmux"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
				"second": {
					{
						Include:         []string{"does-not-exist"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "tmux", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
			},
			expErrs: []string{"error depsolving package sets for \"second\"", "missing packages: does-not-exist"},
		},
		"multi-chain-with-sbom": {
			// two chain depsolves for different pipelines with sbom
			packageSets: map[string][]rpmmd.PackageSet{
				"first": {
					{
						Include:         []string{"kernel", "tmux"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
				"second": {
					{
						Include:         []string{"kernel"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
					{
						Include:         []string{"vim-minimal", "tmux", "zsh"},
						Repositories:    []rpmmd.RepoConfig{s.RepoConfig},
						InstallWeakDeps: true,
					},
				},
			},
			sbomType: sbom.StandardTypeSpdx,
		},
	}

	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			for tcName := range testCases {
				t.Run(tcName, func(t *testing.T) {
					assert := assert.New(t)
					tc := testCases[tcName]

					solver := NewSolver("platform:el9", "9", "x86_64", "rhel9.0", tmpdir)
					solver.SetSBOMType(tc.sbomType)
					res, err := solver.DepsolveAll(tc.packageSets)
					if len(tc.expErrs) != 0 {
						assert.Error(err)
						for _, expErr := range tc.expErrs {
							assert.Contains(err.Error(), expErr)
						}
						return
					} else {
						assert.Nil(err)
						require.NotNil(t, res)
					}

					assert.Equal(len(res), len(tc.packageSets))

					for pipelineName := range tc.packageSets {
						pipelineResult := res[pipelineName]
						assert.NotNil(pipelineResult)

						h.assertDepsolveResult(t, tc.packageSets[pipelineName], pipelineResult)

						if tc.sbomType != sbom.StandardTypeNone {
							require.NotNil(t, pipelineResult.SBOM)
							assert.Equal(sbom.StandardTypeSpdx, pipelineResult.SBOM.DocType)
							assert.NotEmpty(pipelineResult.SBOM.Document)
						} else {
							assert.Nil(pipelineResult.SBOM)
						}
					}
				})
			}
		})
	}
}

func TestSolverFetchMetadata(t *testing.T) {
	requireDNF(t)
	repoServer := rpmrepo.NewTestServer()
	defer repoServer.Close()

	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			solver := newTestSolver(t)

			res, err := solver.FetchMetadata([]rpmmd.RepoConfig{repoServer.RepoConfig})
			require.NoError(t, err)
			require.NotNil(t, res)
			// 1125 is the number of packages in the test repository (internal/mocks/rpmrepo)
			require.Equal(t, 1125, len(res))
			// ensure that the packages are sorted by full NEVRA
			require.Truef(t, sort.SliceIsSorted(res, func(i, j int) bool {
				return res[i].NVR() < res[j].NVR()
			}), "packages are not sorted by NVR")
		})
	}
}

func TestSolverSearchMetadata(t *testing.T) {
	requireDNF(t)

	testCases := []struct {
		name     string
		packages []string
		expNVRs  []string
	}{
		{
			name:     "single package",
			packages: []string{"zsh"},
			expNVRs:  []string{"zsh-5.8-7.el9"},
		},
		{
			name:     "multiple packages",
			packages: []string{"zsh", "bash"},
			expNVRs:  []string{"bash-5.1.8-2.el9", "zsh-5.8-7.el9"},
		},
		{
			name:     "single package with wildcard",
			packages: []string{"zsh*"},
			expNVRs:  []string{"zsh-5.8-7.el9"},
		},
		{
			name:     "multiple packages with wildcard",
			packages: []string{"zsh*", "bash*"},
			expNVRs:  []string{"bash-5.1.8-2.el9", "bash-completion-2.11-4.el9", "zsh-5.8-7.el9"},
		},
	}

	s := rpmrepo.NewTestServer()
	defer s.Close()

	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			solver := newTestSolver(t)

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					res, err := solver.SearchMetadata([]rpmmd.RepoConfig{s.RepoConfig}, tc.packages)
					require.NoError(t, err)
					require.NotNil(t, res)
					require.Equal(t, len(tc.expNVRs), len(res))
					require.Truef(t, sort.SliceIsSorted(res, func(i, j int) bool {
						return res[i].NVR() < res[j].NVR()
					}), "packages are not sorted by NVR")
					for i, pkg := range res {
						require.Equal(t, tc.expNVRs[i], pkg.NVR())
					}
				})
			}
		})
	}
}

func TestValidatePackageSetRepoChain(t *testing.T) {
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

	testCases := []struct {
		name    string
		pkgSets []rpmmd.PackageSet
		errMsg  string
	}{
		{
			name: "happy path - single transaction",
			pkgSets: []rpmmd.PackageSet{
				{Include: []string{"pkg1"}, Repositories: []rpmmd.RepoConfig{baseOS}},
			},
		},
		{
			name: "happy path",
			pkgSets: []rpmmd.PackageSet{
				{Include: []string{"pkg1"}, Repositories: []rpmmd.RepoConfig{baseOS}},
				{Include: []string{"pkg2"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream}},
				{Include: []string{"pkg3"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo}},
			},
		},
		{
			name: "Error: 3 transactions + 3rd one not using repo used by 2nd",
			pkgSets: []rpmmd.PackageSet{
				{Include: []string{"pkg1"}, Repositories: []rpmmd.RepoConfig{baseOS}},
				{Include: []string{"pkg2"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo}},
				{Include: []string{"pkg3"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo2}},
			},
			errMsg: "chained packageSet 2 does not use all of the repos used by its predecessor",
		},
		{
			name: "Error: 3 transactions but last one doesn't specify user repos in 2nd",
			pkgSets: []rpmmd.PackageSet{
				{Include: []string{"pkg1"}, Repositories: []rpmmd.RepoConfig{baseOS}},
				{Include: []string{"pkg2"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo}},
				{Include: []string{"pkg3"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream}},
			},
			errMsg: "chained packageSet 2 does not use all of the repos used by its predecessor",
		},
		{
			name: "Error: single transaction with empty Include",
			pkgSets: []rpmmd.PackageSet{
				{Include: []string{}, Repositories: []rpmmd.RepoConfig{baseOS}},
			},
			errMsg: "packageSet 0 has empty Include list",
		},
		{
			name: "Error: second transaction with empty Include in chain",
			pkgSets: []rpmmd.PackageSet{
				{Include: []string{"pkg1"}, Repositories: []rpmmd.RepoConfig{baseOS}},
				{Include: []string{}, Repositories: []rpmmd.RepoConfig{baseOS, appstream}},
				{Include: []string{"pkg3"}, Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo}},
			},
			errMsg: "packageSet 1 has empty Include list",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePackageSetRepoChain(tc.pkgSets)
			if tc.errMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestApplyRHSMSecrets(t *testing.T) {
	// Create repos with known hashes for testing
	rhsmRepo := rpmmd.RepoConfig{
		Name:          "rhsm-repo",
		BaseURLs:      []string{"https://cdn.redhat.com/content/"},
		RHSM:          true,
		SSLCACert:     "/etc/rhsm/ca/redhat-uep.pem",
		SSLClientCert: "/etc/pki/entitlement/123.pem",
		SSLClientKey:  "/etc/pki/entitlement/123-key.pem",
	}
	mtlsRepo := rpmmd.RepoConfig{
		Name:          "mtls-repo",
		BaseURLs:      []string{"https://example.com/mtls/"},
		RHSM:          false,
		SSLCACert:     "/etc/pki/mtls-repo/cacert",
		SSLClientCert: "/etc/pki/mtls-repo/cert",
		SSLClientKey:  "/etc/pki/mtls-repo/key",
	}
	plainRepo := rpmmd.RepoConfig{
		Name:     "plain-repo",
		BaseURLs: []string{"https://example.com/plain/"},
		RHSM:     false,
	}

	testCases := []struct {
		name         string
		packages     rpmmd.PackageList
		repos        []rpmmd.RepoConfig
		wantPackages rpmmd.PackageList
	}{
		{
			name: "RHSM repo overrides mtls to rhsm",
			packages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: rhsmRepo.Hash(), Secrets: "org.osbuild.mtls"},
			},
			repos: []rpmmd.RepoConfig{rhsmRepo},
			wantPackages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: rhsmRepo.Hash(), Secrets: "org.osbuild.rhsm"},
			},
		},
		{
			name: "non-RHSM repo keeps mtls secrets",
			packages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: mtlsRepo.Hash(), Secrets: "org.osbuild.mtls"},
			},
			repos: []rpmmd.RepoConfig{mtlsRepo},
			wantPackages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: mtlsRepo.Hash(), Secrets: "org.osbuild.mtls"},
			},
		},
		{
			name: "package from unknown repo keeps original secrets",
			packages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: "unknown-repo-hash", Secrets: "org.osbuild.mtls"},
			},
			repos: []rpmmd.RepoConfig{},
			wantPackages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: "unknown-repo-hash", Secrets: "org.osbuild.mtls"},
			},
		},
		{
			name: "package without secrets stays empty for non-RHSM repo",
			packages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: plainRepo.Hash(), Secrets: ""},
			},
			repos: []rpmmd.RepoConfig{plainRepo},
			wantPackages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: plainRepo.Hash(), Secrets: ""},
			},
		},
		{
			name: "RHSM repo sets secrets even if originally empty",
			packages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: rhsmRepo.Hash(), Secrets: ""},
			},
			repos: []rpmmd.RepoConfig{rhsmRepo},
			wantPackages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: rhsmRepo.Hash(), Secrets: "org.osbuild.rhsm"},
			},
		},
		{
			name: "mixed repos - only RHSM ones get overridden",
			packages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: rhsmRepo.Hash(), Secrets: "org.osbuild.mtls"},
				{Name: "pkg2", RepoID: mtlsRepo.Hash(), Secrets: "org.osbuild.mtls"},
				{Name: "pkg3", RepoID: plainRepo.Hash(), Secrets: ""},
			},
			repos: []rpmmd.RepoConfig{rhsmRepo, mtlsRepo, plainRepo},
			wantPackages: rpmmd.PackageList{
				{Name: "pkg1", RepoID: rhsmRepo.Hash(), Secrets: "org.osbuild.rhsm"},
				{Name: "pkg2", RepoID: mtlsRepo.Hash(), Secrets: "org.osbuild.mtls"},
				{Name: "pkg3", RepoID: plainRepo.Hash(), Secrets: ""},
			},
		},
		{
			name:         "empty package list",
			packages:     rpmmd.PackageList{},
			repos:        []rpmmd.RepoConfig{rhsmRepo},
			wantPackages: rpmmd.PackageList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			applyRHSMSecrets(tc.packages, tc.repos)

			require.Equal(t, len(tc.wantPackages), len(tc.packages))
			assert.Equal(t, tc.wantPackages, tc.packages)
		})
	}
}

// expectedDepsolvedPackages returns expected expected depsolved packages metadata.
// Repo-specific values (RemoteLocations, RepoID, etc.) are applied at assertion time.
func expectedDepsolvedPackages(repo rpmmd.RepoConfig) rpmmd.PackageList {
	expectedTemplate := rpmmd.PackageList{
		{Name: "acl", Epoch: 0, Version: "2.3.1", Release: "3.el9", Arch: "x86_64", Location: "Packages/acl-2.3.1-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "986044c3837eddbc9231d7be5e5fc517e245296978b988a803bc9f9172fe84ea"}},
		{Name: "alternatives", Epoch: 0, Version: "1.20", Release: "2.el9", Arch: "x86_64", Location: "Packages/alternatives-1.20-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db"}},
		{Name: "audit-libs", Epoch: 0, Version: "3.0.7", Release: "100.el9", Arch: "x86_64", Location: "Packages/audit-libs-3.0.7-100.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "a4bdda48abaedffeb74398cd55afbd00cb4153ae24bd2a3e6de9d87462df5ffa"}},
		{Name: "basesystem", Epoch: 0, Version: "11", Release: "13.el9", Arch: "noarch", Location: "Packages/basesystem-11-13.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973"}},
		{
			Name:         "bash",
			Epoch:        0,
			Version:      "5.1.8",
			Release:      "2.el9",
			Arch:         "x86_64",
			Group:        "Unspecified",
			DownloadSize: 1772315,
			InstallSize:  7739748,
			License:      "GPLv3+",
			SourceRpm:    "bash-5.1.8-2.el9.src.rpm",
			BuildTime:    time.Date(2021, time.August, 9, 19, 33, 6, 0, time.UTC),
			Packager:     "builder@centos.org",
			Vendor:       "CentOS",
			URL:          "https://www.gnu.org/software/bash",
			Summary:      "The GNU Bourne Again shell",
			Description:  "The GNU Bourne Again shell (Bash) is a shell or command language\ninterpreter that is compatible with the Bourne shell (sh). Bash\nincorporates useful features from the Korn shell (ksh) and the C shell\n(csh). Most sh scripts can be run by bash without modification.",
			Provides: rpmmd.RelDepList{
				{Name: "/bin/bash"},
				{Name: "/bin/sh"},
				{Name: "bash", Relationship: "=", Version: "5.1.8-2.el9"},
				{Name: "bash(x86-64)", Relationship: "=", Version: "5.1.8-2.el9"},
				{Name: "config(bash)", Relationship: "=", Version: "5.1.8-2.el9"},
			},
			Requires: rpmmd.RelDepList{
				{Name: "filesystem", Relationship: ">=", Version: "3"},
				{Name: "libc.so.6(GLIBC_2.34)(64bit)"},
				{Name: "libtinfo.so.6()(64bit)"},
				{Name: "rtld(GNU_HASH)"},
			},
			RegularRequires: rpmmd.RelDepList{
				{Name: "filesystem", Relationship: ">=", Version: "3"},
				{Name: "libc.so.6(GLIBC_2.34)(64bit)"},
				{Name: "libtinfo.so.6()(64bit)"},
				{Name: "rtld(GNU_HASH)"},
			},
			Files:    []string{"/etc/skel/.bash_logout", "/etc/skel/.bash_profile", "/etc/skel/.bashrc", "/usr/bin/alias", "/usr/bin/bash", "/usr/bin/bashbug", "/usr/bin/bashbug-64", "/usr/bin/bg", "/usr/bin/cd", "/usr/bin/command", "/usr/bin/fc", "/usr/bin/fg", "/usr/bin/getopts", "/usr/bin/hash", "/usr/bin/jobs", "/usr/bin/read", "/usr/bin/sh", "/usr/bin/type", "/usr/bin/ulimit", "/usr/bin/umask", "/usr/bin/unalias", "/usr/bin/wait", "/usr/lib/.build-id", "/usr/lib/.build-id/41", "/usr/lib/.build-id/41/61a572a65bba4bbed7ba5a976bedceeb471435", "/usr/share/doc/bash", "/usr/share/doc/bash/FAQ", "/usr/share/doc/bash/INTRO", "/usr/share/doc/bash/RBASH", "/usr/share/doc/bash/README", "/usr/share/doc/bash/bash.html", "/usr/share/doc/bash/bashref.html", "/usr/share/info/bash.info.gz", "/usr/share/licenses/bash", "/usr/share/licenses/bash/COPYING", "/usr/share/locale/af/LC_MESSAGES/bash.mo", "/usr/share/locale/bg/LC_MESSAGES/bash.mo", "/usr/share/locale/ca/LC_MESSAGES/bash.mo", "/usr/share/locale/cs/LC_MESSAGES/bash.mo", "/usr/share/locale/da/LC_MESSAGES/bash.mo", "/usr/share/locale/de/LC_MESSAGES/bash.mo", "/usr/share/locale/el/LC_MESSAGES/bash.mo", "/usr/share/locale/en@boldquot/LC_MESSAGES/bash.mo", "/usr/share/locale/en@quot/LC_MESSAGES/bash.mo", "/usr/share/locale/eo/LC_MESSAGES/bash.mo", "/usr/share/locale/es/LC_MESSAGES/bash.mo", "/usr/share/locale/et/LC_MESSAGES/bash.mo", "/usr/share/locale/fi/LC_MESSAGES/bash.mo", "/usr/share/locale/fr/LC_MESSAGES/bash.mo", "/usr/share/locale/ga/LC_MESSAGES/bash.mo", "/usr/share/locale/gl/LC_MESSAGES/bash.mo", "/usr/share/locale/hr/LC_MESSAGES/bash.mo", "/usr/share/locale/hu/LC_MESSAGES/bash.mo", "/usr/share/locale/id/LC_MESSAGES/bash.mo", "/usr/share/locale/it/LC_MESSAGES/bash.mo", "/usr/share/locale/ja/LC_MESSAGES/bash.mo", "/usr/share/locale/ko/LC_MESSAGES/bash.mo", "/usr/share/locale/lt/LC_MESSAGES/bash.mo", "/usr/share/locale/nb/LC_MESSAGES/bash.mo", "/usr/share/locale/nl/LC_MESSAGES/bash.mo", "/usr/share/locale/pl/LC_MESSAGES/bash.mo", "/usr/share/locale/pt/LC_MESSAGES/bash.mo", "/usr/share/locale/pt_BR/LC_MESSAGES/bash.mo", "/usr/share/locale/ro/LC_MESSAGES/bash.mo", "/usr/share/locale/ru/LC_MESSAGES/bash.mo", "/usr/share/locale/sk/LC_MESSAGES/bash.mo", "/usr/share/locale/sl/LC_MESSAGES/bash.mo", "/usr/share/locale/sr/LC_MESSAGES/bash.mo", "/usr/share/locale/sv/LC_MESSAGES/bash.mo", "/usr/share/locale/tr/LC_MESSAGES/bash.mo", "/usr/share/locale/uk/LC_MESSAGES/bash.mo", "/usr/share/locale/vi/LC_MESSAGES/bash.mo", "/usr/share/locale/zh_CN/LC_MESSAGES/bash.mo", "/usr/share/locale/zh_TW/LC_MESSAGES/bash.mo", "/usr/share/man/man1/..1.gz", "/usr/share/man/man1/:.1.gz", "/usr/share/man/man1/[.1.gz", "/usr/share/man/man1/alias.1.gz", "/usr/share/man/man1/bash.1.gz", "/usr/share/man/man1/bashbug-64.1.gz", "/usr/share/man/man1/bashbug.1.gz", "/usr/share/man/man1/bg.1.gz", "/usr/share/man/man1/bind.1.gz", "/usr/share/man/man1/break.1.gz", "/usr/share/man/man1/builtin.1.gz", "/usr/share/man/man1/builtins.1.gz", "/usr/share/man/man1/caller.1.gz", "/usr/share/man/man1/cd.1.gz", "/usr/share/man/man1/command.1.gz", "/usr/share/man/man1/compgen.1.gz", "/usr/share/man/man1/complete.1.gz", "/usr/share/man/man1/compopt.1.gz", "/usr/share/man/man1/continue.1.gz", "/usr/share/man/man1/declare.1.gz", "/usr/share/man/man1/dirs.1.gz", "/usr/share/man/man1/disown.1.gz", "/usr/share/man/man1/enable.1.gz", "/usr/share/man/man1/eval.1.gz", "/usr/share/man/man1/exec.1.gz", "/usr/share/man/man1/exit.1.gz", "/usr/share/man/man1/export.1.gz", "/usr/share/man/man1/fc.1.gz", "/usr/share/man/man1/fg.1.gz", "/usr/share/man/man1/getopts.1.gz", "/usr/share/man/man1/hash.1.gz", "/usr/share/man/man1/help.1.gz", "/usr/share/man/man1/history.1.gz", "/usr/share/man/man1/jobs.1.gz", "/usr/share/man/man1/let.1.gz", "/usr/share/man/man1/local.1.gz", "/usr/share/man/man1/logout.1.gz", "/usr/share/man/man1/mapfile.1.gz", "/usr/share/man/man1/popd.1.gz", "/usr/share/man/man1/pushd.1.gz", "/usr/share/man/man1/read.1.gz", "/usr/share/man/man1/readonly.1.gz", "/usr/share/man/man1/return.1.gz", "/usr/share/man/man1/set.1.gz", "/usr/share/man/man1/sh.1.gz", "/usr/share/man/man1/shift.1.gz", "/usr/share/man/man1/shopt.1.gz", "/usr/share/man/man1/source.1.gz", "/usr/share/man/man1/suspend.1.gz", "/usr/share/man/man1/times.1.gz", "/usr/share/man/man1/trap.1.gz", "/usr/share/man/man1/type.1.gz", "/usr/share/man/man1/typeset.1.gz", "/usr/share/man/man1/ulimit.1.gz", "/usr/share/man/man1/umask.1.gz", "/usr/share/man/man1/unalias.1.gz", "/usr/share/man/man1/unset.1.gz", "/usr/share/man/man1/wait.1.gz"},
			Location: "Packages/bash-5.1.8-2.el9.x86_64.rpm",
			Checksum: rpmmd.Checksum{Type: "sha256", Value: "3d45552ea940db0556dd2dc73e92c20c0d7cbc9e617f251904f20475d4ecc6b6"},
		},
		{Name: "bzip2-libs", Epoch: 0, Version: "1.0.8", Release: "8.el9", Arch: "x86_64", Location: "Packages/bzip2-libs-1.0.8-8.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "fabd6b5c065c2b9d4a8d39a938ae577d801de2ddc73c8cdf6f7803db29c28d0a"}},
		{Name: "ca-certificates", Epoch: 0, Version: "2020.2.50", Release: "94.el9", Arch: "noarch", Location: "Packages/ca-certificates-2020.2.50-94.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09"}},
		{Name: "centos-gpg-keys", Epoch: 0, Version: "9.0", Release: "9.el9", Arch: "noarch", Location: "Packages/centos-gpg-keys-9.0-9.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "2785ab660c124c9bda4ef4057e72d7fc73e8ac254ddd09a5541a6d323740dad7"}},
		{Name: "centos-stream-release", Epoch: 0, Version: "9.0", Release: "9.el9", Arch: "noarch", Location: "Packages/centos-stream-release-9.0-9.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "44246cc9b62ac0fb833ece49cff6ac0a932234fcba26b8c895f42baebf0a19c2"}},
		{Name: "centos-stream-repos", Epoch: 0, Version: "9.0", Release: "9.el9", Arch: "noarch", Location: "Packages/centos-stream-repos-9.0-9.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "90208bb7dd1558a3311a28ea06d75ad7e83be3f223c5fb2eff1b9ac47bb98ebe"}},
		{Name: "coreutils", Epoch: 0, Version: "8.32", Release: "31.el9", Arch: "x86_64", Location: "Packages/coreutils-8.32-31.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "647a3b9a52df25cb2aaf7f3715b219839b4cf71913638c88172d925173280812"}},
		{Name: "coreutils-common", Epoch: 0, Version: "8.32", Release: "31.el9", Arch: "x86_64", Location: "Packages/coreutils-common-8.32-31.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "864b166ac6d55cad5010da369fd7ad4872f81c2c111867dfbf96ccf4c8273c7e"}},
		{Name: "cpio", Epoch: 0, Version: "2.13", Release: "16.el9", Arch: "x86_64", Location: "Packages/cpio-2.13-16.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "216b76d33443b732be42fe1d443e106a17e632ac9ca465928c37a8c0ede596a4"}},
		{Name: "cracklib", Epoch: 0, Version: "2.9.6", Release: "27.el9", Arch: "x86_64", Location: "Packages/cracklib-2.9.6-27.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "be9deb2efd06b4b2c1c130acae94c687161d04830119e65a989d904ba9fd1864"}},
		{Name: "cracklib-dicts", Epoch: 0, Version: "2.9.6", Release: "27.el9", Arch: "x86_64", Location: "Packages/cracklib-dicts-2.9.6-27.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85"}},
		{Name: "crypto-policies", Epoch: 0, Version: "20220203", Release: "1.gitf03e75e.el9", Arch: "noarch", Location: "Packages/crypto-policies-20220203-1.gitf03e75e.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "28d73d3800cb895b265bc0755c41241c593ebd7551a7da7f001f1e254b85c662"}},
		{Name: "cryptsetup-libs", Epoch: 0, Version: "2.4.3", Release: "1.el9", Arch: "x86_64", Location: "Packages/cryptsetup-libs-2.4.3-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "6919d88afdd2cf89982ec8edd4401ff93394a81873f81cf89bb273384f39104f"}},
		{Name: "dbus", Epoch: 1, Version: "1.12.20", Release: "5.el9", Arch: "x86_64", Location: "Packages/dbus-1.12.20-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "bb85bd28cc162e98da53b756b988ffd9350f4dbcc186f4c6962ae047e27f83d3"}},
		{Name: "dbus-broker", Epoch: 0, Version: "28", Release: "5.el9", Arch: "x86_64", Location: "Packages/dbus-broker-28-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "e9efdcdcfe430e474e3a7f09596a0a5a4314692d9ae846bb1ca86ff88ef81038"}},
		{Name: "dbus-common", Epoch: 1, Version: "1.12.20", Release: "5.el9", Arch: "noarch", Location: "Packages/dbus-common-1.12.20-5.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "150048b6fdafd4271bd6badab3f8a2e56b86967266f890770eab7578289cc773"}},
		{Name: "device-mapper", Epoch: 9, Version: "1.02.181", Release: "3.el9", Arch: "x86_64", Location: "Packages/device-mapper-1.02.181-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "5d1cd7733f147020ef3a9e08fa2e9d74a25e0ac89dfbadc69912541286146feb"}},
		{Name: "device-mapper-libs", Epoch: 9, Version: "1.02.181", Release: "3.el9", Arch: "x86_64", Location: "Packages/device-mapper-libs-1.02.181-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "a716ccca85fad2885af4d099f8c213eb4617d637d8ca6cf7d2b483b9de88a5d3"}},
		{Name: "dracut", Epoch: 0, Version: "055", Release: "10.git20210824.el9", Arch: "x86_64", Location: "Packages/dracut-055-10.git20210824.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "54015283e7f85fbee9d8a814c3bd60c7f81a6eb2ff480ca689e4526637a81c83"}},
		{Name: "elfutils-default-yama-scope", Epoch: 0, Version: "0.186", Release: "1.el9", Arch: "noarch", Location: "Packages/elfutils-default-yama-scope-0.186-1.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0d2dcfaa16f83de78a251cf0b9a4c5e4ec7d4deb2e8d1cae7209be7745fabeb5"}},
		{Name: "elfutils-libelf", Epoch: 0, Version: "0.186", Release: "1.el9", Arch: "x86_64", Location: "Packages/elfutils-libelf-0.186-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0e295e6150b6929408ac29792ec5f3ebeb4a20607eb553177f0e4899b3008d63"}},
		{Name: "elfutils-libs", Epoch: 0, Version: "0.186", Release: "1.el9", Arch: "x86_64", Location: "Packages/elfutils-libs-0.186-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "bcc47b8ab496d3d11d772b037e022bc3a4ce3b080b7d1c24fa7f999426a6b8f3"}},
		{Name: "expat", Epoch: 0, Version: "2.2.10", Release: "5.el9", Arch: "x86_64", Location: "Packages/expat-2.2.10-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "f97cd3c1e79b4dfff232ba0208c2e1a7d557608c1c37e8303de4f75387be9bb7"}},
		{Name: "filesystem", Epoch: 0, Version: "3.16", Release: "2.el9", Arch: "x86_64", Location: "Packages/filesystem-3.16-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "b69a472751268a1b9acd566dc7aa486fc1d6c8cb6d23f36d6a6dfead62e71475"}},
		{Name: "findutils", Epoch: 1, Version: "4.8.0", Release: "5.el9", Arch: "x86_64", Location: "Packages/findutils-4.8.0-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "552548e6d6f9623ccd9d31bb185bba3a66730da6e9d02296b417d501356c3848"}},
		{Name: "gdbm-libs", Epoch: 1, Version: "1.19", Release: "4.el9", Arch: "x86_64", Location: "Packages/gdbm-libs-1.19-4.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "8cd5a78cab8783dd241c52c4fcda28fb111c443887dd6d0fe38385e8383c98b3"}},
		{Name: "glibc", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", Location: "Packages/glibc-2.34-21.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "6e40002c40b2e142dac88fba59d9893054b364585b2bc4b63ebf4cb3066616e2"}},
		{Name: "glibc-common", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", Location: "Packages/glibc-common-2.34-21.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "606fda6e7bbe188920afcae1529967fc13c10763ed727d8ac6ce1037a8549228"}},
		{Name: "glibc-gconv-extra", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", Location: "Packages/glibc-gconv-extra-2.34-21.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "cc159162a083a3adf927bcf36fe4c053f3dd3640ff2f7c544018d354e046eccb"}},
		{Name: "glibc-minimal-langpack", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", Location: "Packages/glibc-minimal-langpack-2.34-21.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "bfec403288415b69acb3fd4bd014561d639673c7002c6968e3722e88cb104bdc"}},
		{Name: "gmp", Epoch: 1, Version: "6.2.0", Release: "10.el9", Arch: "x86_64", Location: "Packages/gmp-6.2.0-10.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "1a6ededc80029ef258288ddbf24bcce7c6228647841416950c88e3f14b7258a2"}},
		{Name: "grep", Epoch: 0, Version: "3.6", Release: "5.el9", Arch: "x86_64", Location: "Packages/grep-3.6-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "10a41b66b1fbd6eb055178e22c37199e5b49b4852e77c806f7af7211044a4a55"}},
		{Name: "gzip", Epoch: 0, Version: "1.10", Release: "8.el9", Arch: "x86_64", Location: "Packages/gzip-1.10-8.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "3b5ce98a03a3336a3f32ac7a0867fbc23da702e8618bfd20d49d882d42a460f4"}},
		{Name: "json-c", Epoch: 0, Version: "0.14", Release: "11.el9", Arch: "x86_64", Location: "Packages/json-c-0.14-11.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8"}},
		{Name: "kbd", Epoch: 0, Version: "2.4.0", Release: "8.el9", Arch: "x86_64", Location: "Packages/kbd-2.4.0-8.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "9c7395caebf76e15f496d9dc7690d772cb34f29d3f6626086b578565e412df51"}},
		{Name: "kbd-misc", Epoch: 0, Version: "2.4.0", Release: "8.el9", Arch: "noarch", Location: "Packages/kbd-misc-2.4.0-8.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "2dda3fe56c9a5bce5880dca58d905682c5e9f94ee023e43a3e311d2d411e1849"}},
		{Name: "kernel", Epoch: 0, Version: "5.14.0", Release: "55.el9", Arch: "x86_64", Location: "Packages/kernel-5.14.0-55.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "be5dba9121cda9eac9cc8f20b24f7e0d198364b370546c3665e4e6ce70a335b4"}},
		{Name: "kernel-core", Epoch: 0, Version: "5.14.0", Release: "55.el9", Arch: "x86_64", Location: "Packages/kernel-core-5.14.0-55.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0afe6e35348485ae2696e6170dcf34370f33fcf42a357fc815e332d939dd1025"}},
		{Name: "kernel-modules", Epoch: 0, Version: "5.14.0", Release: "55.el9", Arch: "x86_64", Location: "Packages/kernel-modules-5.14.0-55.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0914a0cbe0304289e224789f8e50e3e48a2525eba742ad764a1901e8c1351fb5"}},
		{Name: "kmod", Epoch: 0, Version: "28", Release: "7.el9", Arch: "x86_64", Location: "Packages/kmod-28-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "3d4bc7935959a109a10020d0d19a5e059719ae4c99c5f32d3020ff6da47d53ea"}},
		{Name: "kmod-libs", Epoch: 0, Version: "28", Release: "7.el9", Arch: "x86_64", Location: "Packages/kmod-libs-28-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0727ff3131223446158aaec88cbf8f894a9e3592e73f231a1802629518eeb64b"}},
		{Name: "kpartx", Epoch: 0, Version: "0.8.7", Release: "4.el9", Arch: "x86_64", Location: "Packages/kpartx-0.8.7-4.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "8f05761c418a55f811404dc1515b131bafe9b1e3fe56274be6d880c8822984b5"}},
		{Name: "libacl", Epoch: 0, Version: "2.3.1", Release: "3.el9", Arch: "x86_64", Location: "Packages/libacl-2.3.1-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "fd829e9a03f6d321313002d6fcb37ee0434f548aa75fcd3ecdbdd891115de6a7"}},
		{Name: "libattr", Epoch: 0, Version: "2.5.1", Release: "3.el9", Arch: "x86_64", Location: "Packages/libattr-2.5.1-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "d4db095a015e84065f27a642ee7829cd1690041ba8c51501f908cc34760c9409"}},
		{Name: "libblkid", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/libblkid-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "f5cf36e8081c2d72e9dd64dd1614155857dd6e71ebb2237e5b0e11ace5481bac"}},
		{Name: "libcap", Epoch: 0, Version: "2.48", Release: "8.el9", Arch: "x86_64", Location: "Packages/libcap-2.48-8.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "c41f91075ee8ca480c2631a485bcc74876b9317b4dc9bd66566da32313621bd7"}},
		{Name: "libcap-ng", Epoch: 0, Version: "0.8.2", Release: "6.el9", Arch: "x86_64", Location: "Packages/libcap-ng-0.8.2-6.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0ee8b2d02fd362223fcf36c11297e1f9ae939f76cef09c0bce9cad5f53287122"}},
		{Name: "libdb", Epoch: 0, Version: "5.3.28", Release: "53.el9", Arch: "x86_64", Location: "Packages/libdb-5.3.28-53.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "3a44d15d695944bde4e7290800b815f98bfd9cd6f6f868cec3e8991606f556d5"}},
		{Name: "libeconf", Epoch: 0, Version: "0.4.1", Release: "2.el9", Arch: "x86_64", Location: "Packages/libeconf-0.4.1-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "1d6fe169e74daff38ad5b0d6424c4d1b14545d5974c39e4421d20838a68f5892"}},
		{Name: "libevent", Epoch: 0, Version: "2.1.12", Release: "6.el9", Arch: "x86_64", Location: "Packages/libevent-2.1.12-6.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "82179f6f214ddf523e143c16c3474ccf8832551c6305faf89edfbd83b3424d48"}},
		{Name: "libfdisk", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/libfdisk-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "a41bad6e261c719224abfd6745ccb1d2a0cac9d024ca9656904001a38d7cd8c7"}},
		{Name: "libffi", Epoch: 0, Version: "3.4.2", Release: "7.el9", Arch: "x86_64", Location: "Packages/libffi-3.4.2-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "f0ac4b6454d4018833dd10e3f437d8271c7c6a628d99b37e75b83af890b86bc4"}},
		{Name: "libgcc", Epoch: 0, Version: "11.2.1", Release: "9.1.el9", Arch: "x86_64", Location: "Packages/libgcc-11.2.1-9.1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "6fc0ea086ecf7ae65bdfc2e9ba6503ee9d9bf717f3c0a55c4bc9c99e12608edf"}},
		{Name: "libgcrypt", Epoch: 0, Version: "1.10.0", Release: "1.el9", Arch: "x86_64", Location: "Packages/libgcrypt-1.10.0-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "059533802d440244c1fb6f777e20ed445220cdc85300e164d8ffb0ecfdfb42f4"}},
		{Name: "libgpg-error", Epoch: 0, Version: "1.42", Release: "5.el9", Arch: "x86_64", Location: "Packages/libgpg-error-1.42-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe"}},
		{Name: "libkcapi", Epoch: 0, Version: "1.3.1", Release: "3.el9", Arch: "x86_64", Location: "Packages/libkcapi-1.3.1-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "9b4733e8a790b51d845cedfa67e6321fd5a2923dd0fb7ce1f5e630aa382ba3c1"}},
		{Name: "libkcapi-hmaccalc", Epoch: 0, Version: "1.3.1", Release: "3.el9", Arch: "x86_64", Location: "Packages/libkcapi-hmaccalc-1.3.1-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "1b39f1faa4a8813cbfa2650e82f6ea06a4248e0c493ce7e4829c7d892e7757ed"}},
		{Name: "libmount", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/libmount-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "26191af0cc7acf9bb335ebd8b4ed357582165ee3be78fce9f4395f84ad2805ce"}},
		{Name: "libpwquality", Epoch: 0, Version: "1.4.4", Release: "8.el9", Arch: "x86_64", Location: "Packages/libpwquality-1.4.4-8.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "93f00e5efac1e3f1ecbc0d6a4c068772cb12912cd20c9ea58716d6c0cd004886"}},
		{Name: "libseccomp", Epoch: 0, Version: "2.5.2", Release: "2.el9", Arch: "x86_64", Location: "Packages/libseccomp-2.5.2-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "d5c1c4473ebf5fd9c605eb866118d7428cdec9b188db18e45545801cc2a689c3"}},
		{Name: "libselinux", Epoch: 0, Version: "3.3", Release: "2.el9", Arch: "x86_64", Location: "Packages/libselinux-3.3-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "8e589b8408b04cbc19564620b229b6768edbaeb9090885d2273d84b8fc2f172b"}},
		{Name: "libsemanage", Epoch: 0, Version: "3.3", Release: "1.el9", Arch: "x86_64", Location: "Packages/libsemanage-3.3-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "7e62a0ed0a508486b565e48794a146022f344aeb6801834ad7c5afe6a97ef065"}},
		{Name: "libsepol", Epoch: 0, Version: "3.3", Release: "2.el9", Arch: "x86_64", Location: "Packages/libsepol-3.3-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "fc508147fe876706b61941a6ce554d7f7786f1ec3d097c4411fd6c7511acd289"}},
		{Name: "libsigsegv", Epoch: 0, Version: "2.13", Release: "4.el9", Arch: "x86_64", Location: "Packages/libsigsegv-2.13-4.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a"}},
		{Name: "libsmartcols", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/libsmartcols-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "c62433784604a2e6571e0fcbdd4a2d60f059c5c15624207998c5f03b18d9d382"}},
		{Name: "libtasn1", Epoch: 0, Version: "4.16.0", Release: "7.el9", Arch: "x86_64", Location: "Packages/libtasn1-4.16.0-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "656031558c53da4a5b3ccfd883bd6d55996037891323152b1f07e8d1d5377406"}},
		{Name: "libutempter", Epoch: 0, Version: "1.2.1", Release: "6.el9", Arch: "x86_64", Location: "Packages/libutempter-1.2.1-6.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "fab361a9cba04490fd8b5664049983d1e57ebf7c1080804726ba600708524125"}},
		{Name: "libuuid", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/libuuid-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "ffd8317ccc6f80524b7bf15a8157d82f36a2b9c7478bb04eb4a34c18d019e6fa"}},
		{Name: "libxcrypt", Epoch: 0, Version: "4.4.18", Release: "3.el9", Arch: "x86_64", Location: "Packages/libxcrypt-4.4.18-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "97e88678b420f619a44608fff30062086aa1dd6931ecbd54f21bba005ff1de1a"}},
		{Name: "libzstd", Epoch: 0, Version: "1.5.0", Release: "2.el9", Arch: "x86_64", Location: "Packages/libzstd-1.5.0-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "8282f33f06743ab88e36fea978559ac617c44cda14eb65495cad37505fdace41"}},
		{Name: "linux-firmware", Epoch: 0, Version: "20211216", Release: "124.el9", Arch: "noarch", Location: "Packages/linux-firmware-20211216-124.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0524c9cd96db4d57a5c190165ee2f8ade91854e21c19c61c6bd3504030ca24fa"}},
		{Name: "linux-firmware-whence", Epoch: 0, Version: "20211216", Release: "124.el9", Arch: "noarch", Location: "Packages/linux-firmware-whence-20211216-124.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "7c58504c14979118ea36352982aaa5814ba0f448e17c1baddb811b9511315a58"}},
		{Name: "lz4-libs", Epoch: 0, Version: "1.9.3", Release: "5.el9", Arch: "x86_64", Location: "Packages/lz4-libs-1.9.3-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "cba6a63054d070956a182e33269ee245bcfbe87e3e605c27816519db762a66ad"}},
		{Name: "ncurses-base", Epoch: 0, Version: "6.2", Release: "8.20210508.el9", Arch: "noarch", Location: "Packages/ncurses-base-6.2-8.20210508.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8"}},
		{Name: "ncurses-libs", Epoch: 0, Version: "6.2", Release: "8.20210508.el9", Arch: "x86_64", Location: "Packages/ncurses-libs-6.2-8.20210508.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "328f4d50e66b00f24344ebe239817204fda8e68b1d988c6943abb3c36231beaa"}},
		{Name: "openssl", Epoch: 1, Version: "3.0.1", Release: "5.el9", Arch: "x86_64", Location: "Packages/openssl-3.0.1-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "aa9ee73fe806ddeafab4a5b0e370256e6c61f67f67114101d0735aed3c9a5eda"}},
		{Name: "openssl-libs", Epoch: 1, Version: "3.0.1", Release: "5.el9", Arch: "x86_64", Location: "Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666"}},
		{Name: "openssl-pkcs11", Epoch: 0, Version: "0.4.11", Release: "7.el9", Arch: "x86_64", Location: "Packages/openssl-pkcs11-0.4.11-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "4be41142a5fb2b4cd6d812e126838cffa57b7c84e5a79d65f66bb9cf1d2830a3"}},
		{Name: "p11-kit", Epoch: 0, Version: "0.24.1", Release: "2.el9", Arch: "x86_64", Location: "Packages/p11-kit-0.24.1-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6"}},
		{Name: "p11-kit-trust", Epoch: 0, Version: "0.24.1", Release: "2.el9", Arch: "x86_64", Location: "Packages/p11-kit-trust-0.24.1-2.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1"}},
		{Name: "pam", Epoch: 0, Version: "1.5.1", Release: "9.el9", Arch: "x86_64", Location: "Packages/pam-1.5.1-9.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "e64caedce811645ecdd78e7b4ae83c189aa884ff1ba6445374f39186c588c52c"}},
		{Name: "pcre", Epoch: 0, Version: "8.44", Release: "3.el9.3", Arch: "x86_64", Location: "Packages/pcre-8.44-3.el9.3.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "4a3cb61eb08c4f24e44756b6cb329812fe48d5c65c1fba546fadfa975045a8c5"}},
		{Name: "pcre2", Epoch: 0, Version: "10.37", Release: "3.el9.1", Arch: "x86_64", Location: "Packages/pcre2-10.37-3.el9.1.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "441e71f24e95b7c319f02264db53f88aa49778b2214f7dd5c75f1a3838e72dea"}},
		{Name: "pcre2-syntax", Epoch: 0, Version: "10.37", Release: "3.el9.1", Arch: "noarch", Location: "Packages/pcre2-syntax-10.37-3.el9.1.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e"}},
		{Name: "procps-ng", Epoch: 0, Version: "3.3.17", Release: "4.el9", Arch: "x86_64", Location: "Packages/procps-ng-3.3.17-4.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "3a7cc3f6d6dfdaeb9e7bfdb06d968c3ae78246ff4f793c2d2e2bd71156961d69"}},
		{Name: "readline", Epoch: 0, Version: "8.1", Release: "4.el9", Arch: "x86_64", Location: "Packages/readline-8.1-4.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "49945472925286ad89b0575657b43f9224777e36b442f0c88df67f0b61e26aee"}},
		{Name: "sed", Epoch: 0, Version: "4.8", Release: "9.el9", Arch: "x86_64", Location: "Packages/sed-4.8-9.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f"}},
		{Name: "setup", Epoch: 0, Version: "2.13.7", Release: "6.el9", Arch: "noarch", Location: "Packages/setup-2.13.7-6.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "c0202712e8ec928cf61f3d777f23859ba6de2e85786e928ee5472fdde570aeee"}},
		{Name: "shadow-utils", Epoch: 2, Version: "4.9", Release: "3.el9", Arch: "x86_64", Location: "Packages/shadow-utils-4.9-3.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "46fca2ed21478e5143434da4fbd47ca4599a885fab9f8636f9c7ba54942dd27e"}},
		{Name: "systemd", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", Location: "Packages/systemd-249-9.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "eb57c1242f8a7d68e6c258f40b048d8b7bd0749254ac97b7f399b3bc8011a81b"}},
		{Name: "systemd-libs", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", Location: "Packages/systemd-libs-249-9.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "708fbc3c7fd77a21e0b391e2a80d5c344962de9865e79514b2c89210ef06ba39"}},
		{Name: "systemd-pam", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", Location: "Packages/systemd-pam-249-9.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "eb7af981fb95425c68ccb0e4b95688672afd3032b57002e65fda8f734a089556"}},
		{Name: "systemd-rpm-macros", Epoch: 0, Version: "249", Release: "9.el9", Arch: "noarch", Location: "Packages/systemd-rpm-macros-249-9.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "3552f7cc9077d5831f859f6cf721d419eccc83cb381d14a7a1483512272bd586"}},
		{Name: "systemd-udev", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", Location: "Packages/systemd-udev-249-9.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "d9c47e7088b8d279b8fd51e2274df957c3b68a265b42123ef9bbeb339d5ce3ba"}},
		{Name: "tmux", Epoch: 0, Version: "3.2a", Release: "4.el9", Arch: "x86_64", Location: "Packages/tmux-3.2a-4.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "68074b673bfac39af1fbfc85d43bc1c111456db01a563bda6400ad485de5eb70"}},
		{Name: "tzdata", Epoch: 0, Version: "2021e", Release: "1.el9", Arch: "noarch", Location: "Packages/tzdata-2021e-1.el9.noarch.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "42d89577a0f887c4baa162250862dea2c1830b1ced56c45ced9645ad8e2a3671"}},
		{Name: "util-linux", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/util-linux-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "4ca41a925461daa936db284a59bf325ea061cdb39d8738e288cc19afe30a8ae8"}},
		{Name: "util-linux-core", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", Location: "Packages/util-linux-core-2.37.2-1.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "0313682867c1d07785a6d02ff87e1899f484bd1ce6348fa5c673eca78c0da2bd"}},
		{Name: "vim-minimal", Epoch: 2, Version: "8.2.2637", Release: "11.el9", Arch: "x86_64", Location: "Packages/vim-minimal-8.2.2637-11.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "ab6e48c8a118bed88dc734aaf21e743b57e94d448f9e38745c3b777af96809c7"}},
		{Name: "xz", Epoch: 0, Version: "5.2.5", Release: "7.el9", Arch: "x86_64", Location: "Packages/xz-5.2.5-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "b1c2d99961e50bb46400caa528aab9c7b361f5754427fd05ae22a7b551bf2ce5"}},
		{Name: "xz-libs", Epoch: 0, Version: "5.2.5", Release: "7.el9", Arch: "x86_64", Location: "Packages/xz-libs-5.2.5-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "770819da28cce56e2e2b141b0eee1694d7f3dcf78a5700e1469436461399f001"}},
		{Name: "zlib", Epoch: 0, Version: "1.2.11", Release: "31.el9", Arch: "x86_64", Location: "Packages/zlib-1.2.11-31.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "1c59b113fda8863e9066cc5d01c6d00bd9c50c4650e1c5b932082c8886e185d1"}},
		{Name: "zsh", Epoch: 0, Version: "5.8", Release: "7.el9", Arch: "x86_64", Location: "Packages/zsh-5.8-7.el9.x86_64.rpm", Checksum: rpmmd.Checksum{Type: "sha256", Value: "133da157fbd2b43e4a41af3ba7bb5267cf9ebed0aaf8124a76e5eca948c37572"}},
	}

	for idx := range expectedTemplate {
		for _, baseurl := range repo.BaseURLs {
			expectedTemplate[idx].RemoteLocations = append(expectedTemplate[idx].RemoteLocations, fmt.Sprintf("%s/%s", baseurl, expectedTemplate[idx].Location))
		}
		if repo.CheckGPG != nil {
			expectedTemplate[idx].CheckGPG = *repo.CheckGPG
		}
		if repo.IgnoreSSL != nil {
			expectedTemplate[idx].IgnoreSSL = *repo.IgnoreSSL
		}
		expectedTemplate[idx].RepoID = repo.Id
		expectedTemplate[idx].Repo = &repo
	}
	return expectedTemplate
}

func TestErrorRepoInfo(t *testing.T) {
	requireDNF(t)

	type testCase struct {
		repo   rpmmd.RepoConfig
		expMsg string
	}

	testCases := []testCase{
		{
			repo: rpmmd.RepoConfig{
				Name:     "",
				BaseURLs: []string{"https://0.0.0.0/baseos/repo"},
				Metalink: "https://0.0.0.0/baseos/metalink",
			},
			expMsg: "https://0.0.0.0/baseos/repo",
		},
		{
			repo: rpmmd.RepoConfig{
				Name:     "baseos",
				BaseURLs: []string{"https://0.0.0.0/baseos/repo"},
				Metalink: "https://0.0.0.0/baseos/metalink",
			},
			expMsg: "https://0.0.0.0/baseos/repo",
		},
		{
			repo: rpmmd.RepoConfig{
				Name:     "fedora",
				Metalink: "https://0.0.0.0/f35/metalink",
			},
			expMsg: "https://0.0.0.0/f35/metalink",
		},
		{
			repo: rpmmd.RepoConfig{
				Name:       "",
				MirrorList: "https://0.0.0.0/baseos/mirrors",
			},
			expMsg: "https://0.0.0.0/baseos/mirrors",
		},
	}

	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			assert := assert.New(t)
			solver := NewSolver("platform:f38", "38", "x86_64", "fedora-38", "/tmp/cache")
			for idx, tc := range testCases {
				t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
					_, err := solver.Depsolve([]rpmmd.PackageSet{
						{
							Include:      []string{"osbuild"},
							Exclude:      nil,
							Repositories: []rpmmd.RepoConfig{tc.repo},
						},
					}, sbom.StandardTypeNone)
					assert.Error(err)
					assert.Contains(err.Error(), tc.expMsg)
				})
			}
		})
	}
}

func TestHashRequest(t *testing.T) {
	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			solver := NewSolver("platform:f38", "38", "x86_64", "fedora-38", "/tmp/cache")
			repos := []rpmmd.RepoConfig{
				{
					Name:      "A test repository",
					BaseURLs:  []string{"https://arepourl/"},
					IgnoreSSL: common.ToPtr(false),
				},
			}

			req, err := activeHandler.makeDumpRequest(solver.solverCfg(), repos)
			assert.Nil(t, err)
			reqData, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshalling dump request failed: %v", err)
			}
			reqHash := hashRequest(reqData)
			assert.Equal(t, 64, len(reqHash))

			req2, err := activeHandler.makeSearchRequest(solver.solverCfg(), repos, []string{"package0*"})
			assert.Nil(t, err)
			reqData2, err := json.Marshal(req2)
			if err != nil {
				t.Fatalf("marshalling search request failed: %v", err)
			}
			reqHash2 := hashRequest(reqData2)
			assert.Equal(t, 64, len(reqHash2))
			assert.NotEqual(t, reqHash, reqHash2)
		})
	}
}

func TestSolverRunErrorEmptyOutput(t *testing.T) {
	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			fakeSolverPath := filepath.Join(t.TempDir(), "osbuild-depsolve-dnf")
			fakeSolver := `#!/bin/sh -e
cat - > "$0".stdin
exit 1
`
			err := os.WriteFile(fakeSolverPath, []byte(fakeSolver), 0o755)
			assert.NoError(t, err)

			solver := NewSolver("platform:f38", "38", "x86_64", "fedora-38", t.TempDir())
			solver.depsolveDNFCmd = []string{fakeSolverPath}
			res, err := solver.Depsolve(nil, sbom.StandardTypeNone)

			assert.EqualError(t, err, `DNF error occurred: InternalError: osbuild-depsolve-dnf output was empty: depsolve failed with exit code 1`)
			assert.Nil(t, res)
		})
	}
}

func TestSolverRunWithSolverNoError(t *testing.T) {
	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			tmpdir := t.TempDir()
			fakeSolver := `#!/bin/sh -e
cat - > "$0".stdin
echo '{"solver": "zypper"}'
>&2 echo "output-on-stderr"
`
			fakeSolverPath := filepath.Join(tmpdir, "fake-solver")
			err := os.WriteFile(fakeSolverPath, []byte(fakeSolver), 0755) //nolint:gosec
			assert.NoError(t, err)

			var capturedStderr bytes.Buffer
			solver := NewSolver("platform:f38", "38", "x86_64", "fedora-38", "/tmp/cache")
			solver.Stderr = &capturedStderr
			solver.depsolveDNFCmd = []string{fakeSolverPath}
			res, err := solver.Depsolve(nil, sbom.StandardTypeNone)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, "output-on-stderr\n", capturedStderr.String())

			// prerequisite check, i.e. ensure our fake was called in the right way
			stdin, err := os.ReadFile(fakeSolverPath + ".stdin")
			assert.NoError(t, err)
			assert.Contains(t, string(stdin), `"command":"depsolve"`)

			// adding the "solver" did not cause any issues
			assert.NoError(t, err)
			assert.Equal(t, 0, len(res.Repos))
		})
	}
}

func TestDepsolverSubscriptionsError(t *testing.T) {
	if _, err := os.Stat("/etc/yum.repos.d/redhat.repo"); err == nil {
		t.Skip("Test must run on unsubscribed system")
	}

	s := rpmrepo.NewTestServer()
	defer s.Close()

	for _, h := range getTestHandlers() {
		t.Run(h.name, func(t *testing.T) {
			restore := mockActiveHandler(h.handler)
			defer restore()

			tmpdir := t.TempDir()
			solver := NewSolver("platform:el9", "9", "x86_64", "rhel9.0", tmpdir)

			rootDir := t.TempDir()
			reposDir := filepath.Join(rootDir, "etc", "yum.repos.d")
			require.NoError(t, os.MkdirAll(reposDir, 0777))

			s.WriteConfig(filepath.Join(reposDir, "test.repo"))
			s.RepoConfig.RHSM = true

			pkgsets := []rpmmd.PackageSet{
				{
					Include:      []string{"kernel"},
					Repositories: []rpmmd.RepoConfig{s.RepoConfig},
				},
			}
			solver.SetRootDir(rootDir)
			_, err := solver.Depsolve(pkgsets, 0)
			assert.EqualError(t, err, "This system does not have any valid subscriptions. Subscribe it before specifying rhsm: true in sources (error details: no matching key and certificate pair)")
		})
	}
}
