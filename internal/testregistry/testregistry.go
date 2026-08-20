package testregistry

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
)

/*
A base64 encoded gzipped tarball with the following contents:

	-rw-r--r-- root/root        12 2021-09-17 12:32 hello.txt         (Contents: "Hello World")
	drwxr-xr-x root/root         0 1970-01-01 01:00 subdir/
	-rw-r--r-- root/root         8 2021-09-17 12:32 subdir/file.txt   (Contents: "osbuild")
	-rw-r--r-- root/root         7 2021-09-17 12:32 world.txt         (Contents: "hello!")

Can be used with [NewDataBlobFromBase64] to create a data blob for [Repo.AddImage].
*/
const RootLayer = `H4sIAAAJbogA/+SWUYqDMBCG53lP4V5g9x8dzRX2Bvtc0VIhEIhKe/wSKxgU6ktjC/O9hMzAQDL8
/8yltdb9DLeB0gEGKhHCg/UJsBAL54zKFBAC54ZzyrCUSMfYDydPgHfu6R/s5VePilOfzF/of/bv
vG2+lqhyFNGPddP53yjyegCBKcuNROZ77AmBoP+CmbIyqpEM5fqf+3/ubJtsCuz7P1b+L1Du/4f5
v+vrsVPu/Vq9P3ANk//d+x/MZv8TKNf/Qfqf9v9v5fLXK3/lKEc5ypm4AwAA//8DAE6E6nIAEgAA
`

// The following code implements a toy container registry to test with

// Blob interface
type Blob interface {
	GetSize() int64
	GetMediaType() string
	GetDigest() digest.Digest

	Reader() io.Reader
}

// dataBlob //
type dataBlob struct {
	Data      []byte
	MediaType string
}

func NewDataBlobFromBase64(text string) dataBlob {
	data, err := base64.StdEncoding.DecodeString(text)

	if err != nil {
		panic("decoding of text failed")
	}

	return dataBlob{
		Data: data,
	}
}

// Blob interface implementation
func (b dataBlob) GetSize() int64 {
	return int64(len(b.Data))
}

func (b dataBlob) GetMediaType() string {
	if b.MediaType != "" {
		return b.MediaType
	}

	return manifest.DockerV2Schema2LayerMediaType
}

func (b dataBlob) GetDigest() digest.Digest {
	return digest.FromBytes(b.Data)
}

func (b dataBlob) Reader() io.Reader {
	return bytes.NewReader(b.Data)
}

func MakeDescriptorForBlob(b Blob) manifest.Schema2Descriptor {
	return manifest.Schema2Descriptor{
		MediaType: b.GetMediaType(),
		Size:      b.GetSize(),
		Digest:    b.GetDigest(),
	}
}

// SyncMap is a very simple generic map with concurrency-safe accessors.
type SyncMap[T any] struct {
	mappy map[string]T
	mutex sync.Mutex
}

func (sm *SyncMap[T]) Add(name string, elem T) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if sm.mappy == nil {
		// magically handle initialising the map on first Add().
		sm.mappy = make(map[string]T)
	}

	sm.mappy[name] = elem
}

func (sm *SyncMap[T]) Get(name string) (T, bool) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	elem, ok := sm.mappy[name]
	return elem, ok
}

// Repo //
type Repo struct {
	blobs     SyncMap[Blob]
	manifests SyncMap[*manifest.Schema2]
	images    SyncMap[*manifest.Schema2List]
	tags      SyncMap[string]
}

func NewRepo() *Repo {
	return &Repo{
		blobs:     SyncMap[Blob]{},
		manifests: SyncMap[*manifest.Schema2]{},
		images:    SyncMap[*manifest.Schema2List]{},
		tags:      SyncMap[string]{},
	}
}

func (r *Repo) AddBlob(b Blob) manifest.Schema2Descriptor {
	desc := MakeDescriptorForBlob(b)
	r.blobs.Add(desc.Digest.String(), b)
	return desc
}

func (r *Repo) AddObject(v interface{}, mediaType string) manifest.Schema2Descriptor {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic("could not marshal image object")
	}

	blob := dataBlob{
		Data:      data,
		MediaType: mediaType,
	}

	return r.AddBlob(blob)
}

func (r *Repo) AddManifest(mf *manifest.Schema2) manifest.Schema2Descriptor {
	desc := r.AddObject(mf, mf.MediaType)

	r.manifests.Add(desc.Digest.String(), mf)

	return desc
}

func (r *Repo) AddImage(layers []Blob, arches []string, comment string, ctime time.Time) string {

	blobs := make([]manifest.Schema2Descriptor, len(layers))

	for i, layer := range layers {
		blobs[i] = r.AddBlob(layer)
	}

	manifests := make([]manifest.Schema2ManifestDescriptor, len(arches))

	for i, arch := range arches {
		img := manifest.Schema2V1Image{
			Architecture: arch,
			OS:           "linux",
			Author:       "osbuild",
			Comment:      comment,
			Created:      ctime,
		}

		// Add the config object
		config := r.AddObject(img, manifest.DockerV2Schema2ConfigMediaType)

		// make and add the manifest object
		schema := manifest.Schema2FromComponents(config, blobs)
		mf := r.AddManifest(schema)

		desc := manifest.Schema2ManifestDescriptor{
			Schema2Descriptor: mf,
			Platform: manifest.Schema2PlatformSpec{
				Architecture: arch,
				OS:           "linux",
			},
		}

		manifests[i] = desc
	}

	list := manifest.Schema2ListFromComponents(manifests)
	desc := r.AddObject(list, list.MediaType)
	checksum := desc.Digest.String()

	r.images.Add(checksum, list)
	r.tags.Add("latest", checksum)

	return checksum
}

func (r *Repo) AddTag(checksum, tag string) {

	if _, ok := r.images.Get(checksum); !ok {
		panic("cannot tag: image not found: " + checksum)
	}

	r.tags.Add(tag, checksum)
}

func WriteBlob(blob Blob, w http.ResponseWriter) {
	w.Header().Add("Content-Type", blob.GetMediaType())
	w.Header().Add("Content-Length", fmt.Sprintf("%d", blob.GetSize()))
	w.Header().Add("Docker-Content-Digest", blob.GetDigest().String())
	w.WriteHeader(http.StatusOK)

	reader := blob.Reader()

	_, err := io.Copy(w, reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing blob: %v", err)
	}
}

func BlobIsManifest(blob Blob) bool {
	mt := blob.GetMediaType()
	return mt == manifest.DockerV2Schema2MediaType || mt == manifest.DockerV2ListMediaType
}

func (r *Repo) ServeManifest(ref string, w http.ResponseWriter, req *http.Request) {
	if checksum, ok := r.tags.Get(ref); ok {
		ref = checksum
	}

	blob, ok := r.blobs.Get(ref)
	if !ok || !BlobIsManifest(blob) {
		fmt.Fprintf(os.Stderr, "manifest %s not found", ref)
		http.NotFound(w, req)
		return
	}

	WriteBlob(blob, w)
}

func (r *Repo) ServeBlob(ref string, w http.ResponseWriter, req *http.Request) {

	blob, ok := r.blobs.Get(ref)

	if !ok {
		fmt.Fprintf(os.Stderr, "blob %s not found", ref)
		http.NotFound(w, req)
		return
	}

	WriteBlob(blob, w)
}

// Registry //

type Registry struct {
	server *httptest.Server
	repos  SyncMap[*Repo]
}

func (reg *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	parts := strings.SplitN(req.URL.Path, "?", 1)
	paths := strings.Split(strings.Trim(parts[0], "/"), "/")

	// Possbile routes
	// [1] version-check:  /v2/
	// [2] blobs:          /v2/<repo_name>/blobs/<digest>
	// [3] manifest:       /v2/<repo_name>/manifests/<ref>
	//
	// we need at least 4 path components and path has to start with "/v2"

	if len(paths) < 1 || paths[0] != "v2" {
		http.NotFound(w, req)
		return
	}

	// [1] version check
	if len(paths) == 1 {
		w.WriteHeader(200)
		return
	} else if len(paths) < 4 {
		http.NotFound(w, req)
		return
	}

	// we asserted that we have at least 4 path components
	ref := paths[len(paths)-1]
	cmd := paths[len(paths)-2]

	repoName := strings.Join(paths[1:len(paths)-2], "/")

	repo, ok := reg.repos.Get(repoName)
	if !ok {
		fmt.Fprintf(os.Stderr, "repo %s not found", repoName)
		http.NotFound(w, req)
		return
	}

	switch cmd {
	case "manifests":
		repo.ServeManifest(ref, w, req)
	case "blobs":
		repo.ServeBlob(ref, w, req)
	default:
		http.NotFound(w, req)
	}
}

func New() *Registry {

	reg := &Registry{
		repos: SyncMap[*Repo]{
			mappy: make(map[string]*Repo),
		},
	}
	reg.server = httptest.NewTLSServer(reg)

	return reg
}

func (reg *Registry) AddRepo(name string) *Repo {
	repo := NewRepo()
	reg.repos.Add(name, repo)
	return repo
}

func (reg *Registry) GetRef(repo string) string {
	return fmt.Sprintf("%s/%s", reg.server.Listener.Addr().String(), repo)
}

func (reg *Registry) Resolve(target string, imgArch arch.Arch) (container.Spec, error) {

	ref, err := reference.ParseNormalizedNamed(target)
	if err != nil {
		return container.Spec{}, fmt.Errorf("failed to parse '%s': %w", target, err)
	}

	domain := reference.Domain(ref)

	tag := "latest"
	var checksum string

	if tagged, ok := ref.(reference.NamedTagged); ok {
		tag = tagged.Tag()
	}

	if digested, ok := ref.(reference.Digested); ok {
		checksum = string(digested.Digest())
	}

	if domain != reg.server.Listener.Addr().String() {
		return container.Spec{}, fmt.Errorf("unknown domain")
	}

	ref = reference.TrimNamed(ref)
	path := reference.Path(ref)

	repo, ok := reg.repos.Get(path)
	if !ok {
		return container.Spec{}, fmt.Errorf("unknown repo")
	}

	if checksum == "" {
		checksum, ok = repo.tags.Get(tag)
		if !ok {
			return container.Spec{}, fmt.Errorf("unknown tag")
		}
	}

	lst, ok := repo.images.Get(checksum)
	listDigest := checksum

	if ok {
		checksum = ""

		for _, m := range lst.Manifests {
			if common.Must(arch.FromString(m.Platform.Architecture)) == imgArch {
				checksum = m.Digest.String()
				break
			}
		}

		if checksum == "" {
			return container.Spec{}, fmt.Errorf("unsupported architecture")
		}
	}

	mf, ok := repo.manifests.Get(checksum)
	if !ok {
		return container.Spec{}, fmt.Errorf("unknown digest")
	}

	return container.Spec{
		Source:     ref.String(),
		Digest:     checksum,
		ImageID:    mf.ConfigDescriptor.Digest.String(),
		LocalName:  target,
		TLSVerify:  common.ToPtr(false),
		ListDigest: listDigest,
		Arch:       imgArch,
	}, nil
}

func (reg *Registry) Close() {
	reg.server.Close()
}

func NewDistributionRegistry() *Registry {
	cfg := &configuration.Configuration{
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{}, // use inmemory storage driver
		},
	}

	ctx := context.Background()
	regHandler := handlers.NewApp(ctx, cfg)

	ts := httptest.NewTLSServer(regHandler)

	r := &Registry{
		server: ts,
	}

	return r
}

func (r *Registry) Host() string {
	return r.server.Listener.Addr().String()
}

// PopulateWithManifestList adds a manifest list (index) to the registry with
// the given ref. The list contains four containers, one for each architecture
// (amd64, arm64, s390x, ppc64le).
// Returns the manifest list digest and a map of all the digests keyed by the architecture.
func (r *Registry) PopulateWithManifestList(containerRef string) (string, map[string]string, error) {
	imageRef := r.Host() + "/" + containerRef

	// Generate four random images, one for each architecture
	amd64, err := random.Image(1024, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}

	arm64, err := random.Image(1024, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}

	s390x, err := random.Image(2014, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
	}

	ppc64le, err := random.Image(2014, 1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate random container image: %w", err)
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

	digests := make(map[string]string, 4)
	indexDigest, err := index.Digest()
	if err != nil {
		return "", nil, err
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return "", nil, err
	}
	for _, imageManifest := range indexManifest.Manifests {
		digests[imageManifest.Platform.Architecture] = imageManifest.Digest.String()
	}

	return indexDigest.String(), digests, nil
}
