package container_test

import (
	"flag"
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

var forceLocal = flag.Bool(
	"force-local-resolver",
	false,
	"force local resolver, making them fail instead of skip if podman isn't installed or the user is not root",
)

func TestResolver(t *testing.T) {
	require := require.New(t)

	registry := testregistry.NewGoContainerRegistry()
	defer registry.Close()

	digests := make(map[string]map[string]testregistry.Image) // ref -> arch -> digest

	// add 10 manifest lists and then resolve them, verifying the digests we
	// get from the resolver
	for i := range 10 {
		ref := fmt.Sprintf("library/osbuild:%d", i)
		_, images, err := registry.PopulateWithManifestList(ref)
		require.NoError(err)
		digests[ref] = images
	}

	for ref, images := range digests {
		for arch, image := range images {
			resolver := container.NewResolver(arch)
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

func TestResolverFail(t *testing.T) {
	resolver := container.NewResolver("amd64")

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

	registry := testregistry.NewGoContainerRegistry()
	defer registry.Close()

	badRef := fmt.Sprintf("%s/org/notarepo", registry.Host())

	resolver.Add(container.SourceSpec{
		Source:    badRef,
		Name:      "",
		Digest:    common.ToPtr(""),
		TLSVerify: common.ToPtr(false),
		Local:     false,
	})
	specs, err = resolver.Finish()
	assert.Error(t, err)
	assert.Len(t, specs, 0)
}

func TestResolverErrorIncludesSource(t *testing.T) {
	resolver := container.NewResolver("amd64")
	missing := "localhost/image-builder-missing-payload:debug"
	_, err := resolver.Resolve(container.SourceSpec{
		Source: missing,
		Name:   missing,
		Local:  true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.NotRegexp(t, `failed to resolve container: '':`, err.Error())
}

func TestResolverLocalManifest(t *testing.T) {
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
	resolver := container.NewResolverWithTestClient("amd64", func(target string) (*container.Client, error) {
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
	resolver = container.NewResolverWithTestClient("arm64", func(target string) (*container.Client, error) {
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
