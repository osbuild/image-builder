package check

import (
	"log"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "srv-enabled",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, servicesEnabledCheck)
}

func servicesEnabledCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	services := config.Blueprint.Customizations.Services
	if services == nil || len(services.Enabled) == 0 {
		return Skip("no enabled services to check")
	}

	for _, service := range services.Enabled {
		log.Printf("Checking enabled service: %s\n", service)
		state, _, _, err := ExecString("systemctl", "is-enabled", service)
		if err != nil {
			return Fail("service is not enabled:", service, "error:", err)
		}
		if state != "enabled" {
			return Fail("service is not enabled:", service, "state:", state)
		}
		log.Printf("Service was enabled service=%s state=%s\n", service, state)
	}

	return Pass()
}
