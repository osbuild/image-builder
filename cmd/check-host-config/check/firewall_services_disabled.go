package check

import (
	"log"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "fw-srv-disabled",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, firewallServicesDisabledCheck)
}

func firewallServicesDisabledCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	firewall := config.Blueprint.Customizations.Firewall
	if firewall == nil || firewall.Services == nil || len(firewall.Services.Disabled) == 0 {
		return Skip("no disabled firewall services to check")
	}

	for _, service := range firewall.Services.Disabled {
		log.Printf("Checking disabled firewall service: %s\n", service)
		// NOTE: sudo works here without password because we test this only on ami
		// initialised with cloud-init, which sets sudo NOPASSWD for the user
		state, _, code, err := ExecString("sudo", "firewall-cmd", "--query-service="+service)
		if err != nil && code != 1 { // 1 is the exit code for "service not found"
			return Fail("problem checking firewall service:", service, "error:", err)
		}
		if state == "yes" {
			return Fail("firewall service is not disabled:", service, "state:", state)
		}
		log.Printf("Firewall service was disabled service=%s state=%s\n", service, state)
	}

	return Pass()
}
