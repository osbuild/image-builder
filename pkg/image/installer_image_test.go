package image_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func mockPackageSets() map[string]depsolvednf.DepsolveResult {
	repo := rpmmd.RepoConfig{Id: "dummy-repo-id"}
	pkgSets := map[string]depsolvednf.DepsolveResult{
		"build": {
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name: "coreutils",
						Checksum: rpmmd.Checksum{
							Type:  "sha256",
							Value: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
						},
						RemoteLocations: []string{"https://example.com/coreutils"},
						RepoID:          repo.Id,
						Repo:            &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
		"os": {
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name: "kernel",
						Checksum: rpmmd.Checksum{
							Type:  "sha256",
							Value: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
						},
						RemoteLocations: []string{"https://example.com/kernel"},
						RepoID:          repo.Id,
						Repo:            &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
		"anaconda-tree": {
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name: "kernel",
						Checksum: rpmmd.Checksum{
							Type:  "sha256",
							Value: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
						},
						RemoteLocations: []string{"https://example.com/kernel"},
						RepoID:          repo.Id,
						Repo:            &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
	}
	return pkgSets
}

func mockContainerSpecs() map[string][]container.Spec {
	return map[string][]container.Spec{
		"bootiso-tree": {
			{
				Source:  "repo.example.com/container",
				Digest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ImageID: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	}
}

func mockOSTreeCommitSpecs() map[string][]ostree.CommitSpec {
	return map[string][]ostree.CommitSpec{
		"bootiso-tree": {
			{
				Ref: "test/ostree/3",
				URL: "http://localhost:8080/repo",
			},
		},
	}
}

var testPlatform = &platform.Data{
	ImageFormat: platform.FORMAT_ISO,

	Arch:         arch.ARCH_X86_64,
	BIOSPlatform: "i386-pc",
	UEFIVendor:   "test",
}

const (
	product   = "Fedora"
	osversion = "40"
	isolabel  = "Fedora-40-Workstation-x86_64"
)

func TestContainerInstallerUnsetKSOptions(t *testing.T) {
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/osbuild.ks"`, isolabel))
}

func TestContainerInstallerUnsetKSPath(t *testing.T) {
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	// set empty kickstart options (no path)
	img.Kickstart = &kickstart.Options{}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/osbuild.ks"`, isolabel))
}

func TestContainerInstallerSetKSPath(t *testing.T) {
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.Kickstart = &kickstart.Options{
		Path: "/test.ks",
	}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/test.ks"`, isolabel))
	assert.NotContains(t, mfs, "osbuild.ks") // no mention of the default value anywhere
}

func TestContainerInstallerExt4Rootfs(t *testing.T) {
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)

	// Confirm that it includes the rootfs-image pipeline that makes the ext4 rootfs
	assert.Contains(t, mfs, `"name":"rootfs-image"`)
	assert.Contains(t, mfs, `"name:rootfs-image"`)
}

func TestContainerInstallerSquashfsRootfs(t *testing.T) {
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)

	// Confirm that it does not include rootfs-image pipeline
	assert.NotContains(t, mfs, `"name":"rootfs-image"`)
	assert.NotContains(t, mfs, `"name:rootfs-image"`)
}

func TestOSTreeInstallerUnsetKSPath(t *testing.T) {
	img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.Kickstart = &kickstart.Options{
		// the ostree options must be non-nil
		OSTree: &kickstart.OSTree{},
	}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs())
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/osbuild.ks"`, isolabel))
}

func TestOSTreeInstallerSetKSPath(t *testing.T) {
	img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.Kickstart = &kickstart.Options{
		// the ostree options must be non-nil
		OSTree: &kickstart.OSTree{},
		Path:   "/test.ks",
	}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs())
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/test.ks"`, isolabel))
	assert.NotContains(t, mfs, "osbuild.ks") // no mention of the default value anywhere
}

func TestOSTreeInstallerExt4Rootfs(t *testing.T) {
	img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.Kickstart = &kickstart.Options{
		// the ostree options must be non-nil
		OSTree: &kickstart.OSTree{},
	}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs())

	// Confirm that it includes the rootfs-image pipeline that makes the ext4 rootfs
	assert.Contains(t, mfs, `"name":"rootfs-image"`)
	assert.Contains(t, mfs, `"name:rootfs-image"`)
}

func TestOSTreeInstallerSquashfsRootfs(t *testing.T) {
	img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs
	img.Kickstart = &kickstart.Options{
		// the ostree options must be non-nil
		OSTree: &kickstart.OSTree{},
	}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs())

	// Confirm that it does not include rootfs-image pipeline
	assert.NotContains(t, mfs, `"name":"rootfs-image"`)
	assert.NotContains(t, mfs, `"name:rootfs-image"`)
}

func TestTarInstallerUnsetKSOptions(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)

	assert.Contains(t, mfs, fmt.Sprintf(`"inst.stage2=hd:LABEL=%s"`, isolabel))
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/osbuild.ks"`, isolabel))
}

func TestTarInstallerUnsetKSPath(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.Kickstart = &kickstart.Options{}

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)

	assert.Contains(t, mfs, fmt.Sprintf(`"inst.stage2=hd:LABEL=%s"`, isolabel))
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/osbuild.ks"`, isolabel))
}

func TestTarInstallerSetKSPath(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.Kickstart = &kickstart.Options{
		Path: "/test.ks",
	}

	// enable unattended to use the custom kickstart path instead of interactive-defaults
	img.Kickstart.Unattended = true
	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	assert.Contains(t, mfs, fmt.Sprintf(`"inst.ks=hd:LABEL=%s:/test.ks"`, isolabel))
	assert.NotContains(t, mfs, "osbuild.ks") // no mention of the default value anywhere
}

func TestTarInstallerExt4Rootfs(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	// Confirm that it includes the rootfs-image pipeline that makes the ext4 rootfs
	assert.Contains(t, mfs, `"name":"rootfs-image"`)
	assert.Contains(t, mfs, `"name:rootfs-image"`)
}

func TestTarInstallerSquashfsRootfs(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	// Confirm that it does not include rootfs-image pipeline
	assert.NotContains(t, mfs, `"name":"rootfs-image"`)
	assert.NotContains(t, mfs, `"name:rootfs-image"`)
}

func TestLiveInstallerExt4Rootfs(t *testing.T) {
	img := image.NewAnacondaLiveInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	// Confirm that it includes the rootfs-image pipeline that makes the ext4 rootfs
	assert.Contains(t, mfs, `"name":"rootfs-image"`)
	assert.Contains(t, mfs, `"name:rootfs-image"`)
}

func TestLiveInstallerSquashfsRootfs(t *testing.T) {
	img := image.NewAnacondaLiveInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	// Confirm that it does not include rootfs-image pipeline
	assert.NotContains(t, mfs, `"name":"rootfs-image"`)
	assert.NotContains(t, mfs, `"name:rootfs-image"`)
}

func instantiateAndSerialize(t *testing.T, img image.ImageKind, depsolved map[string]depsolvednf.DepsolveResult, containers map[string][]container.Spec, commits map[string][]ostree.CommitSpec) string {
	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	mf := manifest.New()
	_, err := img.InstantiateManifest(&mf, nil, &runner.CentOS{Version: 9}, rng)
	assert.NoError(t, err)

	fmt.Printf("Serializing with commits: %+v\n", commits)
	mfs, err := mf.Serialize(depsolved, containers, commits, nil, nil)
	assert.NoError(t, err)

	return string(mfs)
}

// NOTE(akoutsou):
// The following tests assert that the serialization of installer image kinds
// panics when the ISO-related properties aren't set (product name, product
// version, and ISO label). The panics happen in the stage validation, but we
// might want to catch them earlier (perhaps make them mandatory in the image
// kind or pipeline constructors) in the future.

func TestContainerInstallerPanics(t *testing.T) {
	assert := assert.New(t)
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	assert.PanicsWithError("org.osbuild.grub2.iso: product.name option is required", func() { instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil) })
	img.InstallerCustomizations.Product = product
	assert.PanicsWithError("org.osbuild.grub2.iso: product.version option is required", func() { instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil) })
	img.InstallerCustomizations.OSVersion = osversion
	assert.PanicsWithError("org.osbuild.grub2.iso: isolabel option is required", func() { instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil) })
}

func TestOSTreeInstallerPanics(t *testing.T) {
	assert := assert.New(t)
	img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
	img.Kickstart = &kickstart.Options{
		// the ostree options must be non-nil
		OSTree: &kickstart.OSTree{},
	}

	assert.PanicsWithError("org.osbuild.grub2.iso: product.name option is required",
		func() { instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs()) })

	img.InstallerCustomizations.Product = product
	assert.PanicsWithError("org.osbuild.grub2.iso: product.version option is required",
		func() { instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs()) })

	img.InstallerCustomizations.OSVersion = osversion
	assert.PanicsWithError("org.osbuild.grub2.iso: isolabel option is required",
		func() { instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs()) })
}

func TestTarInstallerPanics(t *testing.T) {
	assert := assert.New(t)
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")

	assert.PanicsWithError("org.osbuild.grub2.iso: product.name option is required",
		func() { instantiateAndSerialize(t, img, mockPackageSets(), nil, nil) })

	img.InstallerCustomizations.Product = product
	assert.PanicsWithError("org.osbuild.grub2.iso: product.version option is required",
		func() { instantiateAndSerialize(t, img, mockPackageSets(), nil, nil) })

	img.InstallerCustomizations.OSVersion = osversion
	assert.PanicsWithError("org.osbuild.grub2.iso: isolabel option is required",
		func() { instantiateAndSerialize(t, img, mockPackageSets(), nil, nil) })
}

func findAnacondaStageModules(t *testing.T, mf manifest.OSBuildManifest, legacyOptions bool) []interface{} {
	pipeline := findPipelineFromOsbuildManifest(t, mf, "anaconda-tree")
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.anaconda")
	assert.NotNil(t, stage)
	anacondaStageOptions := stage["options"].(map[string]interface{})
	assert.NotNil(t, anacondaStageOptions)

	// NOTE: remove this condition and the legacyOptions function argument when
	// we remove support for RHEL 8.
	modulesKey := "activatable-modules"
	if legacyOptions {
		modulesKey = "kickstart-modules"
	}

	if _, ok := anacondaStageOptions[modulesKey]; !ok {
		return []interface{}{}
	}

	return anacondaStageOptions[modulesKey].([]interface{})
}

type testCase struct {
	enable   []string
	disable  []string
	expected []string
}

var moduleTestCases = map[string]testCase{
	"empty-args": {
		enable:   []string{},
		expected: []string{},
	},
	"enable-users": {
		enable: []string{
			anaconda.ModuleUsers,
		},
		expected: []string{
			anaconda.ModuleUsers,
		},
	},
	"disable-storage": {
		disable: []string{
			anaconda.ModuleStorage,
		},
		expected: []string{},
	},
	"enable-users-disable-storage": {
		enable: []string{
			anaconda.ModuleUsers,
		},
		disable: []string{
			anaconda.ModuleStorage,
		},
		expected: []string{
			anaconda.ModuleUsers,
		},
	},
}

func TestContainerInstallerModules(t *testing.T) {
	for name := range moduleTestCases {
		tc := moduleTestCases[name]
		// Run each test case twice: once with activatable-modules and once with kickstart-modules.
		// Remove this when we drop support for RHEL 8.
		for _, legacy := range []bool{true, false} {
			t.Run(name, func(t *testing.T) {
				img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
				img.InstallerCustomizations.Product = product
				img.InstallerCustomizations.OSVersion = osversion
				img.ISOCustomizations.Label = isolabel

				img.InstallerCustomizations.UseLegacyAnacondaConfig = legacy
				img.InstallerCustomizations.EnabledAnacondaModules = tc.enable
				img.InstallerCustomizations.DisabledAnacondaModules = tc.disable

				assert.NotNil(t, img)
				mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
				modules := findAnacondaStageModules(t, manifest.OSBuildManifest(mfs), legacy)
				assert.NotNil(t, modules)
				assert.ElementsMatch(t, modules, tc.expected)
			})
		}
	}
}

func TestOSTreeInstallerModules(t *testing.T) {
	for name := range moduleTestCases {
		tc := moduleTestCases[name]
		// Run each test case twice: once with activatable-modules and once with kickstart-modules.
		// Remove this when we drop support for RHEL 8.
		for _, legacy := range []bool{true, false} {
			t.Run(name, func(t *testing.T) {
				img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
				img.InstallerCustomizations.Product = product
				img.InstallerCustomizations.OSVersion = osversion
				img.ISOCustomizations.Label = isolabel
				img.Kickstart = &kickstart.Options{
					// the ostree options must be non-nil
					OSTree: &kickstart.OSTree{},
				}

				img.InstallerCustomizations.UseLegacyAnacondaConfig = legacy
				img.InstallerCustomizations.EnabledAnacondaModules = tc.enable
				img.InstallerCustomizations.DisabledAnacondaModules = tc.disable

				assert.NotNil(t, img)
				mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs())
				modules := findAnacondaStageModules(t, manifest.OSBuildManifest(mfs), legacy)
				assert.NotNil(t, modules)
				assert.ElementsMatch(t, modules, tc.expected)
			})
		}
	}
}

func TestTarInstallerModules(t *testing.T) {
	for name := range moduleTestCases {
		tc := moduleTestCases[name]
		// Run each test case twice: once with activatable-modules and once with kickstart-modules.
		// Remove this when we drop support for RHEL 8.
		for _, legacy := range []bool{true, false} {
			t.Run(name, func(t *testing.T) {
				img := image.NewAnacondaTarInstaller(testPlatform, "filename")
				img.InstallerCustomizations.Product = product
				img.InstallerCustomizations.OSVersion = osversion
				img.ISOCustomizations.Label = isolabel
				img.InstallerCustomizations.UseLegacyAnacondaConfig = legacy
				img.InstallerCustomizations.EnabledAnacondaModules = tc.enable
				img.InstallerCustomizations.DisabledAnacondaModules = tc.disable

				assert.NotNil(t, img)
				mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
				modules := findAnacondaStageModules(t, manifest.OSBuildManifest(mfs), legacy)
				assert.NotNil(t, modules)
				assert.ElementsMatch(t, modules, tc.expected)
			})
		}
	}
}

func findAnacondaLocale(t *testing.T, mf manifest.OSBuildManifest) string {
	pipeline := findPipelineFromOsbuildManifest(t, mf, "anaconda-tree")
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.locale")
	localeStageOptions := stage["options"].(map[string]interface{})
	language := localeStageOptions["language"].(string)
	return language
}

func TestInstallerLocales(t *testing.T) {
	assert := assert.New(t)

	locales := map[string]string{
		// input: expected output
		"C.UTF-8":     "C.UTF-8",
		"en_US.UTF-8": "en_US.UTF-8",
		"":            "C.UTF-8",  // default
		"whatever":    "whatever", // arbitrary string
	}

	for input, expected := range locales {
		{ // Container
			img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
			assert.NotNil(t, img)

			img.InstallerCustomizations.Product = product
			img.InstallerCustomizations.OSVersion = osversion
			img.ISOCustomizations.Label = isolabel
			img.Locale = input

			mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
			actual := findAnacondaLocale(t, manifest.OSBuildManifest(mfs))

			assert.Equal(expected, actual)
		}

		{ // OSTree
			img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
			assert.NotNil(t, img)

			img.InstallerCustomizations.Product = product
			img.InstallerCustomizations.OSVersion = osversion
			img.ISOCustomizations.Label = isolabel
			img.Kickstart = &kickstart.Options{
				// the ostree options must be non-nil
				OSTree: &kickstart.OSTree{},
			}
			img.Locale = input

			mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
			actual := findAnacondaLocale(t, manifest.OSBuildManifest(mfs))

			assert.Equal(expected, actual)
		}

		{ // Tar
			img := image.NewAnacondaTarInstaller(testPlatform, "filename")
			assert.NotNil(t, img)

			img.InstallerCustomizations.Product = product
			img.InstallerCustomizations.OSVersion = osversion
			img.ISOCustomizations.Label = isolabel
			img.OSCustomizations.Language = input

			mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
			actual := findAnacondaLocale(t, manifest.OSBuildManifest(mfs))

			assert.Equal(expected, actual)
		}

		{ // Net
			img := image.NewAnacondaNetInstaller(testPlatform, "filename")
			assert.NotNil(t, img)

			img.InstallerCustomizations.Product = product
			img.InstallerCustomizations.OSVersion = osversion
			img.ISOCustomizations.Label = isolabel
			img.Language = input

			mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
			actual := findAnacondaLocale(t, manifest.OSBuildManifest(mfs))

			assert.Equal(expected, actual)
		}

		{ // Live
			img := image.NewAnacondaLiveInstaller(testPlatform, "filename")
			assert.NotNil(t, img)

			img.InstallerCustomizations.Product = product
			img.InstallerCustomizations.OSVersion = osversion
			img.ISOCustomizations.Label = isolabel
			img.Locale = input

			mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
			actual := findAnacondaLocale(t, manifest.OSBuildManifest(mfs))

			assert.Equal(expected, actual)
		}
	}
}

// getStageOptions returns the list of strings from a specific option name
func getStageOptions(stageOptions map[string]any, name string) []string {
	var options []string
	if _, ok := stageOptions[name]; ok {
		for _, value := range stageOptions[name].([]any) {
			options = append(options, value.(string))
		}
	}

	return options
}

// findDracutStageOptions returns information about the dracut stage
// This includes:
// - modules
// - add_modules
// - drivers
// - add_drivers
func findDracutStageOptions(t *testing.T, mf manifest.OSBuildManifest, pipelineName string) ([]string, []string, []string, []string) {
	pipeline := findPipelineFromOsbuildManifest(t, mf, pipelineName)
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.dracut")
	assert.NotNil(t, stage)
	stageOptions := stage["options"].(map[string]any)
	assert.NotNil(t, stageOptions)

	modules := getStageOptions(stageOptions, "modules")
	addModules := getStageOptions(stageOptions, "add_modules")
	drivers := getStageOptions(stageOptions, "drivers")
	addDrivers := getStageOptions(stageOptions, "add_drivers")

	return modules, addModules, drivers, addDrivers
}

func findGrub2IsoStageOptions(t *testing.T, mf manifest.OSBuildManifest, pipelineName string) []string {
	pipeline := findPipelineFromOsbuildManifest(t, mf, pipelineName)
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.grub2.iso")
	assert.NotNil(t, stage)
	stageOptions := stage["options"].(map[string]any)
	assert.NotNil(t, stageOptions)

	stageKernelOptions := stageOptions["kernel"].(map[string]any)
	assert.NotNil(t, stageKernelOptions)

	kernelOpts := getStageOptions(stageKernelOptions, "opts")

	return kernelOpts
}

func TestContainerInstallerDracut(t *testing.T) {
	img := image.NewAnacondaContainerInstallerLegacy(testPlatform, "filename", container.SourceSpec{})
	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	testModules := []string{"test-module"}
	testDrivers := []string{"test-driver"}

	img.InstallerCustomizations.AdditionalDracutModules = testModules
	img.InstallerCustomizations.AdditionalDrivers = testDrivers

	assert.NotNil(t, img)
	mfs := instantiateAndSerialize(t, img, mockPackageSets(), mockContainerSpecs(), nil)
	modules, addModules, drivers, addDrivers := findDracutStageOptions(t, manifest.OSBuildManifest(mfs), "anaconda-tree")
	assert.Nil(t, modules)
	assert.NotNil(t, addModules)
	assert.Nil(t, drivers)
	assert.NotNil(t, addDrivers)

	assert.Subset(t, addModules, testModules)
	assert.Subset(t, addDrivers, testDrivers)
}

func TestOSTreeInstallerDracut(t *testing.T) {
	img := image.NewAnacondaOSTreeInstaller(testPlatform, "filename", ostree.SourceSpec{})
	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	img.Kickstart = &kickstart.Options{
		// the ostree options must be non-nil
		OSTree: &kickstart.OSTree{},
	}

	testModules := []string{"test-module"}
	testDrivers := []string{"test-driver"}

	img.InstallerCustomizations.AdditionalDracutModules = testModules
	img.InstallerCustomizations.AdditionalDrivers = testDrivers

	assert.NotNil(t, img)
	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, mockOSTreeCommitSpecs())
	modules, addModules, drivers, addDrivers := findDracutStageOptions(t, manifest.OSBuildManifest(mfs), "anaconda-tree")
	assert.Nil(t, modules)
	assert.NotNil(t, addModules)
	assert.Nil(t, drivers)
	assert.NotNil(t, addDrivers)

	assert.Subset(t, addModules, testModules)
	assert.Subset(t, addDrivers, testDrivers)
}

func TestTarInstallerDracut(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	testModules := []string{"test-module"}
	testDrivers := []string{"test-driver"}

	img.InstallerCustomizations.AdditionalDracutModules = testModules
	img.InstallerCustomizations.AdditionalDrivers = testDrivers

	assert.NotNil(t, img)
	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	modules, addModules, drivers, addDrivers := findDracutStageOptions(t, manifest.OSBuildManifest(mfs), "anaconda-tree")
	assert.Nil(t, modules)
	assert.NotNil(t, addModules)
	assert.Nil(t, drivers)
	assert.NotNil(t, addDrivers)

	assert.Subset(t, addModules, testModules)
	assert.Subset(t, addDrivers, testDrivers)
}

func TestTarInstallerKernelOpts(t *testing.T) {
	img := image.NewAnacondaTarInstaller(testPlatform, "filename")
	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	testOpts := []string{"foo=1", "bar=2"}

	img.InstallerCustomizations.KernelOptionsAppend = testOpts

	assert.NotNil(t, img)
	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)

	opts := findGrub2IsoStageOptions(t, manifest.OSBuildManifest(mfs), "efiboot-tree")

	assert.NotNil(t, opts)
	assert.Subset(t, opts, testOpts)
}

func TestNetInstallerExt4Rootfs(t *testing.T) {
	img := image.NewAnacondaNetInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	// Confirm that it includes the rootfs-image pipeline that makes the ext4 rootfs
	assert.Contains(t, mfs, `"name":"rootfs-image"`)
	assert.Contains(t, mfs, `"name:rootfs-image"`)
}

func TestNetInstallerSquashfsRootfs(t *testing.T) {
	img := image.NewAnacondaNetInstaller(testPlatform, "filename")
	assert.NotNil(t, img)

	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs

	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	// Confirm that it does not include rootfs-image pipeline
	assert.NotContains(t, mfs, `"name":"rootfs-image"`)
	assert.NotContains(t, mfs, `"name:rootfs-image"`)
}

func TestNetInstallerDracut(t *testing.T) {
	img := image.NewAnacondaNetInstaller(testPlatform, "filename")
	img.InstallerCustomizations.Product = product
	img.InstallerCustomizations.OSVersion = osversion
	img.ISOCustomizations.Label = isolabel
	testModules := []string{"test-module"}
	testDrivers := []string{"test-driver"}

	img.InstallerCustomizations.AdditionalDracutModules = testModules
	img.InstallerCustomizations.AdditionalDrivers = testDrivers

	assert.NotNil(t, img)
	mfs := instantiateAndSerialize(t, img, mockPackageSets(), nil, nil)
	modules, addModules, drivers, addDrivers := findDracutStageOptions(t, manifest.OSBuildManifest(mfs), "anaconda-tree")
	assert.Nil(t, modules)
	assert.NotNil(t, addModules)
	assert.Nil(t, drivers)
	assert.NotNil(t, addDrivers)

	assert.Subset(t, addModules, testModules)
	assert.Subset(t, addDrivers, testDrivers)
}
