package main

import (
	"fmt"

	"github.com/osbuild/images/pkg/distro"
)

var distroGetHostDistroName = distro.GetHostDistroName

// findDistro will ensure that the given distro argument do not
// diverge. If no distro is set via the blueprint or the argument
// the host is used to derive the distro.
func findDistro(argDistroName, bpDistroName string) (string, error) {
	switch {
	case argDistroName != "" && bpDistroName != "" && argDistroName != bpDistroName:
		return "", fmt.Errorf("error selecting distro name, cmdline argument %q is different from blueprint %q", argDistroName, bpDistroName)
	case argDistroName != "":
		return argDistroName, nil
	case bpDistroName != "":
		return bpDistroName, nil
	}
	// nothing selected by the user, derive from host
	distroStr, err := distroGetHostDistroName()
	if err != nil {
		return "", fmt.Errorf("error deriving host distro %w", err)
	}
	fmt.Fprintf(osStderr, "No distro name specified, selecting %q based on host, use --distro to override", distroStr)
	return distroStr, nil
}
