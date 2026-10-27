package image_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/testdisk"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/image"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestBootcISONoCustomizations(t *testing.T) {
	containerSource := container.SourceSpec{
		Source: "some-src",
		Name:   "name",
	}
	containers := []container.SourceSpec{containerSource}
	img := image.NewBootcISO(testPlatform, "bootc-test.iso", containerSource)
	require.NotNil(t, img)
	img.ISOCustomizations = manifest.ISOCustomizations{
		RootfsType: manifest.ErofsRootfs,
		BootType:   manifest.Grub2ISOBoot,
		Label:      "bootc-iso-test",
	}
	img.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
	img.KernelVersion = "5.14.0-611.4.1.el9_7.x86_64"
	img.InstallerCustomizations = manifest.InstallerCustomizations{
		Product:   "Fedora",
		OSVersion: "45",
		Release:   "45",
	}

	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	mf := manifest.New()
	_, err := img.InstantiateManifestFromContainer(&mf, containers, &runner.Fedora{}, rng)
	require.NoError(t, err)

	fakeSourceSpecs := map[string][]container.Spec{
		"build": []container.Spec{{Source: "some-src", Digest: makeFakeDigest(t), ImageID: makeFakeDigest(t)}},
		"image": []container.Spec{{Source: "other-src", Digest: makeFakeDigest(t), ImageID: makeFakeDigest(t)}},
	}
	osbm, err := mf.Serialize(nil, fakeSourceSpecs, nil, nil, nil)
	require.NoError(t, err)

	// Find the image pipeline and check for org.osbuild.bootc.install-to-filesystem
	pipeline := findPipelineFromOsbuildManifest(t, osbm, "image")
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.bootc.install-to-filesystem")
	assert.NotNil(t, stage, "org.osbuild.bootc.install-to-filesystem stage not found")

	// Find the bootc-rootfs pipeline and check compression type
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "bootc-rootfs")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.erofs")
	assert.NotNil(t, stage, "org.osbuild.erofs stage not found")

	// Find the efiboot-tree pipeline and check grub2 iso options
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "efiboot-tree")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.grub2.iso")
	require.NotNil(t, stage, "org.osbuild.grub2.iso stage not found")

	// check the efi grub2 kernel arguments for expected default values
	options := stage["options"].(map[string]any)
	require.NotNil(t, options)
	kernel := options["kernel"].(map[string]any)
	require.NotNil(t, kernel)
	efiopts := kernel["opts"].([]any)
	require.NotNil(t, efiopts)
	assert.Contains(t, efiopts, "ostree=@OSTREE@")
	assert.Contains(t, efiopts, "rd.live.image")

	// Find the bootiso-tree pipeline and check grub2 iso legacy options
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "bootiso-tree")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.grub2.iso.legacy")
	require.NotNil(t, stage, "org.osbuild.grub2.iso.legacy stage not found")

	// check the iso grub2 kernel arguments for expected default values
	options = stage["options"].(map[string]any)
	require.NotNil(t, options)
	kernel = options["kernel"].(map[string]any)
	require.NotNil(t, kernel)
	isoopts := kernel["opts"].([]any)
	require.NotNil(t, isoopts)
	assert.Contains(t, isoopts, "ostree=@OSTREE@")
	assert.Contains(t, isoopts, "rd.live.image")

	// UEFI grub2 and iso grub2 args should be identical
	assert.Equal(t, efiopts, isoopts)

	// Find the bootiso pipeline and check for xorrisofs
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "bootiso")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.xorrisofs")
	assert.NotNil(t, stage, "org.osbuild.xorrisofs stage not found")

	// check filename for bootc-test.iso
	stageOptions := stage["options"].(map[string]any)
	require.NotNil(t, stageOptions)
	assert.Equal(t, "bootc-test.iso", stageOptions["filename"].(string))
}

func TestBootcISOCustomizations(t *testing.T) {
	containerSource := container.SourceSpec{
		Source: "some-src",
		Name:   "name",
	}
	containers := []container.SourceSpec{containerSource}
	img := image.NewBootcISO(testPlatform, "bootc-test.iso", containerSource)
	require.NotNil(t, img)
	img.ISOCustomizations = manifest.ISOCustomizations{
		RootfsType: manifest.ErofsRootfs,
		BootType:   manifest.Grub2ISOBoot,
		Label:      "bootc-iso-test",
	}
	img.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
	img.KernelVersion = "5.14.0-611.4.1.el9_7.x86_64"
	img.InstallerCustomizations = manifest.InstallerCustomizations{
		Product:   "Fedora",
		OSVersion: "45",
		Release:   "45",
	}

	// Add some additional kernel arguments
	img.OSCustomizations.KernelOptionsAppend = []string{"console=ttyS0"}
	img.KernelOpts = []string{"loglevel=3"}

	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	mf := manifest.New()
	_, err := img.InstantiateManifestFromContainer(&mf, containers, &runner.Fedora{}, rng)
	require.NoError(t, err)

	fakeSourceSpecs := map[string][]container.Spec{
		"build": []container.Spec{{Source: "some-src", Digest: makeFakeDigest(t), ImageID: makeFakeDigest(t)}},
		"image": []container.Spec{{Source: "other-src", Digest: makeFakeDigest(t), ImageID: makeFakeDigest(t)}},
	}
	osbm, err := mf.Serialize(nil, fakeSourceSpecs, nil, nil, nil)
	require.NoError(t, err)

	// Find the image pipeline and check for org.osbuild.bootc.install-to-filesystem
	pipeline := findPipelineFromOsbuildManifest(t, osbm, "image")
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.bootc.install-to-filesystem")
	assert.NotNil(t, stage, "org.osbuild.bootc.install-to-filesystem stage not found")

	// Find the bootc-rootfs pipeline and check compression type
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "bootc-rootfs")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.erofs")
	assert.NotNil(t, stage, "org.osbuild.erofs stage not found")

	// Find the efiboot-tree pipeline and check grub2 iso options
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "efiboot-tree")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.grub2.iso")
	require.NotNil(t, stage, "org.osbuild.grub2.iso stage not found")

	// check the efi grub2 kernel arguments for expected default values
	options := stage["options"].(map[string]any)
	require.NotNil(t, options)
	kernel := options["kernel"].(map[string]any)
	require.NotNil(t, kernel)
	efiopts := kernel["opts"].([]any)
	require.NotNil(t, efiopts)
	assert.Contains(t, efiopts, "ostree=@OSTREE@")
	assert.Contains(t, efiopts, "rd.live.image")
	assert.Contains(t, efiopts, "console=ttyS0")
	assert.Contains(t, efiopts, "loglevel=3")

	// Find the bootiso-tree pipeline and check grub2 iso legacy options
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "bootiso-tree")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.grub2.iso.legacy")
	require.NotNil(t, stage, "org.osbuild.grub2.iso.legacy stage not found")

	// check the iso grub2 kernel arguments for expected default values
	options = stage["options"].(map[string]any)
	require.NotNil(t, options)
	kernel = options["kernel"].(map[string]any)
	require.NotNil(t, kernel)
	isoopts := kernel["opts"].([]any)
	require.NotNil(t, isoopts)
	assert.Contains(t, isoopts, "ostree=@OSTREE@")
	assert.Contains(t, isoopts, "rd.live.image")
	assert.Contains(t, isoopts, "console=ttyS0")
	assert.Contains(t, isoopts, "loglevel=3")

	// UEFI grub2 and iso grub2 args should be identical
	assert.Equal(t, efiopts, isoopts)

	// Find the bootiso pipeline and check for xorrisofs
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "bootiso")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.xorrisofs")
	assert.NotNil(t, stage, "org.osbuild.xorrisofs stage not found")

	// check filename for bootc-test.iso
	stageOptions := stage["options"].(map[string]any)
	require.NotNil(t, stageOptions)
	assert.Equal(t, "bootc-test.iso", stageOptions["filename"].(string))
}
