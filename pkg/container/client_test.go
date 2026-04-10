package container_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/internal/testregistry"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
)

//

func TestClientResolve(t *testing.T) {

	registry := testregistry.New()
	defer registry.Close()

	repo := registry.AddRepo("library/osbuild")
	listDigest := repo.AddImage(
		[]testregistry.Blob{testregistry.NewDataBlobFromBase64(testregistry.RootLayer)},
		[]string{"amd64", "ppc64le"},
		"cool container",
		time.Time{})

	ref := registry.GetRef("library/osbuild")
	client, err := container.NewClient(ref)

	assert.NoError(t, err)
	assert.NotNil(t, client)

	client.SkipTLSVerify()

	ctx := t.Context()

	client.SetArchitectureChoice("amd64")
	spec, err := client.Resolve(ctx, "", false)

	assert.NoError(t, err)
	assert.Equal(t, container.Spec{
		Source:     ref,
		Digest:     "sha256:f29b6cd42a94a574583439addcd6694e6224f0e4b32044c9e3aee4c4856c2a50",
		ImageID:    "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f",
		TLSVerify:  client.GetTLSVerify(),
		LocalName:  client.Target.String(),
		ListDigest: listDigest,
		Arch:       arch.ARCH_X86_64,
	}, spec)

	client.SetArchitectureChoice("ppc64le")
	spec, err = client.Resolve(ctx, "", false)

	assert.NoError(t, err)
	assert.Equal(t, container.Spec{
		Source:     ref,
		Digest:     "sha256:d49eebefb6c7ce5505594bef652bd4adc36f413861bd44209d9b9486310b1264",
		ImageID:    "sha256:d2ab8fea7f08a22f03b30c13c6ea443121f25e87202a7496e93736efa6fe345a",
		TLSVerify:  client.GetTLSVerify(),
		LocalName:  client.Target.String(),
		ListDigest: listDigest,
		Arch:       arch.ARCH_PPC64LE,
	}, spec)

	// don't have that architecture
	client.SetArchitectureChoice("s390x")
	_, err = client.Resolve(ctx, "", false)

	assert.Error(t, err)
}

func TestClientAuthFilePath(t *testing.T) {

	client, err := container.NewClient("quay.io/osbuild/osbuild")
	assert.NoError(t, err)

	authFilePath := client.GetAuthFilePath()
	assert.NotEmpty(t, authFilePath)
	assert.Equal(t, authFilePath, container.GetDefaultAuthFile())

	// make sure the file is accessible
	_, err = os.ReadFile(authFilePath)
	assert.True(t, err == nil || os.IsNotExist(err))

	t.Run("XDG_RUNTIME_DIR", func(t *testing.T) {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

		if runtimeDir == "" {
			t.Skip("XDG_RUNTIME_DIR not set, skipping test")
			return
		}

		t.Cleanup(func() {
			os.Setenv("XDG_RUNTIME_DIR", runtimeDir)
		})

		os.Unsetenv("XDG_RUNTIME_DIR")

		authFilePath := container.GetDefaultAuthFile()
		assert.NotEmpty(t, authFilePath)
		_, err = os.ReadFile(authFilePath)
		assert.True(t, err == nil || os.IsNotExist(err))
	})

}
