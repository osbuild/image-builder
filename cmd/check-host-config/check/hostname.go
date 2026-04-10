package check

import (
	"errors"
	"fmt"
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
)

const hostnameFilePath = "/etc/hostname"

func init() {
	RegisterCheck(Metadata{
		Name:                   "hostname",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, hostnameCheck)
}

var ErrHostname = errors.New("hostname")

func getHostname() (string, error) {
	if hostname, _, _, err := ExecString("hostnamectl", "hostname"); err == nil && hostname != "" {
		return hostname, nil
	}

	if hostname, _, _, err := ExecString("hostname"); err == nil && hostname != "" {
		return hostname, nil
	}

	data, err := ReadFile(hostnameFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: could not read %q", ErrHostname, hostnameFilePath)
	}

	firstLine, _, _ := strings.Cut(string(data), "\n")
	hostname := strings.TrimSpace(firstLine)
	if hostname != "" {
		return hostname, nil
	}
	return "", fmt.Errorf("%w: could not get hostname: tried hostnamectl, hostname, and %s", ErrHostname, hostnameFilePath)
}

func hostnameCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	expected := config.Blueprint.Customizations.Hostname
	if expected == nil || *expected == "" {
		return Skip("no hostname customization")
	}

	hostname, err := getHostname()
	if err != nil {
		return err
	}

	// we only emit a warning here since the hostname gets reset by cloud-init and we're not
	// entirely sure how to deal with it yet on the service level
	if hostname != *expected {
		return Warning("hostname does not match, got", hostname, "expected", *expected)
	}

	return Pass()
}
