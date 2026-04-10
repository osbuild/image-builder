package check_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/test"
)

// ExecResult holds the result of a mocked Exec call (stdout, stderr, exit code, error).
// Key for mockExec is joinArgs(name, arg...).
type ExecResult struct {
	Stdout []byte
	Stderr []byte
	Code   int
	Err    error
}

// installMockExec installs check.Exec to return values from the map keyed by joinArgs(name, arg...).
// If m is nil or the command is not in the map, returns (nil, nil, 0, nil).
func installMockExec(t *testing.T, m map[string]ExecResult) {
	t.Helper()
	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		if m == nil {
			return nil, nil, 0, nil
		}
		cmd := joinArgs(name, arg...)
		if r, ok := m[cmd]; ok {
			return r.Stdout, r.Stderr, r.Code, r.Err
		}
		return nil, nil, 0, nil
	})
}

// GrepInput is the input to Grep(pattern, filename). Used as map key for mockGrep.
type GrepInput struct {
	Pattern  string
	Filename string
}

// GrepResult holds the result of a mocked Grep call.
type GrepResult struct {
	Found bool
	Err   error
}

// installMockGrep installs check.Grep to return values from the map keyed by GrepInput.
func installMockGrep(t *testing.T, m map[GrepInput]GrepResult) {
	t.Helper()
	test.MockGlobal(t, &check.Grep, func(pattern, filename string) (bool, error) {
		if m == nil {
			return false, nil
		}
		key := GrepInput{Pattern: pattern, Filename: filename}
		if r, ok := m[key]; ok {
			return r.Found, r.Err
		}
		return false, nil
	})
}

// ReadFileResult holds the result of a mocked ReadFile call.
type ReadFileResult struct {
	Data []byte
	Err  error
}

// installMockReadFile installs check.ReadFile to return values from the map keyed by filename.
func installMockReadFile(t *testing.T, m map[string]ReadFileResult) {
	t.Helper()
	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if m == nil {
			return nil, nil
		}
		if r, ok := m[filename]; ok {
			return r.Data, r.Err
		}
		return nil, nil
	})
}

// installMockExists installs check.Exists to return the bool for each path.
// If m is nil or path not in map, returns false.
func installMockExists(t *testing.T, m map[string]bool) {
	t.Helper()
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		if m == nil {
			return false
		}
		if v, ok := m[name]; ok {
			return v
		}
		return false
	})
}

// installMockExistsDir installs check.ExistsDir to return the bool for each path.
func installMockExistsDir(t *testing.T, m map[string]bool) {
	t.Helper()
	test.MockGlobal(t, &check.ExistsDir, func(name string) bool {
		if m == nil {
			return false
		}
		if v, ok := m[name]; ok {
			return v
		}
		return false
	})
}

// StatResult holds the result of a mocked Stat call (mode, uid, gid, isDir, err).
// The mock returns a mockFileInfo built from the path and these fields.
type StatResult struct {
	Mode  fs.FileMode
	Uid   uint32
	Gid   uint32
	IsDir bool
	Err   error
}

// mockFileInfo is a mock implementation of os.FileInfo for testing.
type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	uid     uint32
	gid     uint32
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() any {
	return &syscall.Stat_t{Uid: m.uid, Gid: m.gid}
}

// installMockStat installs check.Stat to return mockFileInfo from the map keyed by path.
func installMockStat(t *testing.T, m map[string]StatResult) {
	t.Helper()
	test.MockGlobal(t, &check.Stat, func(name string) (os.FileInfo, error) {
		if m == nil {
			return nil, nil
		}
		if r, ok := m[name]; ok {
			if r.Err != nil {
				return nil, r.Err
			}
			return &mockFileInfo{
				name:  filepath.Base(name),
				mode:  r.Mode,
				uid:   r.Uid,
				gid:   r.Gid,
				isDir: r.IsDir,
			}, nil
		}
		return nil, nil
	})
}

// LookupUIDResult holds the result of a mocked LookupUID call.
type LookupUIDResult struct {
	Uid uint32
	Err error
}

// installMockLookupUID installs check.LookupUID to return values from the map keyed by username.
func installMockLookupUID(t *testing.T, m map[string]LookupUIDResult) {
	t.Helper()
	test.MockGlobal(t, &check.LookupUID, func(username string) (uint32, error) {
		if m == nil {
			return 0, nil
		}
		if r, ok := m[username]; ok {
			return r.Uid, r.Err
		}
		return 0, nil
	})
}

// LookupGIDResult holds the result of a mocked LookupGID call.
type LookupGIDResult struct {
	Gid uint32
	Err error
}

// installMockLookupGID installs check.LookupGID to return values from the map keyed by groupname.
func installMockLookupGID(t *testing.T, m map[string]LookupGIDResult) {
	t.Helper()
	test.MockGlobal(t, &check.LookupGID, func(groupname string) (uint32, error) {
		if m == nil {
			return 0, nil
		}
		if r, ok := m[groupname]; ok {
			return r.Gid, r.Err
		}
		return 0, nil
	})
}
