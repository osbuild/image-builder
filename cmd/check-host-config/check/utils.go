package check

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/osbuild/images/pkg/distro"
)

// OSRelease contains parsed fields from /etc/os-release
type OSRelease struct {
	ID           string
	VersionID    string
	Version      string
	MajorVersion int // Extracted major version from VersionID (e.g., 9 from "9.0")
}

// ParseOSRelease is a mockable function that reads and parses /etc/os-release file.
// The default implementation calls distro.ReadOSReleaseFromTree("/") to read from
// the system root, which automatically tries /etc/os-release and /usr/lib/os-release.
// The osReleasePath parameter is kept for API compatibility but ignored in the default implementation.
var ParseOSRelease = func(osReleasePath string) (*OSRelease, error) {
	log.Printf("ParseOSRelease: reading from system root\n")
	osrelease, err := distro.ReadOSReleaseFromTree("/")
	if err != nil {
		log.Printf("ParseOSRelease failed: %v\n", err)
		return nil, err
	}

	release := &OSRelease{
		ID:        osrelease["ID"],
		VersionID: osrelease["VERSION_ID"],
		Version:   osrelease["VERSION"],
	}

	// Extract major version from VersionID (e.g., "9.0" -> 9)
	if release.VersionID != "" {
		majorVersionStr := release.VersionID
		if idx := strings.Index(majorVersionStr, "."); idx != -1 {
			majorVersionStr = majorVersionStr[:idx]
		}
		majorVersion, err := strconv.Atoi(majorVersionStr)
		if err != nil {
			// If parsing fails, leave MajorVersion as 0 (zero value)
			// This allows callers to check for 0 to detect invalid versions
			release.MajorVersion = 0
		} else {
			release.MajorVersion = majorVersion
		}
	}

	return release, nil
}

// ExecCommand is mockable version of os/exec.Command
var ExecCommand = exec.Command

// Exec is mockable version of os/exec.Command.Run
var Exec = func(name string, arg ...string) ([]byte, []byte, int, error) {
	cmdStr := name
	if len(arg) > 0 {
		cmdStr += " " + strings.Join(arg, " ")
	}

	cmd := exec.Command(name, arg...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		log.Printf("Exec: %s (%s)\n%s\n%s", cmdStr, err, stdoutBuf.String(), stderrBuf.String())
	} else {
		log.Printf("Exec: %s\n", cmdStr)
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode, err
}

// ExecString is a convenience function that returns the stdout and stderr as strings
// and trims the whitespace. It uses mockable Exec.
func ExecString(name string, arg ...string) (string, string, int, error) {
	stdout, stderr, exitCode, err := Exec(name, arg...)
	return strings.TrimSpace(string(stdout)), strings.TrimSpace(string(stderr)), exitCode, err
}

// Exists is mockable version of os.Stat
var Exists = func(name string) bool {
	log.Printf("Exists: %s\n", name)
	_, err := os.Stat(name)
	exists := !os.IsNotExist(err)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Exists failed: %s (error: %v)\n", name, err)
	}
	return exists
}

// ExistsDir is mockable version that checks if a path exists and is a directory
var ExistsDir = func(name string) bool {
	log.Printf("ExistsDir: %s\n", name)
	info, err := os.Stat(name)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("ExistsDir failed: %s (error: %v)\n", name, err)
		}
		return false
	}
	return info.IsDir()
}

// Stat is mockable version of os.Stat
var Stat = func(name string) (os.FileInfo, error) {
	log.Printf("Stat: %s\n", name)
	return os.Stat(name)
}

// Grep is mockable version of os.ReadFile with grep capabilities
var Grep = func(pattern, filename string) (bool, error) {
	log.Printf("Grep: %s %s\n", pattern, filename)
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("Grep failed: %s %s (error: %v)\n", pattern, filename, err)
		return false, err
	}
	return strings.Contains(string(content), pattern), nil
}

// ReadFile is mockable version of os.ReadFile
var ReadFile = func(filename string) ([]byte, error) {
	log.Printf("ReadFile: %s\n", filename)
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("ReadFile failed: %s (error: %v)\n", filename, err)
	}
	return data, err
}

// LookupUID is a mockable function that looks up a user by name and returns the UID.
// The default implementation uses os/user.Lookup.
var LookupUID = func(username string) (uint32, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse UID: %w", err)
	}
	return uint32(uid), nil
}

// LookupGID is a mockable function that looks up a group by name and returns the GID.
// The default implementation uses os/user.LookupGroup.
var LookupGID = func(groupname string) (uint32, error) {
	g, err := user.LookupGroup(groupname)
	if err != nil {
		return 0, err
	}
	gid, err := strconv.ParseUint(g.Gid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse GID: %w", err)
	}
	return uint32(gid), nil
}

// resolveUser converts an any value (string, int, or int64) to a uint32 UID.
// If the value is a string, it looks up the user name. If it's numeric, it converts directly.
//
//nolint:gosec // G115: caller guarantees UID is in uint32 range
func resolveUser(value any) (uint32, error) {
	switch v := value.(type) {
	case string:
		return LookupUID(v)
	case int:
		return uint32(v), nil
	case int64:
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("unsupported type for user: %T (expected string, int, or int64)", value)
	}
}

// resolveGroup converts an any value (string, int, or int64) to a uint32 GID.
// If the value is a string, it looks up the group name. If it's numeric, it converts directly.
//
//nolint:gosec // G115: caller guarantees GID is in uint32 range
func resolveGroup(value any) (uint32, error) {
	switch v := value.(type) {
	case string:
		return LookupGID(v)
	case int:
		return uint32(v), nil
	case int64:
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("unsupported type for group: %T (expected string, int, or int64)", value)
	}
}
