package depsolvednf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionListAllPackages(t *testing.T) {
	transactions := TransactionList{
		{
			{Name: "pkg-b", Epoch: 0, Version: "1.0", Release: "1", Arch: "x86_64"},
			{Name: "pkg-a", Epoch: 0, Version: "1.0", Release: "1", Arch: "x86_64"},
		},
		{
			{Name: "pkg-c", Epoch: 0, Version: "1.0", Release: "1", Arch: "x86_64"},
		},
	}

	all := transactions.AllPackages()
	require.Len(t, all, 3)
	// Should be sorted by full NEVRA
	expectedOrder := []string{"pkg-a", "pkg-b", "pkg-c"}
	for i := range all {
		assert.Equal(t, expectedOrder[i], all[i].Name)
	}
}

func TestTransactionListAllPackagesEmpty(t *testing.T) {
	transactions := TransactionList{}
	all := transactions.AllPackages()
	assert.Empty(t, all)
}

func TestTransactionListFindPackage(t *testing.T) {
	transactions := TransactionList{
		{{Name: "pkg-a"}},
		{{Name: "pkg-b"}, {Name: "pkg-c"}},
	}

	pkg, err := transactions.FindPackage("pkg-b")
	require.NoError(t, err)
	assert.Equal(t, "pkg-b", pkg.Name)

	pkg, err = transactions.FindPackage("pkg-c")
	require.NoError(t, err)
	assert.Equal(t, "pkg-c", pkg.Name)

	_, err = transactions.FindPackage("not-found")
	assert.Error(t, err)
}

func TestTransactionListGetFilesTransactionInfo(t *testing.T) {
	defaultTransactions := TransactionList{
		{
			{Name: "filesystem", Files: []string{"/", "/usr", "/bin"}},
		},
		{
			{Name: "bash", Files: []string{"/bin/bash", "/usr/bin/bash"}},
			{Name: "coreutils", Files: []string{"/bin/ls", "/bin/cat"}},
		},
	}

	// TransactionList where the same file appears in multiple transactions
	duplicateFileTransactions := TransactionList{
		{
			{Name: "pkg-a", Files: []string{"/common/file"}},
		},
		{
			{Name: "pkg-b", Files: []string{"/common/file"}},
		},
	}

	tests := map[string]struct {
		transactions TransactionList
		paths        []string
		expected     map[string]TransactionFileInfo
	}{
		"empty-paths": {
			transactions: defaultTransactions,
			paths:        []string{},
			expected:     nil,
		},
		"single-file-found": {
			transactions: defaultTransactions,
			paths:        []string{"/usr/bin/bash"},
			expected: map[string]TransactionFileInfo{
				"/usr/bin/bash": {Path: "/usr/bin/bash", TxIndex: 1},
			},
		},
		"multiple-files-different-transactions": {
			transactions: defaultTransactions,
			paths:        []string{"/usr", "/bin/cat"},
			expected: map[string]TransactionFileInfo{
				"/usr":     {Path: "/usr", TxIndex: 0},
				"/bin/cat": {Path: "/bin/cat", TxIndex: 1},
			},
		},
		"file-not-found-excluded-from-result": {
			transactions: defaultTransactions,
			paths:        []string{"/usr", "/not/found"},
			expected: map[string]TransactionFileInfo{
				"/usr": {Path: "/usr", TxIndex: 0},
			},
		},
		"all-files-not-found": {
			transactions: defaultTransactions,
			paths:        []string{"/not/found", "/also/not/found"},
			expected:     map[string]TransactionFileInfo{},
		},
		"earliest-transaction-wins": {
			transactions: duplicateFileTransactions,
			paths:        []string{"/common/file"},
			expected: map[string]TransactionFileInfo{
				"/common/file": {Path: "/common/file", TxIndex: 0},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.transactions.GetFilesTransactionInfo(tc.paths)

			if tc.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.Len(t, result, len(tc.expected))
			for path, expectedInfo := range tc.expected {
				actualInfo, found := result[path]
				require.True(t, found, "expected path %q not found in result", path)
				assert.Equal(t, expectedInfo.Path, actualInfo.Path)
				assert.Equal(t, expectedInfo.TxIndex, actualInfo.TxIndex)
				assert.NotNil(t, actualInfo.Package, "Package pointer should not be nil")
			}
		})
	}
}
