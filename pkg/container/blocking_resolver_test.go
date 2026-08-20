package container_test

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/testregistry"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"
)

func TestBlockingResolver(t *testing.T) {
	require := require.New(t)

	registry := testregistry.NewDistributionRegistry()
	defer registry.Close()

	allImages := make(map[string]map[string]testregistry.Image) // ref -> arch -> digest

	// add 10 manifest lists and then resolve them, verifying the digests we
	// get from the resolver
	for i := range 10 {
		ref := fmt.Sprintf("library/osbuild:%d", i)
		_, images, err := registry.PopulateWithManifestList(ref)
		require.NoError(err)
		allImages[ref] = images
	}

	for ref, images := range allImages {
		for arch, image := range images {
			resolver := container.NewBlockingResolver(arch)
			resolver.Add(container.SourceSpec{
				Source:    registry.GetRef(ref),
				Name:      "",
				Digest:    common.ToPtr(""),
				TLSVerify: common.ToPtr(false),
				Local:     false,
			})

			have, err := resolver.Finish()
			require.NoError(err)
			require.NotNil(have)
			require.Equal(image.Digest(), have[0].Digest)
		}
	}
}

func TestBlockingResolverResolveAll(t *testing.T) {
	// Similar test as above but resolving all containers at the same time
	require := require.New(t)

	registry := testregistry.NewDistributionRegistry()
	defer registry.Close()

	allImages := make(map[string]map[string]testregistry.Image) // ref -> arch -> digest

	for i := range 10 {
		ref := fmt.Sprintf("library/osbuild:%d", i)
		_, images, err := registry.PopulateWithManifestList(ref)
		require.NoError(err)
		refWithHost := registry.GetRef(ref)
		allImages[refWithHost] = images
	}

	// make one resolver for each arch and resolve all container refs for that
	// architecture
	for _, arch := range []string{"amd64", "arm64", "s390x", "ppc64le"} {
		resolver := container.NewBlockingResolver(arch)

		for refWithHost := range allImages {
			resolver.Add(container.SourceSpec{
				Source:    refWithHost,
				Name:      refWithHost,
				Digest:    common.ToPtr(""),
				TLSVerify: common.ToPtr(false),
				Local:     false,
			})
		}

		results, err := resolver.Finish()
		require.NoError(err)
		require.NotNil(results)

		for _, result := range results {
			// the LocalName should be the Name we added to the source spec, which is refWithHost
			expImage := allImages[result.LocalName][arch]
			require.Equal(expImage.Digest(), result.Digest)
			require.Equal(expImage.ImageID(), result.ImageID)
		}
	}
}

func TestBlockingResolverFail(t *testing.T) {
	resolver := container.NewBlockingResolver("amd64")

	resolver.Add(container.SourceSpec{
		Source:    "invalid-reference@${IMAGE_DIGEST}",
		Name:      "",
		Digest:    common.ToPtr(""),
		TLSVerify: common.ToPtr(false),
		Local:     false,
	})
	specs, err := resolver.Finish()
	assert.Error(t, err)
	assert.Len(t, specs, 0)

	registry := testregistry.NewDistributionRegistry()
	defer registry.Close()

	resolver.Add(container.SourceSpec{
		Source:    registry.GetRef("repo"),
		Name:      "",
		Digest:    common.ToPtr(""),
		TLSVerify: common.ToPtr(false),
		Local:     false,
	})
	specs, err = resolver.Finish()
	assert.Error(t, err)
	assert.Len(t, specs, 0)
}

func TestBlockingResolverLocalManifest(t *testing.T) {
	currentUser, err := user.Current()
	assert.NoError(t, err)

	if !*forceLocal {
		// local resolver tests aren't forced, so we can skip
		// them if the user is not root or the podman executable
		// is not installed
		if currentUser.Uid != "0" {
			t.Skip("User is not root, skipping test")
		}

		_, err = exec.LookPath("podman")
		if err != nil {
			t.Skip("Podman not available, skipping test")
		}
	}

	containerFile, err := os.CreateTemp(t.TempDir(), "Containerfile")
	assert.NoError(t, err)

	tmpStorage := t.TempDir()

	_, err = containerFile.Write([]byte("FROM scratch"))
	assert.NoError(t, err)

	cmd := exec.Command( //nolint:gosec
		"podman",
		"--root", tmpStorage, // don't dirty the default store
		"build",
		"--platform", "linux/amd64,linux/arm64",
		"--manifest", "multi-arch",
		"-f", containerFile.Name(),
		".",
	)
	// cleanup the containers
	defer func() {
		cmd := exec.Command("podman", "--root", tmpStorage, "system", "prune", "-f")
		err := cmd.Run()
		assert.NoError(t, err)
	}()

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	assert.NoError(t, err)

	// try resolve an x86_64 container using a local manifest list
	resolver := container.NewBlockingResolverWithTestClient("amd64", func(target string) (*container.Client, error) {
		return container.NewClientWithTestStorage(target, tmpStorage)
	})

	resolver.Add(container.SourceSpec{
		Source:    "localhost/multi-arch",
		Name:      "",
		Digest:    common.ToPtr(""),
		TLSVerify: common.ToPtr(false),
		Local:     true,
	})
	specs, err := resolver.Finish()
	assert.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Equal(t, specs[0].LocalName, "localhost/multi-arch:latest")
	assert.Equal(t, specs[0].Arch.String(), arch.ARCH_X86_64.String())

	// try resolve an  aarch64 container using a local manifest list
	resolver = container.NewBlockingResolverWithTestClient("arm64", func(target string) (*container.Client, error) {
		return container.NewClientWithTestStorage(target, tmpStorage)
	})

	resolver.Add(container.SourceSpec{
		Source:    "localhost/multi-arch",
		Name:      "",
		Digest:    common.ToPtr(""),
		TLSVerify: common.ToPtr(false),
		Local:     true,
	})
	specs, err = resolver.Finish()
	assert.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Equal(t, specs[0].LocalName, "localhost/multi-arch:latest")
	assert.Equal(t, specs[0].Arch.String(), arch.ARCH_AARCH64.String())
}
