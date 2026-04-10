package check

import (
	"log"
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "kernel",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
		TempDisabled:           "https://github.com/osbuild/images/pull/2175",
	}, kernelCheck)
}

func kernelCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	expected := config.Blueprint.Customizations.Kernel
	if expected == nil {
		return Skip("no kernel to check")
	}

	// Only query RPM for the kernel package provides. We do no test if the
	// specific kernel is actually booted as the testing in container is not
	// reliable.
	if expected.Name != "" {
		_, _, _, err := ExecString("rpm", "-q", "--provides", expected.Name)
		if err != nil {
			return Fail("kernel package not found:", expected.Name, "error:", err)
		}

		log.Printf("Kernel name check passed: %s is installed\n", expected.Name)
	}

	if len(expected.Append) > 0 {
		cmdline, err := ReadFile("/proc/cmdline")
		if err != nil {
			return Fail("failed to read /proc/cmdline:", err)
		}

		if !strings.Contains(string(cmdline), expected.Append) {
			return Fail("kernel options append does not match:", expected.Append)
		}
	}

	return Pass()
}
