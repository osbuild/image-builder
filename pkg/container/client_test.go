package container_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestClientGetDefaultAuthFile(t *testing.T) {
	testCases := []struct {
		name string

		// set REGISTRY_AUTH_FILE variable to a random tempdir
		authFileEnv bool
		// make the auth file path readable
		authFileReadable bool

		// set XDG_RUNTIME_DIR variable to a random tempdir
		runtimeDirEnv bool
		// make the auth file in the runtime dir readable
		runtimeDirReadable bool
	}{
		{
			name: "empty",
		},
		{
			name: "authfile",

			authFileEnv:      true,
			authFileReadable: true,
		},
		{
			name: "authfile-unreadable",

			authFileEnv:      true,
			authFileReadable: false,
		},
		{
			name: "runtimedir",

			runtimeDirEnv:      true,
			runtimeDirReadable: true,
		},
		{
			name: "runtimedir-unreadable",

			runtimeDirEnv:      true,
			runtimeDirReadable: false,
		},

		{
			name: "both",

			authFileEnv:      true,
			authFileReadable: true,

			runtimeDirEnv:      true,
			runtimeDirReadable: true,
		},
		{
			name: "both-unreadable-authfile",

			authFileEnv:      true,
			authFileReadable: false,

			runtimeDirEnv:      true,
			runtimeDirReadable: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			// Can't test unreadable permissions when running as root
			if !tc.authFileReadable || !tc.runtimeDirReadable {
				currentUser, err := user.Current()
				require.NoError(err)
				if currentUser.Uid == "0" {
					t.Skip("user is root, skipping tests that rely on limited permissions")
				}
			}
			tmp := t.TempDir()

			// create the expected output based on the test variables since
			// paths and files are created dynamically (using tmp)
			expected := "/var/empty/containers-auth.json" // the default when nothing is set

			os.Unsetenv("XDG_RUNTIME_DIR")
			if tc.runtimeDirEnv {
				runtimeDir := filepath.Join(tmp, "run")
				os.Setenv("XDG_RUNTIME_DIR", runtimeDir)

				var mode os.FileMode = 0o000
				if tc.runtimeDirReadable {
					mode = 0o700
					expected = filepath.Join(runtimeDir, "containers", "auth.json")
				}
				require.NoError(os.MkdirAll(runtimeDir, mode))
			}

			os.Unsetenv("REGISTRY_AUTH_FILE")
			if tc.authFileEnv {
				authFileDir := filepath.Join(tmp, "reg")
				require.NoError(os.MkdirAll(authFileDir, 0o700))
				authFilepath := filepath.Join(authFileDir, "auth.json")
				os.Setenv("REGISTRY_AUTH_FILE", authFilepath)

				authFile, err := os.Create(authFilepath)
				require.NoError(err)
				if tc.authFileReadable {
					expected = authFilepath
				} else {
					require.NoError(authFile.Chmod(0o000))
				}
			}

			require.Equal(expected, container.GetDefaultAuthFile())
		})
	}
}

func TestApplyDefaultDomainPath(t *testing.T) {
	testCases := []struct {
		name string

		target        string
		defaultDomain string
		defaultPath   string

		expectedTarget          string
		expectedAppliedDefaults bool
	}{
		{
			name: "no-slash",

			target:        "myimage",
			defaultDomain: "example.org",
			defaultPath:   "library",

			expectedTarget:          "example.org/library/myimage",
			expectedAppliedDefaults: true,
		},
		{
			name: "no-slash-with-tag",

			target:        "myimage:latest",
			defaultDomain: "example.org",
			defaultPath:   "library",

			expectedTarget:          "example.org/library/myimage:latest",
			expectedAppliedDefaults: true,
		},
		{
			name: "slash-without-dot-colon",

			target:        "user/myimage",
			defaultDomain: "example.org",
			defaultPath:   "library",

			expectedTarget:          "example.org/library/user/myimage",
			expectedAppliedDefaults: true,
		},
		{
			name: "domain-with-dot",

			target:        "c.example.com/osbuild/myimage",
			defaultDomain: "example.org",
			defaultPath:   "lib",

			expectedTarget:          "c.example.com/osbuild/myimage",
			expectedAppliedDefaults: false,
		},
		{
			name: "domain-with-colon",

			target:        "localhost:5000/myimage",
			defaultDomain: "example.org",
			defaultPath:   "lib",

			expectedTarget:          "localhost:5000/myimage",
			expectedAppliedDefaults: false,
		},
		{
			name: "domain-localhost",

			target:        "localhost/myimage",
			defaultDomain: "example.org",
			defaultPath:   "lib",

			expectedTarget:          "localhost/myimage",
			expectedAppliedDefaults: false,
		},
		{
			name: "empty-default-domain",

			target:        "myimage",
			defaultDomain: "",
			defaultPath:   "library",

			expectedTarget:          "myimage",
			expectedAppliedDefaults: false,
		},
		{
			name: "empty-default-path",

			target:        "myimage",
			defaultDomain: "example.org",
			defaultPath:   "",

			expectedTarget:          "example.org/myimage",
			expectedAppliedDefaults: true,
		},
		{
			name: "no-defaults",

			target:        "myimage",
			defaultDomain: "",
			defaultPath:   "",

			expectedTarget:          "myimage",
			expectedAppliedDefaults: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, applied := container.ApplyDefaultDomainPath(tc.target, tc.defaultDomain, tc.defaultPath)
			assert.Equal(t, tc.expectedTarget, actual)
			assert.Equal(t, tc.expectedAppliedDefaults, applied)
		})
	}
}

func TestSetArchitectureChoice(t *testing.T) {
	testCases := []struct {
		name string

		inputArch string

		expectedArch    string
		expectedVariant string
	}{
		{
			name: "empty",
		},
		{
			name:      "x86_64",
			inputArch: "x86_64",

			expectedArch: "amd64",
		},
		{
			name:      "amd64",
			inputArch: "amd64",

			expectedArch: "amd64",
		},
		{
			name:      "aarch64",
			inputArch: "aarch64",

			expectedArch:    "arm64",
			expectedVariant: "v8",
		},
		{
			name:      "armhfp",
			inputArch: "armhfp",

			expectedArch:    "arm",
			expectedVariant: "v7",
		},
		{
			name:      "arm64",
			inputArch: "arm64",

			expectedArch: "arm64",
		},
		{
			name:      "s390x",
			inputArch: "s390x",

			expectedArch: "s390x",
		},
		{
			name:      "ppc64le",
			inputArch: "ppc64le",

			expectedArch: "ppc64le",
		},
		{
			name:      "not-an-arch",
			inputArch: "not-an-arch",

			expectedArch: "not-an-arch",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl, err := container.NewClient("nothing")
			require.NoError(t, err)

			cl.SetArchitectureChoice(tc.inputArch)
			sysctx := container.ClientSysctx(cl)
			assert.Equal(t, tc.expectedArch, sysctx.ArchitectureChoice)
			assert.Equal(t, tc.expectedVariant, sysctx.VariantChoice)
		})
	}
}

func TestParseImageName(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		expectErr string
	}{
		{
			name:      "no-colon",
			input:     "nocolon",
			expectErr: "invalid image name 'nocolon'",
		},
		{
			name:      "unknown-transport",
			input:     "invalid:/some/path",
			expectErr: "unknown transport 'invalid'",
		},
		{
			name:  "valid-oci-archive",
			input: "oci-archive:/dev/null",
		},
		{
			name:  "valid-oci",
			input: "oci:/dev/null",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := container.ParseImageName(tc.input)
			if tc.expectErr != "" {
				assert.Nil(t, ref)
				assert.EqualError(t, err, tc.expectErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ref)
			}
		})
	}
}

func TestGetManifest(t *testing.T) {
	require := require.New(t)

	registry := testregistry.New()
	defer registry.Close()

	repo := registry.AddRepo("library/osbuild")
	listDigest := repo.AddImage(
		[]testregistry.Blob{testregistry.NewDataBlobFromBase64(testregistry.RootLayer)},
		[]string{"amd64", "arm64"},
		"some kind of container",
		time.Time{},
	)

	// These digests were retrieved by manually inspecting the manifest list in
	// the registry after adding the images above.
	amd64Digest := "sha256:505fe73a6102a624a46a1732e14e47034b9cca6f1ceaa3ef728aefbb2390f026"
	arm64Digest := "sha256:28ddcb768b2624aac065aaa4feccb1be3e340054954183c2908a607b1f133478"

	ref := registry.GetRef("library/osbuild")
	client, err := container.NewClient(ref)
	client.SkipTLSVerify()

	require.NoError(err)
	require.NotNil(t, client)

	ctx := t.Context()

	t.Run("manifest-list", func(t *testing.T) { // test getting manifest list
		assert := assert.New(t)
		manifest, err := client.GetManifest(ctx, "", false)
		assert.NoError(err)
		assert.NotEmpty(manifest)

		digest, err := manifest.Digest()
		assert.NoError(err)
		assert.Equal(listDigest, digest.String())
	})

	t.Run("instance-manifest-amd64", func(t *testing.T) { // test getting specific instance manifest
		assert := assert.New(t)
		manifest, err := client.GetManifest(ctx, digest.Digest(amd64Digest), false)
		assert.NoError(err)

		digest, err := manifest.Digest()
		assert.NoError(err)
		assert.Equal(amd64Digest, digest.String())
	})

	t.Run("instance-manifest-arm64", func(t *testing.T) { // test getting specific instance manifest
		assert := assert.New(t)
		manifest, err := client.GetManifest(ctx, digest.Digest(arm64Digest), false)
		assert.NoError(err)

		digest, err := manifest.Digest()
		assert.NoError(err)
		assert.Equal(arm64Digest, digest.String())
	})
}

func TestGetManifestLocal(t *testing.T) {
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

	cmd := exec.Command(
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
		// fmt.Printf("Waiting for inspection: %s\n", tmpStorage)
		// time.Sleep(2 * time.Minute)
		cmd := exec.Command("podman", "--root", tmpStorage, "system", "prune", "-f")
		err := cmd.Run()
		assert.NoError(t, err)
	}()

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	fmt.Printf("build: %s\n", cmd.Stdout)

	err = cmd.Run()
	assert.NoError(t, err)

	client, err := container.NewClientWithTestStorage("localhost/multi-arch", tmpStorage)
	assert.NoError(t, err)
	ctx := t.Context()

	manifestList, imageDigests := getManifestListAndImageDigests(t, tmpStorage, "localhost/multi-arch:latest")

	t.Run("manifest-list", func(t *testing.T) { // test getting manifest list
		assert := assert.New(t)
		manifest, err := client.GetManifest(ctx, "", true)
		assert.NoError(err)
		assert.NotEmpty(manifest)
		assert.Equal(manifestList, string(manifest.Data))
	})

	for idx, imageDigest := range imageDigests {
		t.Run(fmt.Sprintf("instance-manifests-%d", idx), func(t *testing.T) { // test getting specific instance manifest
			assert := assert.New(t)
			manifest, err := client.GetManifest(ctx, digest.Digest(imageDigest), true)
			assert.NoError(err)
			digest, err := manifest.Digest()
			assert.NoError(err)
			assert.Equal(digest.String(), imageDigest)
		})
	}
}

func getManifestListAndImageDigests(t *testing.T, storage string, manifestName string) (string, []string) {
	t.Helper()
	cmd := exec.Command("skopeo", "inspect", "--raw", fmt.Sprintf("containers-storage:[%s]%s", storage, manifestName))
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fail()
	}

	// parse manifest list for image IDs
	type manifest struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		// we don't care about the rest of the fields here
	}

	type manifestList struct {
		SchemaVersion int        `json:"schemaVersion"`
		MediaType     string     `json:"mediaType"`
		Manifests     []manifest `json:"manifests"`
	}

	var ml manifestList
	if err := json.Unmarshal(stdout.Bytes(), &ml); err != nil {
		t.Fail()
	}

	// make sure we got a manifest list (image index)
	expectedType := "application/vnd.oci.image.index.v1+json"
	if ml.MediaType != expectedType {
		t.Fatalf("expected manifest list (mediaType: %s) but instead got %s", expectedType, ml.MediaType)
	}

	digests := make([]string, len(ml.Manifests))
	for idx, manifest := range ml.Manifests {
		// verify the mediatype of each
		expectedType := "application/vnd.oci.image.manifest.v1+json"
		if manifest.MediaType != expectedType {
			t.Fatalf("expected image manifest (mediaType: %s) but instead got %s", expectedType, manifest.MediaType)
		}
		digests[idx] = manifest.Digest
	}

	return stdout.String(), digests
}
