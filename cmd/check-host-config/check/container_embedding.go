package check

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:              "container-embedding",
		RequiresBlueprint: true,
	}, containerEmbeddingCheck)
}

type podmanImage struct {
	Names []string `json:"Names"`
}

// containerNameMatches reports whether a podman image name matches the
// expected needle. Short names (without a domain/path component) are
// normalized by the container runtime: skopeo/containers-storage adds
// "docker.io/library/" (the Docker default) while locally-built images
// may get "localhost/". We check the needle against all known
// normalizations.
func containerNameMatches(podmanName, needle string) bool {
	candidates := []string{needle}
	nameBeforeTag := strings.SplitN(needle, ":", 2)[0]
	if !strings.Contains(nameBeforeTag, "/") {
		candidates = append(candidates,
			"localhost/"+needle,
			"docker.io/library/"+needle,
		)
	}
	for _, c := range candidates {
		if podmanName == c || strings.HasPrefix(podmanName, c+":") {
			return true
		}
	}
	return false
}

func containerEmbeddingCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	containers := config.Blueprint.Containers
	if len(containers) == 0 {
		return Skip("no containers to check")
	}

	stdout, _, _, err := Exec("sudo", "podman", "images", "--format", "json")
	if err != nil {
		return Fail("failed to list podman images:", err)
	}

	var images []podmanImage
	if err := json.Unmarshal(stdout, &images); err != nil {
		return Fail("failed to parse podman images output:", err)
	}

	for _, ctr := range containers {
		// The blueprint Name, when set, is used as the local name for the
		// container in the image storage (see Spec.LocalName). When empty,
		// the source reference is used instead.
		needle := ctr.Source
		if ctr.Name != "" {
			needle = ctr.Name
		}
		if needle == "" {
			continue
		}

		found := false
		for _, img := range images {
			for _, name := range img.Names {
				if containerNameMatches(name, needle) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			return Fail(fmt.Sprintf("embedded container %q (source %q) not found in podman images", needle, ctr.Source))
		}
		log.Printf("Container %q found in podman images\n", needle)
	}

	return Pass()
}
