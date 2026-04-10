package check

import (
	"log"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "fw-srv-enabled",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, firewallServicesEnabledCheck)
}

func firewallServicesEnabledCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	firewall := config.Blueprint.Customizations.Firewall
	if firewall == nil || firewall.Services == nil || len(firewall.Services.Enabled) == 0 {
		return Skip("no enabled firewall services to check")
	}

	for _, service := range firewall.Services.Enabled {
		log.Printf("Checking enabled firewall service: %s\n", service)
		// NOTE: sudo works here without password because we test this only on ami
		// initialised with cloud-init, which sets sudo NOPASSWD for the user
		state, _, _, err := ExecString("sudo", "firewall-cmd", "--query-service="+service)
		if err != nil {
			return Fail("firewall service is not enabled:", service, "error:", err)
		}
		if state != "yes" {
			return Fail("firewall service is not enabled:", service, "state:", state)
		}
		log.Printf("Firewall service was enabled service=%s state=%s\n", service, state)
	}

	return Pass()
}
