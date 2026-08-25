package bootc

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/bib/osinfo"
)

func newTestFS(t *testing.T, root string) podmanUnshareFS {
	t.Helper()
	for _, tool := range []string{"cat", "stat", "find", "test"} {
		if _, err := exec.LookPath(tool); err != nil {
			// "test" is usually a shell builtin, resolve via sh if missing
			if tool == "test" {
				continue
			}
			t.Skipf("skipping: %q not found in PATH", tool)
		}
	}
	return podmanUnshareFS{root: root}
}

func TestPodmanUnshareFSReadFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello"), []byte("world"), 0644))
	fsys := newTestFS(t, root)

	content, err := fs.ReadFile(fsys, "hello")
	require.NoError(t, err)
	assert.Equal(t, "world", string(content))

	_, err = fs.ReadFile(fsys, "missing")
	assert.True(t, os.IsNotExist(err), "expected IsNotExist, got %v", err)
}

func TestPodmanUnshareFSStat(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("12345"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0755))
	fsys := newTestFS(t, root)

	fi, err := fs.Stat(fsys, "file")
	require.NoError(t, err)
	assert.False(t, fi.IsDir())
	assert.Equal(t, "file", fi.Name())
	assert.Equal(t, int64(5), fi.Size())

	fi, err = fs.Stat(fsys, "dir")
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	_, err = fs.Stat(fsys, "nope")
	assert.True(t, os.IsNotExist(err), "expected IsNotExist, got %v", err)
}

func TestPodmanUnshareFSStatFollowsSymlink(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "real"), 0755))
	require.NoError(t, os.Symlink("real", filepath.Join(root, "link")))
	fsys := newTestFS(t, root)

	fi, err := fs.Stat(fsys, "link")
	require.NoError(t, err)
	assert.True(t, fi.IsDir(), "stat should follow the symlink to the directory")
}

func TestPodmanUnshareFSReadDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "b-file"), nil, 0644))
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub", "a-dir"), 0755))
	fsys := newTestFS(t, root)

	entries, err := fs.ReadDir(fsys, "sub")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	// entries must be sorted by name
	assert.Equal(t, "a-dir", entries[0].Name())
	assert.True(t, entries[0].IsDir())
	assert.Equal(t, "b-file", entries[1].Name())
	assert.False(t, entries[1].IsDir())

	_, err = fs.ReadDir(fsys, "does-not-exist")
	assert.True(t, os.IsNotExist(err), "expected IsNotExist, got %v", err)
}

func TestPodmanUnshareFSGlob(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "usr/lib/bootupd/updates/EFI")
	require.NoError(t, os.MkdirAll(filepath.Join(base, "fedora"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "BOOT"), 0755))
	fsys := newTestFS(t, root)

	matches, err := fs.Glob(fsys, "usr/lib/bootupd/updates/EFI/*")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"usr/lib/bootupd/updates/EFI/BOOT",
		"usr/lib/bootupd/updates/EFI/fedora",
	}, matches)
}

func TestPodmanUnshareFSOpen(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "data"), []byte("payload"), 0644))
	fsys := newTestFS(t, root)

	f, err := fsys.Open("data")
	require.NoError(t, err)
	defer f.Close()

	fi, err := f.Stat()
	require.NoError(t, err)
	assert.Equal(t, "data", fi.Name())

	buf := make([]byte, 7)
	n, err := f.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(buf[:n]))
}

func TestPodmanUnshareFSOpenDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dir", "b-file"), nil, 0644))
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir", "a-dir"), 0755))
	fsys := newTestFS(t, root)

	f, err := fsys.Open("dir")
	require.NoError(t, err)
	defer f.Close()

	fi, err := f.Stat()
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// Reading bytes from a directory must fail.
	_, err = f.Read(make([]byte, 1))
	assert.Error(t, err)

	// The returned file must implement fs.ReadDirFile.
	rdf, ok := f.(fs.ReadDirFile)
	require.True(t, ok, "directory file must implement fs.ReadDirFile")

	// Paged reads: one entry at a time, then io.EOF.
	first, err := rdf.ReadDir(1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, "a-dir", first[0].Name())

	second, err := rdf.ReadDir(1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Equal(t, "b-file", second[0].Name())

	_, err = rdf.ReadDir(1)
	assert.ErrorIs(t, err, io.EOF)
}

func TestPodmanUnshareFSOpenMissing(t *testing.T) {
	fsys := newTestFS(t, t.TempDir())

	_, err := fsys.Open("nope")
	assert.True(t, os.IsNotExist(err), "expected IsNotExist, got %v", err)

	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr)
	assert.Equal(t, "open", pathErr.Op)
}

func TestPodmanUnshareFSTestFS(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello"), []byte("world"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub/nested"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub/file"), []byte("data"), 0644))
	fsys := newTestFS(t, root)

	require.NoError(t, fstest.TestFS(fsys, "hello", "sub/file", "sub/nested"))
}

func TestPodmanUnshareFSWithOsinfo(t *testing.T) {
	root := t.TempDir()
	fsys := newTestFS(t, root)

	writeFile := func(rel, content string) {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	}

	writeFile("etc/os-release", `ID="fedora"
VERSION_ID="40"
NAME="Fedora Linux"
PLATFORM_ID="platform:f40"
`)
	writeFile("usr/lib/modules/6.1.0-1.fc40.x86_64/vmlinuz", "kernel")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "usr/lib/bootupd/updates/EFI/fedora"), 0755))

	info, err := osinfo.Load(fsys)
	require.NoError(t, err)
	assert.Equal(t, "fedora", info.OSRelease.ID)
	assert.Equal(t, "40", info.OSRelease.VersionID)
	assert.Equal(t, "Fedora Linux", info.OSRelease.Name)
	assert.Equal(t, "fedora", info.UEFIVendor)
	require.NotNil(t, info.KernelInfo)
	assert.Equal(t, "6.1.0-1.fc40.x86_64", info.KernelInfo.Version)
}
