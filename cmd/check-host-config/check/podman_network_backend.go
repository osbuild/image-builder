package check

import (
	"encoding/json"
	"log"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:              "podman-network-backend",
		RequiresBlueprint: true,
	}, podmanNetworkBackendCheck)
}

type podmanInfo struct {
	Host struct {
		NetworkBackend string `json:"networkBackend"`
	} `json:"host"`
}

func getPodmanNetworkBackend(sudo bool) (string, error) {
	var stdout []byte
	var err error

	if sudo {
		stdout, _, _, err = Exec("sudo", "podman", "info", "--format", "json")
	} else {
		stdout, _, _, err = Exec("podman", "info", "--format", "json")
	}
	if err != nil {
		return "", err
	}

	var info podmanInfo
	if err := json.Unmarshal(stdout, &info); err != nil {
		return "", err
	}

	backend := info.Host.NetworkBackend
	if backend == "" {
		backend = "undefined"
	}
	return backend, nil
}

// podmanNetworkBackendCheck verifies that rootful and rootless podman use the
// same network backend. When containers are embedded into the image as root,
// certain podman versions may interpret the existing storage as a migration
// and fall back to 'cni' for rootful only, creating an inconsistency.
func podmanNetworkBackendCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	if len(config.Blueprint.Containers) == 0 {
		return Skip("no embedded containers")
	}

	rootful, err := getPodmanNetworkBackend(true)
	if err != nil {
		return Fail("failed to get rootful podman network backend:", err)
	}
	log.Printf("Rootful podman network backend: %s\n", rootful)

	rootless, err := getPodmanNetworkBackend(false)
	if err != nil {
		return Fail("failed to get rootless podman network backend:", err)
	}
	log.Printf("Rootless podman network backend: %s\n", rootless)

	if rootful != rootless {
		return Fail("podman network backends are inconsistent:", "rootful="+rootful, "rootless="+rootless)
	}

	return Pass()
}
