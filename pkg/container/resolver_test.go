package container_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/internal/testregistry"
	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/container"
)

var forceLocal = flag.Bool(
	"force-local-resolver",
	false,
	"force local resolver, making them fail instead of skip if podman isn't installed or the user is not root",
)

func TestResolver(t *testing.T) {

	registry := testregistry.New()
	defer registry.Close()
	repo := registry.AddRepo("library/osbuild")
	ref := registry.GetRef("library/osbuild")

	refs := make([]string, 10)
	for i := 0; i < len(refs); i++ {
		checksum := repo.AddImage(
			[]testregistry.Blob{testregistry.NewDataBlobFromBase64(testregistry.RootLayer)},
			[]string{"amd64", "ppc64le"},
			fmt.Sprintf("image %d", i),
			time.Time{})

		tag := fmt.Sprintf("%d", i)
		repo.AddTag(checksum, tag)
		refs[i] = fmt.Sprintf("%s:%s", ref, tag)
	}

	resolver := container.NewResolver("amd64")

	for _, r := range refs {
		resolver.Add(container.SourceSpec{
			Source:    r,
			Name:      "",
			Digest:    common.ToPtr(""),
			TLSVerify: common.ToPtr(false),
			Local:     false,
		})
	}

	have, err := resolver.Finish()
	assert.NoError(t, err)
	assert.NotNil(t, have)

	assert.Len(t, have, len(refs))

	want := make([]container.Spec, len(refs))
	for i, r := range refs {
		spec, err := registry.Resolve(r, arch.ARCH_X86_64)
		assert.NoError(t, err)
		want[i] = spec
	}

	assert.ElementsMatch(t, have, want)
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

	registry := testregistry.New()
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
