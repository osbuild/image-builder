package check

import (
	"log"
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "fw-ports",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, firewallPortsCheck)
}

func firewallPortsCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	firewall := config.Blueprint.Customizations.Firewall
	if firewall == nil || len(firewall.Ports) == 0 {
		return Skip("no firewall ports to check")
	}

	for _, port := range firewall.Ports {
		// firewall-cmd --query-port uses / as the port/protocol separator, but
		// in the blueprint we use :.
		portQuery := strings.ReplaceAll(port, ":", "/")
		log.Printf("Checking enabled firewall port: %s\n", portQuery)
		// NOTE: sudo works here without password because we test this only on ami
		// initialised with cloud-init, which sets sudo NOPASSWD for the user
		state, _, _, err := ExecString("sudo", "firewall-cmd", "--query-port="+portQuery)
		if err != nil {
			return Fail("firewall port is not enabled:", port, "error:", err)
		}
		if state != "yes" {
			return Fail("firewall port is not enabled:", port, "state:", state)
		}
		log.Printf("Firewall port was enabled port=%s state=%s\n", portQuery, state)
	}

	return Pass()
}
