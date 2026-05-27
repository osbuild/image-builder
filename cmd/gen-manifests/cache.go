package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/osbuild"
)

// initCacheDir initializes the cache directory
// Uses XDG_CACHE_HOME or /var/cache for root
func initCacheDir() error {
	var baseDir string

	if os.Geteuid() == 0 {
		// Running as root, use /var/cache
		baseDir = "/var/cache"
	} else {
		// Check XDG_CACHE_HOME first
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			baseDir = xdg
		} else {
			// Fallback to ~/.cache
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			baseDir = filepath.Join(home, ".cache")
		}
	}

	cacheDir = filepath.Join(baseDir, "osbuild-gen-manifests")
	return os.MkdirAll(cacheDir, 0755)
}

// getOSBuildVersion gets the osbuild version, using cache if the binary hasn't changed
func getOSBuildVersion() (string, error) {
	// Find osbuild binary path
	osbuildPath, err := exec.LookPath("osbuild")
	if err != nil {
		return "", fmt.Errorf("osbuild not found in PATH: %w", err)
	}

	// Get osbuild binary modification time
	binaryInfo, err := os.Stat(osbuildPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat osbuild binary: %w", err)
	}
	binaryModTime := binaryInfo.ModTime()

	// Check cached version file
	versionPath := filepath.Join(cacheDir, "version")
	if versionInfo, err := os.Stat(versionPath); err == nil {
		// Version file exists, check if it's newer than the binary
		if versionInfo.ModTime().After(binaryModTime) {
			// Cache is newer, use it
			cachedVersion, err := os.ReadFile(versionPath)
			if err == nil && len(cachedVersion) > 0 {
				return strings.TrimSpace(string(cachedVersion)), nil
			}
		}
	}

	// Cache miss or binary is newer - call osbuild --version
	version, err := osbuild.OSBuildVersion()
	if err != nil {
		return "", err
	}

	// Write to cache
	if err := os.WriteFile(versionPath, []byte(version), 0600); err != nil {
		// Non-fatal - just continue without caching
		fmt.Fprintf(os.Stderr, "Warning: failed to cache osbuild version: %v\n", err)
	}

	return version, nil
}

// getCachePath returns the cache file path for a given SHA256 hash
// Uses first 2 characters as subdirectory for better filesystem performance
func getCachePath(sha256sum string) string {
	if len(sha256sum) < 2 {
		return ""
	}
	subdir := sha256sum[0:2]
	return filepath.Join(cacheDir, subdir, sha256sum)
}

// readCache attempts to read cached osbuild inspect output
// Returns empty string if cache miss or error
func readCache(sha256sum string) string {
	cachePath := getCachePath(sha256sum)
	if cachePath == "" {
		return ""
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		// Cache miss or read error - treat as miss
		return ""
	}

	return string(data)
}

// writeCache stores osbuild inspect output in cache
func writeCache(sha256sum, output string) error {
	cachePath := getCachePath(sha256sum)
	if cachePath == "" {
		return fmt.Errorf("invalid cache path for hash: %s", sha256sum)
	}

	// Create subdirectory if needed
	subdir := filepath.Dir(cachePath)
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	return os.WriteFile(cachePath, []byte(output), 0600)
}

// appendHistory appends a cache key to the history file for a manifest
// History files are stored in cache_root/hist/<checksumKey> with one cache key per line
// Cache keys are SHA256(canonical JSON + osbuild version)
// History entries are stored as: "timestamp cacheKey" (UTC Unix timestamp in seconds)
func appendHistory(checksumKey, cacheKey string) error {
	histDir := filepath.Join(cacheDir, "hist")
	if err := os.MkdirAll(histDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	histPath := filepath.Join(histDir, checksumKey)

	// Check if this cache key is already in the history
	if data, err := os.ReadFile(histPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			// Extract cache key from "timestamp cacheKey" format
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 2 && parts[1] == cacheKey {
				// Already in history, nothing to do
				return nil
			}
		}
	}

	// Append timestamp and cache key (UTC Unix timestamp in seconds)
	timestamp := time.Now().UTC().Unix()
	f, err := os.OpenFile(histPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%d %s\n", timestamp, cacheKey); err != nil {
		return fmt.Errorf("failed to write to history file: %w", err)
	}

	return nil
}

// readHistory reads all cache keys from the history file for a manifest
// Returns cache keys (SHA256 of canonical JSON + osbuild version)
func readHistory(checksumKey string) ([]string, error) {
	histPath := filepath.Join(cacheDir, "hist", checksumKey)
	data, err := os.ReadFile(histPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var sums []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sums = append(sums, line)
		}
	}

	return sums, nil
}

// generateCacheKey generates a cache key by hashing canonical JSON + osbuild version
// This ensures different osbuild versions naturally use different cache entries
func generateCacheKey(manifestJSON []byte, osbuildVersion string) (string, error) {
	// Round-trip through any to ensure all map keys are sorted
	var canonical any
	if err := json.Unmarshal(manifestJSON, &canonical); err != nil {
		return "", fmt.Errorf("failed to unmarshal for canonicalization: %w", err)
	}

	canonicalBytes, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("failed to marshal canonical form: %w", err)
	}

	h := sha256.New()
	h.Write(canonicalBytes)
	h.Write([]byte(osbuildVersion))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// isSHA256Hex checks if a string is a valid 64-character SHA256 hex string
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
