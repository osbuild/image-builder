package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesCheck(t *testing.T) {
	tests := []struct {
		name          string
		config        *blueprint.FileCustomization
		mockExists    map[string]bool
		mockStat      map[string]StatResult
		mockReadFile  map[string]ReadFileResult
		mockLookupUID map[string]LookupUIDResult
		mockLookupGID map[string]LookupGIDResult
		wantErr       error
	}{
		{
			name:       "basic matching file",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: int64(0)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
		},
		{
			name:       "matching file with different uid/gid",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(1000), Group: int64(1000)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 1000, Gid: 1000, IsDir: false},
			},
		},
		{
			name:       "matching file with content",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: int64(0), Data: "test content"},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockReadFile: map[string]ReadFileResult{
				"/etc/testfile": {Data: []byte("test content")},
			},
		},
		{
			name:       "non-matching mode",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0755", User: int64(0), Group: int64(0)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:       "non-matching uid",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(1000), Group: int64(0)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:       "non-matching gid",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: int64(1000)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:       "non-matching content",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: int64(0), Data: "test content"},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockReadFile: map[string]ReadFileResult{
				"/etc/testfile": {Data: []byte("different content")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:       "read file error",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: int64(0), Data: "test content"},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockReadFile: map[string]ReadFileResult{
				"/etc/testfile": {Err: errors.New("permission denied")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:       "stat error",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: int64(0)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Err: errors.New("permission denied")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:       "matching file with string username",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: "testuser", Group: "testgroup"},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 1000, Gid: 1000, IsDir: false},
			},
			mockLookupUID: map[string]LookupUIDResult{"testuser": {Uid: 1000}},
			mockLookupGID: map[string]LookupGIDResult{"testgroup": {Gid: 1000}},
		},
		{
			name:       "non-matching string username",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: "testuser", Group: int64(0)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockLookupUID: map[string]LookupUIDResult{"testuser": {Uid: 1000}},
			wantErr:       check.ErrCheckFailed,
		},
		{
			name:       "non-matching string groupname",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: "testgroup"},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockLookupGID: map[string]LookupGIDResult{"testgroup": {Gid: 1000}},
			wantErr:       check.ErrCheckFailed,
		},
		{
			name:       "lookup uid error",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: "nonexistent", Group: int64(0)},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockLookupUID: map[string]LookupUIDResult{"nonexistent": {Err: errors.New("user not found")}},
			wantErr:       check.ErrCheckFailed,
		},
		{
			name:       "lookup gid error",
			config:     &blueprint.FileCustomization{Path: "/etc/testfile", Mode: "0644", User: int64(0), Group: "nonexistent"},
			mockExists: map[string]bool{"/etc/testfile": true},
			mockStat: map[string]StatResult{
				"/etc/testfile": {Mode: 0644, Uid: 0, Gid: 0, IsDir: false},
			},
			mockLookupGID: map[string]LookupGIDResult{"nonexistent": {Err: errors.New("group not found")}},
			wantErr:       check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExists(t, tt.mockExists)
			installMockStat(t, tt.mockStat)
			installMockReadFile(t, tt.mockReadFile)
			installMockLookupUID(t, tt.mockLookupUID)
			installMockLookupGID(t, tt.mockLookupGID)

			chk, found := check.FindCheckByName("files")
			require.True(t, found, "Files Check not found")
			config := buildConfig(&blueprint.Customizations{
				Files: []blueprint.FileCustomization{*tt.config},
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
