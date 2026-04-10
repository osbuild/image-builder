package osbuild

import (
	"fmt"
	"testing"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPMStageOptionsClone(t *testing.T) {
	tests := []struct {
		name string
		opts *RPMStageOptions
	}{
		{
			name: "nil",
			opts: nil,
		},
		{
			name: "empty",
			opts: &RPMStageOptions{},
		},
		{
			name: "all-fields",
			opts: &RPMStageOptions{
				DBPath:           "/var/lib/rpm",
				GPGKeys:          []string{"key1", "key2"},
				GPGKeysFromTree:  []string{"/etc/pki/rpm-gpg/RPM-GPG-KEY-fedora"},
				DisableDracut:    true,
				Exclude:          &Exclude{Docs: true},
				OSTreeBooted:     common.ToPtr(true),
				KernelInstallEnv: &KernelInstallEnv{BootRoot: "/boot"},
				InstallLangs:     []string{"en_US", "de_DE"},
			},
		},
		{
			name: "only-slices",
			opts: &RPMStageOptions{
				GPGKeys:         []string{"single-key"},
				GPGKeysFromTree: []string{"/path/to/key"},
				InstallLangs:    []string{"en_US"},
			},
		},
		{
			name: "only-pointers",
			opts: &RPMStageOptions{
				Exclude:          &Exclude{Docs: false},
				OSTreeBooted:     common.ToPtr(false),
				KernelInstallEnv: &KernelInstallEnv{BootRoot: ""},
			},
		},
		{
			name: "empty-slices",
			opts: &RPMStageOptions{
				GPGKeys:         []string{},
				GPGKeysFromTree: []string{},
				InstallLangs:    []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := tt.opts.Clone()
			assert.Equal(t, tt.opts, clone)

			if tt.opts == nil {
				assert.Nil(t, clone)
				return
			}

			assert.NotSame(t, tt.opts, clone)

			// Verify deep copy of slices (modifying clone shouldn't affect original)
			if len(clone.GPGKeys) > 0 {
				clone.GPGKeys[0] = "modified"
				assert.NotEqual(t, tt.opts.GPGKeys[0], clone.GPGKeys[0])
			}
			if len(clone.GPGKeysFromTree) > 0 {
				clone.GPGKeysFromTree[0] = "modified"
				assert.NotEqual(t, tt.opts.GPGKeysFromTree[0], clone.GPGKeysFromTree[0])
			}
			if len(clone.InstallLangs) > 0 {
				clone.InstallLangs[0] = "modified"
				assert.NotEqual(t, tt.opts.InstallLangs[0], clone.InstallLangs[0])
			}

			// Verify deep copy of pointer fields
			if clone.Exclude != nil {
				assert.NotSame(t, tt.opts.Exclude, clone.Exclude)
				clone.Exclude.Docs = !clone.Exclude.Docs
				assert.NotEqual(t, tt.opts.Exclude.Docs, clone.Exclude.Docs)
			}
			if clone.OSTreeBooted != nil {
				assert.NotSame(t, tt.opts.OSTreeBooted, clone.OSTreeBooted)
			}
			if clone.KernelInstallEnv != nil {
				assert.NotSame(t, tt.opts.KernelInstallEnv, clone.KernelInstallEnv)
				clone.KernelInstallEnv.BootRoot = "modified"
				assert.NotEqual(t, tt.opts.KernelInstallEnv.BootRoot, clone.KernelInstallEnv.BootRoot)
			}
		})
	}
}

func TestNewRPMStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rpm",
		Options: &RPMStageOptions{},
		Inputs:  &RPMStageInputs{},
	}
	actualStage := NewRPMStage(&RPMStageOptions{}, &RPMStageInputs{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewRpmStageSourceFilesInputs(t *testing.T) {

	assert := assert.New(t)
	require := require.New(t)

	pkgs := rpmmd.PackageList{
		{
			Name:            "openssl-libs",
			Epoch:           1,
			Version:         "3.0.1",
			Release:         "5.el9",
			Arch:            "x86_64",
			RemoteLocations: []string{"https://example.com/repo/Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm"},
			Checksum:        rpmmd.Checksum{Type: "sha256", Value: "fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666"},
			Secrets:         "",
			CheckGPG:        false,
			IgnoreSSL:       true,
		},
		{
			Name:            "openssl-pkcs11",
			Epoch:           0,
			Version:         "0.4.11",
			Release:         "7.el9",
			Arch:            "x86_64",
			RemoteLocations: []string{"https://example.com/repo/Packages/openssl-pkcs11-0.4.11-7.el9.x86_64.rpm"},
			Checksum:        rpmmd.Checksum{Type: "sha256", Value: "4be41142a5fb2b4cd6d812e126838cffa57b7c84e5a79d65f66bb9cf1d2830a3"},
			Secrets:         "",
			CheckGPG:        false,
			IgnoreSSL:       true,
		},
		{
			Name:            "p11-kit",
			Epoch:           0,
			Version:         "0.24.1",
			Release:         "2.el9",
			Arch:            "x86_64",
			RemoteLocations: []string{"https://example.com/repo/Packages/p11-kit-0.24.1-2.el9.x86_64.rpm"},
			Checksum:        rpmmd.Checksum{Type: "sha256", Value: "da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6"},
			Secrets:         "",
			CheckGPG:        false,
			IgnoreSSL:       true,
		},
		{
			Name:            "package-with-sha1-checksum",
			Epoch:           1,
			Version:         "3.4.2.",
			Release:         "10.el9",
			Arch:            "x86_64",
			RemoteLocations: []string{"https://example.com/repo/Packages/package-with-sha1-checksum-4.3.2-10.el9.x86_64.rpm"},
			Checksum:        rpmmd.Checksum{Type: "sha1", Value: "6e01b8076a2ab729d564048bf2e3a97c7ac83c13"},
			Secrets:         "",
			CheckGPG:        true,
			IgnoreSSL:       true,
		},
		{
			Name:            "package-with-md5-checksum",
			Epoch:           1,
			Version:         "3.4.2.",
			Release:         "5.el9",
			Arch:            "x86_64",
			RemoteLocations: []string{"https://example.com/repo/Packages/package-with-md5-checksum-4.3.2-5.el9.x86_64.rpm"},
			Checksum:        rpmmd.Checksum{Type: "md5", Value: "8133f479f38118c5f9facfe2a2d9a071"},
			Secrets:         "",
			CheckGPG:        true,
			IgnoreSSL:       true,
		},
	}
	inputs := NewRpmStageSourceFilesInputs(pkgs)

	refsArrayPtr, convOk := inputs.Packages.References.(*FilesInputSourceArrayRef)
	require.True(convOk)
	require.NotNil(refsArrayPtr)

	refsArray := *refsArrayPtr

	for idx := range refsArray {
		refItem := refsArray[idx]
		pkg := pkgs[idx]
		assert.Equal(pkg.Checksum.String(), refItem.ID)

		if pkg.CheckGPG {
			// GPG check enabled: metadata expected
			require.NotNil(refItem.Options)
			require.NotNil(refItem.Options.Metadata)

			md, convOk := refItem.Options.Metadata.(*RPMStageReferenceMetadata)
			require.True(convOk)
			require.NotNil(md)
			assert.Equal(md.CheckGPG, pkg.CheckGPG)
		}
	}
}

func TestGPGKeysForPackages(t *testing.T) {
	// Define key values as variables for reuse
	key1 := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nkey1\n-----END PGP PUBLIC KEY BLOCK-----"
	key2 := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nkey2\n-----END PGP PUBLIC KEY BLOCK-----"
	keyA := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nkeyA\n-----END PGP PUBLIC KEY BLOCK-----"
	keyB := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nkeyB\n-----END PGP PUBLIC KEY BLOCK-----"

	repoWithKey1 := &rpmmd.RepoConfig{GPGKeys: []string{key1}}
	repoWithKey2 := &rpmmd.RepoConfig{GPGKeys: []string{key2}}
	repoWithMultipleKeys := &rpmmd.RepoConfig{GPGKeys: []string{keyA, keyB}}
	repoNoKeys := &rpmmd.RepoConfig{GPGKeys: nil}

	tests := map[string]struct {
		pkgs        rpmmd.PackageList
		expected    []string
		expectError string
	}{
		"empty-package-list": {
			pkgs:     rpmmd.PackageList{},
			expected: nil,
		},
		"repo-without-gpg-keys-checkgpg-false": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoNoKeys, CheckGPG: false},
			},
			expected: nil,
		},
		"single-package-with-keys": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoWithKey1},
			},
			expected: nil,
		},
		"multiple-packages-same-repo-deduplicated": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoWithKey1, CheckGPG: true},
				{Name: "pkg2", Repo: repoWithKey1, CheckGPG: true},
				{Name: "pkg3", Repo: repoWithKey1, CheckGPG: true},
			},
			expected: []string{key1},
		},
		"multiple-packages-different-repos": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoWithKey1, CheckGPG: true},
				{Name: "pkg3", Repo: repoNoKeys, CheckGPG: false},
				{Name: "pkg2", Repo: repoWithKey2, CheckGPG: true},
			},
			expected: []string{key1, key2},
		},
		"repo-with-multiple-keys": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoWithMultipleKeys, CheckGPG: true},
			},
			expected: []string{keyA, keyB},
		},
		// Error cases
		"error-checkgpg-true-no-keys": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoNoKeys, CheckGPG: true},
			},
			expectError: fmt.Sprintf(
				"package \"pkg1\" requires GPG check but repo %q has no GPG keys configured", repoNoKeys.Id),
		},
		"error-nil-repo-among-valid": {
			pkgs: rpmmd.PackageList{
				{Name: "pkg1", Repo: repoWithKey1},
				{Name: "pkg2", Repo: nil},
			},
			expectError: "package \"pkg2\" has nil Repo pointer. This is a bug in depsolving.",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := GPGKeysForPackages(tc.pkgs)

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenRPMStagesFromTransactions(t *testing.T) {
	// Shared test data
	key1 := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nkey1\n-----END PGP PUBLIC KEY BLOCK-----"
	key2 := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nkey2\n-----END PGP PUBLIC KEY BLOCK-----"

	repoWithKey1 := &rpmmd.RepoConfig{GPGKeys: []string{key1}}
	repoWithKey2 := &rpmmd.RepoConfig{GPGKeys: []string{key2}}
	repoNoKeys := &rpmmd.RepoConfig{}

	tests := map[string]struct {
		transactions depsolvednf.TransactionList
		baseOpts     *RPMStageOptions
		expectError  string
		expectStages int
		validate     func(t *testing.T, stages []*Stage)
	}{
		"empty-transactions": {
			transactions: depsolvednf.TransactionList{},
			baseOpts:     &RPMStageOptions{},
			expectStages: 0,
		},
		"nil-base-opts": {
			transactions: depsolvednf.TransactionList{
				{{
					Name:     "pkg-a",
					Repo:     repoWithKey1,
					Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"},
					CheckGPG: true,
				}},
			},
			baseOpts:     nil,
			expectStages: 1,
			validate: func(t *testing.T, stages []*Stage) {
				opts := stages[0].Options.(*RPMStageOptions)
				assert.Equal(t, []string{key1}, opts.GPGKeys)
			},
		},
		"single-transaction-with-gpg-keys": {
			transactions: depsolvednf.TransactionList{
				{
					{
						Name:     "pkg-a",
						Repo:     repoWithKey1,
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"},
						CheckGPG: true,
					},
					{
						Name:     "pkg-b",
						Repo:     repoWithKey1,
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbb"},
						CheckGPG: true,
					},
				},
			},
			baseOpts:     &RPMStageOptions{},
			expectStages: 1,
			validate: func(t *testing.T, stages []*Stage) {
				opts := stages[0].Options.(*RPMStageOptions)
				assert.Equal(t, []string{key1}, opts.GPGKeys)
			},
		},
		"multiple-transactions-different-repos": {
			transactions: depsolvednf.TransactionList{
				{
					{
						Name:     "pkg-a",
						Repo:     repoWithKey1,
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"},
						CheckGPG: true,
					},
				},
				{
					{
						Name:     "pkg-b",
						Repo:     repoWithKey2,
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbb"},
						CheckGPG: true,
					},
					{
						Name:     "pkg-c",
						Repo:     repoWithKey1,
						Checksum: rpmmd.Checksum{Type: "sha256", Value: "ccc"},
						CheckGPG: true,
					},
				},
			},
			baseOpts:     &RPMStageOptions{},
			expectStages: 2,
			validate: func(t *testing.T, stages []*Stage) {
				opts0 := stages[0].Options.(*RPMStageOptions)
				opts1 := stages[1].Options.(*RPMStageOptions)
				assert.Equal(t, []string{key1}, opts0.GPGKeys)
				assert.Equal(t, []string{key1, key2}, opts1.GPGKeys)
			},
		},
		"empty-transaction-skipped": {
			transactions: depsolvednf.TransactionList{
				{{Name: "pkg-a", Repo: repoNoKeys, Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"}}},
				{}, // empty
				{{Name: "pkg-b", Repo: repoNoKeys, Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbb"}}},
			},
			baseOpts:     &RPMStageOptions{},
			expectStages: 2,
			validate: func(t *testing.T, stages []*Stage) {
				for idx, stage := range stages {
					opts := stage.Options.(*RPMStageOptions)
					assert.Empty(t, opts.GPGKeys, "stage %d", idx)
				}
			},
		},
		"base-opts-copied-to-all-stages": {
			transactions: depsolvednf.TransactionList{
				{{Name: "pkg-a", Repo: repoNoKeys, Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"}}},
				{{Name: "pkg-b", Repo: repoNoKeys, Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbb"}}},
			},
			baseOpts: &RPMStageOptions{
				DBPath:           "/custom/db",
				DisableDracut:    true,
				OSTreeBooted:     common.ToPtr(true),
				Exclude:          &Exclude{Docs: true},
				KernelInstallEnv: &KernelInstallEnv{BootRoot: "/boot/efi"},
				InstallLangs:     []string{"cs_CZ", "sk_SK"},
			},
			expectStages: 2,
			validate: func(t *testing.T, stages []*Stage) {
				for idx, stage := range stages {
					opts := stage.Options.(*RPMStageOptions)
					assert.Equal(t, "/custom/db", opts.DBPath, "stage %d", idx)
					assert.True(t, opts.DisableDracut, "stage %d", idx)
					assert.True(t, *opts.OSTreeBooted, "stage %d", idx)
					require.NotNil(t, opts.Exclude, "stage %d", idx)
					assert.True(t, opts.Exclude.Docs, "stage %d", idx)
					assert.Empty(t, opts.GPGKeys, "stage %d", idx)
					assert.Equal(t, "/boot/efi", opts.KernelInstallEnv.BootRoot, "stage %d", idx)
					assert.Equal(t, []string{"cs_CZ", "sk_SK"}, opts.InstallLangs, "stage %d", idx)
				}
			},
		},
		"gpgkeys-from-tree-assigned-to-correct-transaction": {
			transactions: depsolvednf.TransactionList{
				{{
					Name:     "filesystem",
					Repo:     repoNoKeys,
					Files:    []string{"/etc"},
					Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"},
				}},
				{{
					Name:     "gpg-keys",
					Repo:     repoNoKeys,
					Files:    []string{"/etc/pki/rpm-gpg/key1"},
					Checksum: rpmmd.Checksum{Type: "sha256", Value: "bbb"},
				}},
				{{
					Name:     "app",
					Repo:     repoNoKeys,
					Checksum: rpmmd.Checksum{Type: "sha256", Value: "ccc"},
				}},
			},
			baseOpts: &RPMStageOptions{
				GPGKeysFromTree: []string{"/etc/pki/rpm-gpg/key1"},
			},
			expectStages: 3,
			validate: func(t *testing.T, stages []*Stage) {
				opts0 := stages[0].Options.(*RPMStageOptions)
				opts1 := stages[1].Options.(*RPMStageOptions)
				opts2 := stages[2].Options.(*RPMStageOptions)
				assert.Empty(t, opts0.GPGKeysFromTree, "tx0 should not have key")
				assert.Equal(t, []string{"/etc/pki/rpm-gpg/key1"}, opts1.GPGKeysFromTree, "tx1 should have key")
				assert.Empty(t, opts2.GPGKeysFromTree, "tx2 should not have key")
			},
		},
		"gpgkeys-from-tree-not-found-error": {
			transactions: depsolvednf.TransactionList{
				{{Name: "pkg-a", Repo: repoNoKeys, Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"}}},
			},
			baseOpts: &RPMStageOptions{
				GPGKeysFromTree: []string{"/not/provided/by/any/package"},
			},
			expectError: "GPGKeysFromTree path",
		},
		// Error propagation from GPGKeysForPackages
		"error-nil-repo-pointer": {
			transactions: depsolvednf.TransactionList{
				{{Name: "pkg-a", Repo: nil, Checksum: rpmmd.Checksum{Type: "sha256", Value: "aaa"}}},
			},
			baseOpts:    &RPMStageOptions{},
			expectError: "package \"pkg-a\" has nil Repo pointer. This is a bug in depsolving.",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			stages, err := GenRPMStagesFromTransactions(tc.transactions, tc.baseOpts)

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			assert.Len(t, stages, tc.expectStages)

			if tc.validate != nil {
				tc.validate(t, stages)
			}
		})
	}
}
