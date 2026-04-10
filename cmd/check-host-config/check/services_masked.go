package check

import (
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "srv-masked",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, servicesMaskedCheck)
}

func servicesMaskedCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	services := config.Blueprint.Customizations.Services
	if services == nil || len(services.Masked) == 0 {
		return Skip("no masked services to check")
	}

	stdout, _, _, err := ExecString("systemctl", "list-unit-files", "--state=masked")
	if err != nil {
		return Fail("failed to list masked services:", err)
	}

	for _, service := range services.Masked {
		// Prevent false positives by appending suffix if it is not present
		if !strings.Contains(service, ".") {
			service = service + ".service"
		}

		if !strings.Contains(stdout, service) {
			return Fail("service is not masked:", service)
		}
	}

	return Pass()
}
