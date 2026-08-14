package testregistry

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type credentials struct {
	username string
	password string
}

type Registry struct {
	server *httptest.Server
}

func New() *Registry {
	return newRegistry(nil)
}

func NewWithCredentials(user, pass string) *Registry {
	return newRegistry(
		&credentials{
			username: user,
			password: pass,
		},
	)
}

// basicAuthMiddleware intercepts requests to enforce credentials
func basicAuthMiddleware(next http.Handler, creds credentials) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != creds.username || pass != creds.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Registry"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newRegistry(creds *credentials) *Registry {
	handler := registry.New()
	if creds != nil {
		handler = basicAuthMiddleware(handler, *creds)
	}

	ts := httptest.NewTLSServer(handler)

	r := &Registry{
		server: ts,
	}

	return r
}

func (reg *Registry) Close() {
	reg.server.Close()
}

func (reg *Registry) GetRef(repo string) string {
	return fmt.Sprintf("%s/%s", reg.server.Listener.Addr().String(), repo)
}

func (r *Registry) Host() string {
	return r.server.Listener.Addr().String()
}

// Image is a simple struct that holds the image ID and digest of a single
// image.
type Image struct {
	imageID string
	digest  string
}

func (img Image) ImageID() string {
	return img.imageID
}

func (img Image) Digest() string {
	return img.digest
}

// PopulateWithManifestList adds a manifest list (index) to the registry with
// the given ref. The list contains four containers, one for each architecture
// (amd64, arm64, s390x, ppc64le).
// Returns the manifest list digest and a map of all the digests keyed by the architecture.
func (r *Registry) PopulateWithManifestList(containerRef string) (string, map[string]Image, error) {
	imageRef := r.Host() + "/" + containerRef

	// Generate four random images, one for each architecture
	amd64, err := random.Image(1024, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}
	amd64ImageID, err := amd64.ConfigName()
	if err != nil {
		return "", nil, err
	}

	arm64, err := random.Image(1024, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}
	arm64ImageID, err := arm64.ConfigName()
	if err != nil {
		return "", nil, err
	}

	s390x, err := random.Image(2014, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}
	s390xImageID, err := s390x.ConfigName()
	if err != nil {
		return "", nil, err
	}

	ppc64le, err := random.Image(2014, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}
	ppc64leImageID, err := ppc64le.ConfigName()
	if err != nil {
		return "", nil, err
	}

	// Add images to an index and define the architecture
	index := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{
			Add: amd64,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
		},
		mutate.IndexAddendum{
			Add: arm64,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					Architecture: "arm64",
					OS:           "linux",
				},
			},
		},
		mutate.IndexAddendum{
			Add: s390x,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					Architecture: "s390x",
					OS:           "linux",
				},
			},
		},
		mutate.IndexAddendum{
			Add: ppc64le,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					Architecture: "ppc64le",
					OS:           "linux",
				},
			},
		},
	)

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	tr := &http.Transport{
		// disable TLS verification
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint:gosec // this is a test registry
	}

	if err := remote.WriteIndex(ref, index, remote.WithTransport(tr)); err != nil {
		return "", nil, fmt.Errorf("failed to push image: %w", err)
	}

	indexDigest, err := index.Digest()
	if err != nil {
		return "", nil, err
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return "", nil, err
	}

	images := map[string]Image{
		"amd64": {
			imageID: amd64ImageID.String(),
		},
		"arm64": {
			imageID: arm64ImageID.String(),
		},
		"s390x": {
			imageID: s390xImageID.String(),
		},
		"ppc64le": {
			imageID: ppc64leImageID.String(),
		},
	}
	for _, imageManifest := range indexManifest.Manifests {
		image := images[imageManifest.Platform.Architecture]
		image.digest = imageManifest.Digest.String()
		images[imageManifest.Platform.Architecture] = image
	}

	return indexDigest.String(), images, nil
}
