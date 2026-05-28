package container

import (
	"path/filepath"

	"github.com/osbuild/images/pkg/customizations/fsnode"
)

// NetworkBackend is the type of network backend used by Podman.
type NetworkBackend string

const (
	NetworkBackendCNI      NetworkBackend = "cni"
	NetworkBackendNetavark NetworkBackend = "netavark"

	// DefaultStoragePath is the default container storage path used by Podman.
	DefaultStoragePath = "/var/lib/containers/storage"
)

// GenDefaultNetworkBackendFile creates an fsnode.File that writes the given
// network backend name to <storagePath>/defaultNetworkBackend.
//
// Certain versions of Podman fall back to 'cni' when they find existing
// container images in the system storage, assuming a migration from an older
// version. Writing this file prevents that behavior and forces Podman to use
// the specified backend.
//
// The storagePath parameter must match the container storage location for the
// image type. OSTree-based images relocate container storage to /usr/share
// because /var is not part of the ostree commit.
func GenDefaultNetworkBackendFile(storagePath string, backend NetworkBackend) (*fsnode.File, error) {
	if storagePath == "" {
		storagePath = DefaultStoragePath
	}
	file, err := fsnode.NewFile(filepath.Join(storagePath, "defaultNetworkBackend"), nil, nil, nil, []byte(backend))
	if err != nil {
		return nil, err
	}
	return file, nil
}
