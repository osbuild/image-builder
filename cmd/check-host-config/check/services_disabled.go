package check

import (
	"log"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "srv-disabled",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, servicesDisabledCheck)
}

func servicesDisabledCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	services := config.Blueprint.Customizations.Services
	if services == nil || len(services.Disabled) == 0 {
		return Skip("no disabled services to check")
	}

	for _, service := range services.Disabled {
		log.Printf("Checking disabled service: %s\n", service)
		// systemctl is-enabled returns non-zero exit code for disabled services,
		// but still outputs "disabled", so we check the output regardless of error
		state, _, _, err := ExecString("systemctl", "is-enabled", service)
		if state == "" && err != nil {
			// If we got no output and an error, the service might not exist
			return Fail("service is not disabled:", service, "error:", err)
		}

		if state != "disabled" {
			return Fail("service is not disabled:", service, "state:", state)
		}
		log.Printf("Service was disabled service=%s state=%s\n", service, state)
	}

	return Pass()
}
