package main

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // manifest fingerprints, not a security boundary
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/flatpak"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/ostree"
)

// manifestChecksum is the method reponsible for doing the hard work
func manifestChecksum(b []byte) string {
	sum := sha1.Sum(b) //nolint:gosec // manifest fingerprints, not a security boundary
	return hex.EncodeToString(sum[:])
}

// Checksums records manifest digests and writes them under dir.
type Checksums struct {
	dir       string
	processed sync.Map // key: checksum basename (no .json), value: hex digest
}

func newChecksums(dir string) *Checksums {
	return &Checksums{dir: dir}
}

func checksumBasename(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), ".json")
}

func writeChecksumFileIfChanged(path, digest string) error {
	want := digest + "\n"
	b, err := os.ReadFile(path)
	if err == nil && strings.TrimSuffix(string(b), "\n") == digest {
		return nil
	}
	return os.WriteFile(path, []byte(want), 0o644) //nolint:gosec // checksum artifacts are world-readable in git
}

func (c *Checksums) recordManifestChecksum(ms manifest.OSBuildManifest, depsolved map[string]depsolvednf.DepsolveResult, containers map[string][]container.Spec, commits map[string][]ostree.CommitSpec, flatpaks map[string][]flatpak.Spec, cr buildRequest, filename string, metadata bool) error {
	var buf bytes.Buffer
	if err := save(&buf, true, ms, depsolved, containers, commits, flatpaks, cr, filename, metadata); err != nil {
		return err
	}
	name := checksumBasename(filename)
	digest := manifestChecksum(buf.Bytes())
	path := filepath.Join(c.dir, name)
	if err := writeChecksumFileIfChanged(path, digest); err != nil {
		return fmt.Errorf("failed to write checksum %q: %w", path, err)
	}
	c.processed.Store(name, digest)
	return nil
}

func (c *Checksums) deleteStaleChecksums() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("failed to read checksum directory %q: %w", c.dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if _, ok := c.processed.Load(name); ok || e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(c.dir, name)); err != nil {
			return fmt.Errorf("failed to remove stale checksum %q: %w", name, err)
		}
	}
	return nil
}
