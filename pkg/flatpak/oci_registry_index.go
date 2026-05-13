package flatpak

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/container"
)

// Functionality query for flatpaks in an OCI registry that supports the registry index
// specification [1], for example quay.io or any other repository based on pulp.
//
// We try to be as similar to Flatpak in our queries as we can be.
//
// [1]: https://github.com/flatpak/flatpak-oci-specs/blob/main/registry-index.md

const (
	// Flatpak only uses the static endpoint
	ENDPOINT_STATIC = "/index/static"
)

type ResponseRoot struct {
	Registry string               `json:"Registry"`
	Results  []ResponseRepository `json:"Results"`
}

type ResponseRepository struct {
	Name   string              `json:"Name"`
	Images []*ResponseImage    `json:"Images"`
	Lists  []ResponseImageList `json:"Lists"`
}

type ResponseImage struct {
	Tags         []string          `json:"Tags"`
	Digest       string            `json:"Digest"`
	MediaType    string            `json:"MediaType"`
	OS           string            `json:"OS"`
	Architecture string            `json:"Architecture"`
	Annotations  map[string]string `json:"Annotations"`
	Labels       map[string]string `json:"Labels"`
}

type ResponseImageList struct{}

// QueryOCIRegistryIndex looks up a flatpak ref via the registry index, then uses
// [container.Resolver] to resolve the digest-pinned image to a [container.Spec]
// (manifest digest, config digest / image ID, source name, optional list digest).
//
// imageArch is the flatpak architecture string from the ref (e.g. x86_64); it is passed
// to the container resolver for manifest-list selection.
func QueryOCIRegistryIndex(uri, ref, os, tag, imageArch string) (*container.Spec, error) {
	res, err := fetchRegistryIndex(uri, os, tag)
	if err != nil {
		return nil, err
	}

	repoName, manifestDigest, err := findFlatpakInIndex(res, ref)
	if err != nil {
		return nil, err
	}

	imageRef := ociImageRefFromIndexComponents(res.Registry, repoName, manifestDigest)

	r := container.NewBlockingResolver(imageArch)
	spec, err := r.Resolve(container.SourceSpec{
		Source:    imageRef,
		Name:      repoName,
		TLSVerify: nil,
		Local:     false,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve flatpak container %q: %w", imageRef, err)
	}
	return &spec, nil
}

// findFlatpakInIndex returns the repository name and manifest digest for the first image
// whose org.flatpak.ref label equals wantRef.
func findFlatpakInIndex(res *ResponseRoot, wantRef string) (repoName, manifestDigest string, err error) {
	for _, result := range res.Results {
		for _, img := range result.Images {
			if img == nil || img.Labels == nil {
				continue
			}
			if img.Labels["org.flatpak.ref"] != wantRef {
				continue
			}
			if res.Registry == "" {
				return "", "", fmt.Errorf("registry index: found %q but Registry field was missing", wantRef)
			}
			return result.Name, img.Digest, nil
		}
	}
	return "", "", fmt.Errorf("did not find image %q", wantRef)
}

// ociImageRefFromIndexComponents builds a docker-style reference host/repo@digest for
// [container.NewClient] / the blocking resolver.
func ociImageRefFromIndexComponents(registryURL, repoName, manifestDigest string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(registryURL, "https://"), "http://")
	host = strings.TrimSuffix(host, "/")
	repoPath := strings.TrimPrefix(repoName, "/")
	return fmt.Sprintf("%s/%s@%s", host, repoPath, manifestDigest)
}

func httpClient() (*http.Client, error) {
	return &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
		Timeout:   300 * time.Second,
	}, nil
}

func fetchRegistryIndex(uri, os, tag string) (*ResponseRoot, error) {
	client, err := httpClient()
	if err != nil {
		return nil, err
	}

	uri, err = url.JoinPath(uri, ENDPOINT_STATIC)
	if err != nil {
		return nil, fmt.Errorf("could not format URI: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	// The default set of query parameters that are always passed by
	// flatpak. See [1].
	//
	// [1]: https://github.com/flatpak/flatpak-oci-specs/blob/main/registry-index.md#appendix---usage-by-flatpak
	q := req.URL.Query()
	q.Set("label:org.flatpak.ref:exists", "1")
	q.Set("os", os)
	q.Set("tag", tag)
	req.URL.RawQuery = q.Encode()

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry index %q returned status: %s", uri, res.Status)
	}

	var root ResponseRoot
	if err := json.NewDecoder(res.Body).Decode(&root); err != nil {
		return nil, err
	}

	return &root, nil
}
