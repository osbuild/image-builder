package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/osbuild/image-builder-cli/internal/olog"
)

// IsMountpoint checks if the target path is a mount point
func IsMountpoint(path string) bool {
	return exec.Command("mountpoint", path).Run() == nil
}

// Synchronously invoke a command, propagating stdout and stderr
// to the current process's stdout and stderr
func RunCmdSync(cmdName string, args ...string) error {
	olog.Printf("Running: %s %s", cmdName, strings.Join(args, " "))
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running %s %s: %w", cmdName, strings.Join(args, " "), err)
	}
	return nil
}

// OutputErr takes an error from exec.Command().Output() and tries
// generate an error with stderr details
func OutputErr(err error) error {
	if err, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%w, stderr:\n%s", err, err.Stderr)
	}
	return err
}

// ShortenString shortens a string to the specified length. If the string is
// longer than length, it appends a unicode ellipsis character. If length is 0,
// it returns the unmodified string.
func ShortenString(msg string, length int) string {
	if length > 0 && len(msg) > length {
		return msg[:length-1] + "…"
	}
	return msg
}
