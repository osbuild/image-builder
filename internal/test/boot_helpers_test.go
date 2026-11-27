package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	if !IsRegularFile(path) {
		t.Errorf("IsRegularFile(%q) = false, want true", path)
	}
	if IsRegularFile(filepath.Dir(path)) {
		t.Error("IsRegularFile(directory) = true, want false")
	}
	if IsRegularFile(filepath.Join(path, "missing")) {
		t.Error("IsRegularFile(missing) = true, want false")
	}
}
