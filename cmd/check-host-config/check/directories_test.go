package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectoriesCheck(t *testing.T) {
	tests := []struct {
		name          string
		config        *blueprint.DirectoryCustomization
		mockExistsDir map[string]bool
		mockStat      map[string]StatResult
		mockLookupUID map[string]LookupUIDResult
		mockLookupGID map[string]LookupGIDResult
		wantErr       error
	}{
		{
			name: "basic matching directory",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/testdir",
				Mode: "0755",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
		},
		{
			name: "matching directory with different uid/gid",
			config: &blueprint.DirectoryCustomization{
				Path:  "/etc/testdir",
				Mode:  "0755",
				User:  int64(1000),
				Group: int64(1000),
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 1000, Gid: 1000, IsDir: true},
			},
		},
		{
			name: "matching directory without mode",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/testdir",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
		},
		{
			name: "non-matching mode",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/testdir",
				Mode: "0700",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "non-matching uid",
			config: &blueprint.DirectoryCustomization{
				Path:  "/etc/testdir",
				Mode:  "0755",
				User:  int64(1000),
				Group: int64(0),
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "non-matching gid",
			config: &blueprint.DirectoryCustomization{
				Path:  "/etc/testdir",
				Mode:  "0755",
				Group: int64(1000),
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "stat error",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/testdir",
				Mode: "0755",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Err: errors.New("permission denied")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "path is not a directory",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/testfile",
				Mode: "0644",
			},
			mockExistsDir: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "matching directory with string username",
			config: &blueprint.DirectoryCustomization{
				Path:  "/etc/testdir",
				Mode:  "0755",
				User:  "testuser",
				Group: "testgroup",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 1000, Gid: 1000, IsDir: true},
			},
			mockLookupUID: map[string]LookupUIDResult{"testuser": {Uid: 1000}},
			mockLookupGID: map[string]LookupGIDResult{"testgroup": {Gid: 1000}},
		},
		{
			name: "lookup uid error",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/testdir",
				Mode: "0755",
				User: "nonexistent",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
			mockLookupUID: map[string]LookupUIDResult{"nonexistent": {Err: errors.New("user not found")}},
			wantErr:       check.ErrCheckFailed,
		},
		{
			name: "lookup gid error",
			config: &blueprint.DirectoryCustomization{
				Path:  "/etc/testdir",
				Mode:  "0755",
				Group: "nonexistent",
			},
			mockExistsDir: map[string]bool{"/etc/testdir": true},
			mockStat: map[string]StatResult{
				"/etc/testdir": {Mode: 0755, Uid: 0, Gid: 0, IsDir: true},
			},
			mockLookupGID: map[string]LookupGIDResult{"nonexistent": {Err: errors.New("group not found")}},
			wantErr:       check.ErrCheckFailed,
		},
		{
			name: "directory does not exist",
			config: &blueprint.DirectoryCustomization{
				Path: "/etc/nonexistent",
				Mode: "0755",
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExistsDir(t, tt.mockExistsDir)
			installMockStat(t, tt.mockStat)
			installMockLookupUID(t, tt.mockLookupUID)
			installMockLookupGID(t, tt.mockLookupGID)

			chk, found := check.FindCheckByName("directories")
			require.True(t, found, "Directories Check not found")
			config := buildConfig(&blueprint.Customizations{
				Directories: []blueprint.DirectoryCustomization{*tt.config},
			})

			err := chk.Func(chk.Meta, config)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
