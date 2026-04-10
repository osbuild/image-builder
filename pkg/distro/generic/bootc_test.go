package generic

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/randutil"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/bootc"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/bootc/bootctest"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/manifestgen"
	"github.com/osbuild/images/pkg/manifestgen/manifestmock"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/osbuild/manifesttest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBootc(t *testing.T) {
	type testCase struct {
		info           *bootc.Info
		expectedDistro *BootcDistro
		expectedError  string
	}

	testCases := map[string]testCase{
		"empty": {
			expectedError: "failed to initialize bootc distro: container info is empty",
		},

		"ok": {
			info: &bootc.Info{
				Imgref:        "example.com/containers/distro-bootc:version12",
				ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
				Arch:          "arm64",
				DefaultRootFs: "xfs",
				Size:          100 * datasizes.MiB,
				OSInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "distroID",
						VersionID: "83",
					},
				},
			},
			expectedDistro: &BootcDistro{
				imgref:      "example.com/containers/distro-bootc:version12",
				imageID:     "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
				buildImgref: "example.com/containers/distro-bootc:version12",
				sourceInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "distroID",
						VersionID: "83",
					},
				},
				buildSourceInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "distroID",
						VersionID: "83",
					},
				},
				id: distro.ID{
					Name:         "bootc-distroID",
					MajorVersion: 83,
					MinorVersion: -1,
				},

				releasever:    "83",
				defaultFs:     "xfs",
				rootfsMinSize: 200 * datasizes.MiB,
				arches: map[string]distro.Arch{
					"aarch64": &architecture{
						arch: arch.ARCH_AARCH64,
					},
				},
			},
		},

		"noimgref": {
			info: &bootc.Info{
				ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
				Arch:          "aarch64",
				DefaultRootFs: "xfs",
				Size:          100 * datasizes.MiB,
				OSInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "fedora",
						VersionID: "2000",
					},
				},
			},
			expectedError: "failed to initialize bootc distro: missing required info: Imgref",
		},

		"noimageid": {
			info: &bootc.Info{
				Imgref:        "example.com/containers/distro-bootc:version12",
				Arch:          "amd64",
				DefaultRootFs: "xfs",
				Size:          100 * datasizes.MiB,
				OSInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "aos",
						VersionID: "5000",
					},
				},
			},
			expectedDistro: &BootcDistro{
				imgref:      "example.com/containers/distro-bootc:version12",
				buildImgref: "example.com/containers/distro-bootc:version12",
				sourceInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "aos",
						VersionID: "5000",
					},
				},
				buildSourceInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "aos",
						VersionID: "5000",
					},
				},
				id: distro.ID{
					Name:         "bootc-aos",
					MajorVersion: 5000,
					MinorVersion: -1,
				},

				releasever:    "5000",
				defaultFs:     "xfs",
				rootfsMinSize: 200 * datasizes.MiB,
				arches: map[string]distro.Arch{
					"x86_64": &architecture{
						arch: arch.ARCH_X86_64,
					},
				},
			},
		},

		"missing-multiple": {
			info: &bootc.Info{
				Imgref: "example.com/containers/distro-bootc:version12",
			},
			expectedError: "failed to initialize bootc distro: missing required info: Arch, DefaultRootFs, Size, OSInfo",
		},

		"osinfo-without-values": {
			info: &bootc.Info{
				Imgref:        "example.com/containers/distro-bootc:version12",
				ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
				Arch:          "aarch64",
				DefaultRootFs: "xfs",
				Size:          100 * datasizes.MiB,
				OSInfo:        &osinfo.Info{},
			},
			expectedError: "failed to initialize bootc distro: missing required info: OSInfo.OSRelease.ID, OSInfo.OSRelease.VersionID",
		},

		"unknown-arch": {
			info: &bootc.Info{
				Imgref:        "example.com/containers/distro-bootc:version12",
				ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
				Arch:          "not-an-arch",
				DefaultRootFs: "xfs",
				Size:          100 * datasizes.MiB,
				OSInfo: &osinfo.Info{
					OSRelease: osinfo.OSRelease{
						ID:        "aos",
						VersionID: "5000",
					},
				},
			},
			expectedError: "failed to set bootc distro architecture: unsupported architecture \"not-an-arch\"",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			d, err := NewBootc("bootc", tc.info)

			if tc.expectedError != "" {
				require.EqualError(err, tc.expectedError)
				return
			}

			require.NotNil(d)
			loadImageTypes(t, tc.expectedDistro)
			require.Equal(tc.expectedDistro, d)
		})
	}
}

func TestSetBuildContainer(t *testing.T) {
	// base bootc container info to initialise the distro before setting the
	// build container info
	baseBootcInfo := &bootc.Info{
		Imgref:        "example.com/containers/distro-bootc:version12",
		ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
		Arch:          "aarch64",
		DefaultRootFs: "xfs",
		Size:          100 * datasizes.MiB,
		OSInfo: &osinfo.Info{
			OSRelease: osinfo.OSRelease{
				ID:        "whatever",
				VersionID: "39",
			},
		},
	}

	type testCase struct {
		buildInfo       *bootc.Info
		expectedImgref  string
		expectedImageID string
		expectedError   string
	}

	testCases := map[string]testCase{
		"empty": {
			expectedError: "failed to set build container for bootc distro: container info is empty",
		},

		"ok": {
			buildInfo: &bootc.Info{
				Imgref:  "example.com/containers/distro-bootc:build42",
				ImageID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Arch:    "arm64",
			},
			expectedImgref:  "example.com/containers/distro-bootc:build42",
			expectedImageID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},

		"noimgref": {
			buildInfo: &bootc.Info{
				ImageID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Arch:    "arm64",
			},
			expectedError: "failed to set build container for bootc distro: missing required info: Imgref",
		},

		"missing-multiple": {
			buildInfo: &bootc.Info{
				ImageID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			expectedError: "failed to set build container for bootc distro: missing required info: Imgref, Arch",
		},

		"noimageid": {
			buildInfo: &bootc.Info{
				Imgref: "example.com/containers/distro-bootc:build13",
				Arch:   "arm64",
			},
			expectedImgref: "example.com/containers/distro-bootc:build13",
		},

		"arch-mismatch": {
			buildInfo: &bootc.Info{
				Imgref: "example.com/containers/distro-bootc:build99",
				Arch:   "amd64",
			},
			expectedError: "failed to set build container for bootc distro: build container architecture \"x86_64\" does not match base container \"aarch64\"",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			bd, err := NewBootc("bootc", baseBootcInfo)
			require.NoError(err)
			require.NotNil(bd)

			err = bd.SetBuildContainer(tc.buildInfo)
			if tc.expectedError != "" {
				require.EqualError(err, tc.expectedError)
				return
			}

			require.Equal(tc.expectedImgref, bd.buildImgref)
			require.Equal(tc.expectedImageID, bd.buildImageID)
		})
	}
}

func TestSetBuildContainerWrongNumArches(t *testing.T) {
	baseBootcInfo := &bootc.Info{
		Imgref:        "example.com/containers/distro-bootc:version12",
		ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
		Arch:          "aarch64",
		DefaultRootFs: "xfs",
		Size:          100 * datasizes.MiB,
		OSInfo: &osinfo.Info{
			OSRelease: osinfo.OSRelease{
				ID:        "whatever",
				VersionID: "39",
			},
		},
	}
	buildInfo := &bootc.Info{
		Imgref: "example.com/containers/distro-bootc:build99",
		Arch:   "aarch64",
	}

	require := require.New(t)
	bd, err := NewBootc("bootc", baseBootcInfo)
	require.NoError(err)
	require.NotNil(bd)

	require.Len(bd.arches, 1)

	// add a second architecture to test the error handling
	bd.arches["s390x"] = &architecture{
		distro:     bd,
		arch:       arch.ARCH_S390X,
		imageTypes: map[string]distro.ImageType{},
	}
	require.EqualError(bd.SetBuildContainer(buildInfo), "found 2 architectures for bootc distro while setting build container: bootc distro should have exactly 1 architecture")

	// remove the architectures to test the error handling
	bd.arches = nil
	require.EqualError(bd.SetBuildContainer(buildInfo), "found 0 architectures for bootc distro while setting build container: bootc distro should have exactly 1 architecture")
}

// Helper function for loading static bootc image type definitions onto the
// expected distro object.
func loadImageTypes(t *testing.T, d *BootcDistro) {
	t.Helper()

	require := require.New(t)

	distroYAML, err := defs.LoadDistroWithoutImageTypes("bootc-generic-1")
	require.NoError(err)

	fs, err := disk.NewFSType(d.defaultFs)
	require.NoError(err)

	distroYAML.DefaultFSType = fs // It's very weird that this is required here
	require.NoError(distroYAML.LoadImageTypes())

	for archName, arch := range d.arches {
		darch := arch.(*architecture)
		darch.imageTypes = map[string]distro.ImageType{}
		darch.distro = d // link distro to architecture as well
		require.NotNil(darch)
		for _, imgTypeYaml := range distroYAML.ImageTypes() {
			require.NoError(darch.addBootcImageType(bootcImageType{ImageTypeYAML: imgTypeYaml}))
		}
		d.arches[archName] = darch
	}
}

type manifestTestCase struct {
	config            *blueprint.Blueprint
	imageOptions      distro.ImageOptions
	imageRef          string
	imageType         string
	depsolved         map[string]depsolvednf.DepsolveResult
	containers        map[string][]container.Spec
	expStages         map[string][]string
	notExpectedStages map[string][]string
	err               string
	warnings          []string
}

func NewTestBootcDistro(t *testing.T) *BootcDistro {
	t.Helper()
	distro, err := NewBootc("bootc", &bootc.Info{
		Imgref:        "example.com/containers/distro-bootc:version12",
		ImageID:       "acf88e518194fac963a1b2e2e4110e38a4ce5fb3fceddd624fae8997d4566930",
		Arch:          "amd64",
		DefaultRootFs: "xfs",
		Size:          100 * datasizes.MiB,
		OSInfo: &osinfo.Info{
			OSRelease: osinfo.OSRelease{
				Name:      "DistroID",
				ID:        "distroID",
				VersionID: "83",
			},
			KernelInfo: &osinfo.KernelInfo{
				Version: "6.17.7-300.fc43.x86_64",
			},
			InitrdModules: []string{"ostree", "livenet", "dmsquash-live"},
		},
	})
	require.NoError(t, err)
	return distro
}

func NewTestBootcImageType(t *testing.T, imgTypeName string) *bootcImageType {
	t.Helper()
	distro := NewTestBootcDistro(t)
	arch, err := distro.GetArch("x86_64")
	require.NoError(t, err)
	imgType, err := arch.GetImageType(imgTypeName)
	require.NoError(t, err)
	return imgType.(*bootcImageType)
}

func getUserConfig() *blueprint.Blueprint {
	// add a user
	pass := randutil.String(20)
	key := "ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	return &blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			User: []blueprint.UserCustomization{
				{
					Name:     "tester",
					Password: &pass,
					Key:      &key,
				},
			},
		},
	}
}

func TestManifestGenerationUserConfig(t *testing.T) {
	userConfig := getUserConfig()
	testCases := map[string]manifestTestCase{
		"qcow2-user": {
			config:    userConfig,
			imageType: "qcow2",
		},
		"pxe-user": {
			config:    userConfig,
			imageType: "pxe-tar-xz",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			imgType := NewTestBootcImageType(t, tc.imageType)
			require.NotNil(t, imgType)
			_, _, err := imgType.Manifest(tc.config, tc.imageOptions, nil, common.ToPtr(int64(0)))
			assert.NoError(t, err)
		})
	}
}

// Disk images require a container for the build/image pipelines
var containerSpec = container.Spec{
	Source:  "test-container",
	Digest:  "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	ImageID: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
}

// diskContainers can be passed to Serialize() to get a minimal disk image
var diskContainers = map[string][]container.Spec{
	"build": {
		containerSpec,
	},
	"image": {
		containerSpec,
	},
	"target": {
		containerSpec,
	},
}

// isoContainers can be passed to Serialize() to get a minimal bootc-generic-iso image
var isoContainers = map[string][]container.Spec{
	"build": {
		containerSpec,
	},
	"os-tree": {
		containerSpec,
	},
}

// simplified representation of a manifest
type testManifest struct {
	Pipelines []pipeline `json:"pipelines"`
}
type pipeline struct {
	Name   string  `json:"name"`
	Stages []stage `json:"stages"`
}
type stage struct {
	Type string `json:"type"`
}

func checkStages(serialized manifest.OSBuildManifest, pipelineStages map[string][]string, missingStages map[string][]string) error {
	mf := &testManifest{}
	if err := json.Unmarshal(serialized, mf); err != nil {
		return err
	}
	pipelineMap := map[string]pipeline{}
	for _, pl := range mf.Pipelines {
		pipelineMap[pl.Name] = pl
	}

	for plname, stages := range pipelineStages {
		pl, found := pipelineMap[plname]
		if !found {
			return fmt.Errorf("pipeline %q not found", plname)
		}

		stageMap := map[string]bool{}
		for _, stage := range pl.Stages {
			stageMap[stage.Type] = true
		}
		for _, stage := range stages {
			if _, found := stageMap[stage]; !found {
				return fmt.Errorf("pipeline %q - stage %q - not found", plname, stage)
			}
		}
	}

	for plname, stages := range missingStages {
		pl, found := pipelineMap[plname]
		if !found {
			return fmt.Errorf("pipeline %q not found", plname)
		}

		stageMap := map[string]bool{}
		for _, stage := range pl.Stages {
			stageMap[stage.Type] = true
		}
		for _, stage := range stages {
			if _, found := stageMap[stage]; found {
				return fmt.Errorf("pipeline %q - stage %q - found (but should not be)", plname, stage)
			}
		}
	}

	return nil
}

func TestManifestSerialization(t *testing.T) {
	baseConfig := &blueprint.Blueprint{}
	userConfig := getUserConfig()
	testCases := map[string]manifestTestCase{
		"qcow2-base": {
			config:     baseConfig,
			imageType:  "qcow2",
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
				"image": {
					"org.osbuild.users",
				},
			},
		},
		"qcow2-user": {
			config:     userConfig,
			imageType:  "qcow2",
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.users", // user creation stage when we add users
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
			},
		},
		"qcow2-nocontainer": {
			config:    userConfig,
			imageType: "qcow2",
			err:       `cannot serialize pipeline "build": BuildrootFromContainer: serialization not started`,
		},
		"pxe-base": {
			config:     baseConfig,
			imageType:  "pxe-tar-xz",
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
				"image": {
					"org.osbuild.users",
				},
			},
		},
		"pxe-user": {
			config:     userConfig,
			imageType:  "pxe-tar-xz",
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.users", // user creation stage when we add users
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
			},
		},
		"pxe-nocontainer": {
			config:    userConfig,
			imageType: "pxe-tar-xz",
			err:       `cannot serialize pipeline "build": BuildrootFromContainer: serialization not started`,
		},
	}

	// Use an empty config: only the imgref is required
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			imgType := NewTestBootcImageType(t, tc.imageType)

			assert := assert.New(t)
			mf, _, err := imgType.Manifest(tc.config, tc.imageOptions, nil, common.ToPtr(int64(0)))
			assert.NoError(err) // this isn't the error we're testing for

			if tc.err != "" {
				_, err := mf.Serialize(tc.depsolved, tc.containers, nil, nil, nil)
				assert.EqualError(err, tc.err)
			} else {
				manifestJson, err := mf.Serialize(tc.depsolved, tc.containers, nil, nil, nil)
				assert.NoError(err)
				assert.NoError(checkStages(manifestJson, tc.expStages, tc.notExpectedStages))
			}
		})
	}
}

func TestBootcDistroGetArch(t *testing.T) {
	imgType := NewTestBootcImageType(t, "qcow2")
	distro := imgType.Arch().Distro()

	arch, err := distro.GetArch("x86_64")
	assert.NoError(t, err)
	assert.Equal(t, arch, imgType.Arch())

	_, err = distro.GetArch("aarch64")
	assert.EqualError(t, err, `requested bootc arch "aarch64" does not match available arches [x86_64]`)
}

func TestManifestGenerationOvaFilename(t *testing.T) {
	bp := getUserConfig()
	imgOptions := distro.ImageOptions{}

	bd := NewTestBootcDistro(t)
	imgType, err := bd.arches["x86_64"].GetImageType("ova")
	assert.NoError(t, err)

	mf, _, err := imgType.Manifest(bp, imgOptions, nil, common.ToPtr(int64(0)))
	assert.NoError(t, err)
	manifestJson, err := mf.Serialize(nil, diskContainers, nil, nil, nil)
	assert.NoError(t, err)
	mani, err := manifesttest.NewManifestFromBytes(manifestJson)
	assert.NoError(t, err)
	archivePipeline := mani.Pipeline("archive")
	assert.NotNil(t, archivePipeline)
	stages := archivePipeline.Stages
	assert.Len(t, stages, 1)
	var tarStageOptions osbuild.TarStageOptions
	err = json.Unmarshal(stages[0].Options, &tarStageOptions)
	assert.NoError(t, err)
	assert.Equal(t, "image.ova", tarStageOptions.Filename)
}

func TestManifestGenerationBlueprintValidation(t *testing.T) {
	imageOptions := distro.ImageOptions{}
	config := &blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Repositories: []blueprint.RepositoryCustomization{
				{
					Id: "foo",
				},
			},
		},
	}

	testCases := map[string]manifestTestCase{
		"qcow2-base": {
			config:       config,
			imageOptions: imageOptions,
			imageRef:     "example-img-ref",
			imageType:    "qcow2",
			warnings:     []string{`blueprint validation failed for image type "qcow2": customizations.repositories: not supported`},
		},
		"pxe-base": {
			config:       config,
			imageOptions: imageOptions,
			imageRef:     "example-img-ref",
			imageType:    "pxe-tar-xz",
			warnings:     []string{`blueprint validation failed for image type "pxe-tar-xz": customizations.repositories: not supported`},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			imgType := NewTestBootcImageType(t, tc.imageType)
			assert := assert.New(t)
			_, warnings, err := imgType.Manifest(config, imageOptions, nil, common.ToPtr(int64(0)))
			if tc.err != "" {
				assert.EqualError(err, tc.err)
			}
			if len(tc.warnings) > 0 {
				assert.Equal(tc.warnings, warnings)
			}
		})
	}
}

func TestBootcIsoManifestSerialization(t *testing.T) {
	bd := NewTestBootcDistro(t)
	imgType, err := bd.arches["x86_64"].GetImageType("bootc-generic-iso")
	assert.NoError(t, err)

	bp := &blueprint.Blueprint{}
	imgOptions := distro.ImageOptions{}

	mf, _, err := imgType.Manifest(bp, imgOptions, nil, common.ToPtr(int64(0)))
	assert.NoError(t, err)

	manifestJson, err := mf.Serialize(nil, isoContainers, nil, nil, nil)
	assert.NoError(t, err)

	expStages := map[string][]string{
		"build":   {"org.osbuild.container-deploy"},
		"os-tree": {"org.osbuild.container-deploy"},
		"bootiso": {"org.osbuild.xorrisofs"},
	}
	assert.NoError(t, checkStages(manifestJson, expStages, nil))
}

func TestContainerSourceLocality(t *testing.T) {
	bd := NewTestBootcDistro(t)
	archi, err := bd.GetArch("x86_64")
	require.NoError(t, err)

	for _, local := range []bool{true, false} {
		for _, imgTypeName := range archi.ListImageTypes() {
			name := fmt.Sprintf("%s-local=%v", imgTypeName, local)
			t.Run(name, func(t *testing.T) {
				imgType, err := archi.GetImageType(imgTypeName)
				require.NoError(t, err)

				// The legacy ISO (anaconda-iso, iso) loads a real distro
				// definition via newDistroYAMLFrom() to find installer
				// packages. The generic test distro doesn't carry real OS
				// release data, so this image type cannot be tested here.
				if imgTypeName == "anaconda-iso" || imgTypeName == "iso" {
					t.Skipf("skipping %s: legacy ISO requires real distro definitions not available in the test distro", imgTypeName)
				}

				imgOptions := distro.ImageOptions{
					Bootc: &distro.BootcImageOptions{
						// InstallerPayloadRef is required by the bootc_iso image
						// type (bootc-installer) but harmlessly ignored by disk and
						// PXE types. For bootc_generic_iso it adds an optional
						// payload container whose locality follows the same flag.
						// Setting it unconditionally keeps the test simple and
						// focused on container source locality.
						InstallerPayloadRef:      "registry.example.com/payload:latest",
						UseRemoteContainerSource: !local,
					},
				}

				mf, _, err := imgType.Manifest(&blueprint.Blueprint{}, imgOptions, nil, common.ToPtr(int64(0)))
				require.NoError(t, err)

				containerSpecs := manifestmock.ResolveContainers(mf.GetContainerSourceSpecs())

				manifestJson, err := mf.Serialize(nil, containerSpecs, nil, nil, nil)
				require.NoError(t, err)

				mani, err := manifesttest.NewManifestFromBytes(manifestJson)
				require.NoError(t, err)

				if local {
					assert.Contains(t, mani.Sources, "org.osbuild.containers-storage")
					assert.NotContains(t, mani.Sources, "org.osbuild.skopeo")
				} else {
					assert.Contains(t, mani.Sources, "org.osbuild.skopeo")
					assert.NotContains(t, mani.Sources, "org.osbuild.containers-storage")
				}
			})
		}
	}
}

func canRunIntegration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("test needs root")
	}
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("test needs installed podman")
	}
	if _, err := exec.LookPath("systemd-detect-virt"); err != nil {
		t.Skip("test needs systemd-detect-virt")
	}
	// exit code "0" means the container is detected
	if err := exec.Command("systemd-detect-virt", "-c", "-q").Run(); err == nil {
		t.Skip("test cannot run inside a container")
	}

}

func genManifest(t *testing.T, imgType distro.ImageType) string {
	var bp blueprint.Blueprint

	mg, err := manifestgen.New(nil, &manifestgen.Options{
		OverrideRepos: []rpmmd.RepoConfig{
			{Id: "not-used", BaseURLs: []string{"not-used"}},
		},
	})
	assert.NoError(t, err)
	manifestJson, err := mg.Generate(&bp, imgType, nil)
	assert.NoError(t, err)

	// XXX: it would be nice to return an *osbuild.Manifest here
	// and do all of this more structed, however this is not
	// working currently as osbuild.NewManifestsFromBytes() cannot
	// unmarshal our manifests because of:
	// "unexpected source name: org.osbuild.containers-storage"
	return string(manifestJson)
}

func TestBuildContainerHandling(t *testing.T) {
	canRunIntegration(t)

	extraFiles := map[string]string{
		"/test.md": "Build container handling: base image",
	}
	imgTag := bootctest.NewFakeContainer(t, "bootc", extraFiles)

	extraFilesBuild := map[string]string{
		"/test.md": "Build container handling: build image",
	}
	buildImgTag := bootctest.NewFakeContainer(t, "build", extraFilesBuild)

	for _, withBuildContainer := range []bool{true, false} {
		t.Run(fmt.Sprintf("build-cnt:%v", withBuildContainer), func(t *testing.T) {
			require := require.New(t)

			bootcContainer, err := bootc.NewContainer(imgTag)
			require.NoError(err)
			bootcInfo, err := bootcContainer.ResolveInfo()
			require.NoError(err)

			distri, err := NewBootc("bootc", bootcInfo)
			require.NoError(err)
			if withBuildContainer {
				buildContainer, err := bootc.NewContainer(buildImgTag)
				require.NoError(err)
				buildInfo, err := buildContainer.ResolveInfo()
				require.NoError(err)
				err = distri.SetBuildContainer(buildInfo)
				require.NoError(err)
			}

			archi, err := distri.GetArch(arch.Current().String())
			require.NoError(err)
			imgType, err := archi.GetImageType("qcow2")
			assert.NoError(t, err)

			manifestJson := genManifest(t, imgType)
			pipelineNames, err := manifesttest.PipelineNamesFrom([]byte(manifestJson))
			require.NoError(err)
			buildStages, err := manifesttest.StagesForPipeline([]byte(manifestJson), "build")
			require.NoError(err)
			// the bootc container is always pulled
			assert.Contains(t, manifestJson, imgTag)
			if withBuildContainer {
				assert.Contains(t, manifestJson, buildImgTag)
				// validate that the usr/lib/bootc/install/ dir is copied
				assert.Contains(t, manifestJson, "usr/lib/bootc/install/")
				assert.Contains(t, buildStages, "org.osbuild.copy")
				// validate that we have a "target" pipeline for raw content
				assert.Contains(t, pipelineNames, "target")
			} else {
				assert.NotContains(t, manifestJson, buildImgTag)
				assert.NotContains(t, manifestJson, "usr/lib/bootc/install/")
				assert.NotContains(t, buildStages, "org.osbuild.copy")
				assert.NotContains(t, pipelineNames, "target")
			}
		})
	}
}

func TestIntegratedBuildDiskYAML(t *testing.T) {
	canRunIntegration(t)

	diskYAML := `
partition_table:
  type: gpt
  partitions:
    - size: 100_000_000
      payload_type: raw
      payload:
        source_path: /lib/modules/6.17/aboot.img
    - size: 10_000_000_000
      payload_type: filesystem
      payload:
        type: ext4
        mountpoint: /
`
	extraFiles := map[string]string{
		"/usr/lib/bootc-image-builder/disk.yaml": diskYAML,
		"/lib/modules/6.17/aboot.img":            "fake aboot.img content",
	}

	imgTag := bootctest.NewFakeContainer(t, "bootc", extraFiles)

	extraFilesBuild := map[string]string{
		"/test.md": "Integrated build disk YAML",
	}
	buildImgTag := bootctest.NewFakeContainer(t, "build", extraFilesBuild)

	for _, withBuildContainer := range []bool{true, false} {
		t.Run(fmt.Sprintf("build-cnt:%v", withBuildContainer), func(t *testing.T) {
			require := require.New(t)

			bootcContainer, err := bootc.NewContainer(imgTag)
			require.NoError(err)
			bootcInfo, err := bootcContainer.ResolveInfo()
			require.NoError(err)
			distri, err := NewBootc("bootc", bootcInfo)
			require.NoError(err)
			if withBuildContainer {
				buildContainer, err := bootc.NewContainer(buildImgTag)
				require.NoError(err)
				buildInfo, err := buildContainer.ResolveInfo()
				require.NoError(err)
				err = distri.SetBuildContainer(buildInfo)
				require.NoError(err)
			}

			archi, err := distri.GetArch(arch.Current().String())
			require.NoError(err)
			imgType, err := archi.GetImageType("qcow2")
			assert.NoError(t, err)

			manifestJson := genManifest(t, imgType)
			mani, err := manifesttest.NewManifestFromBytes([]byte(manifestJson))
			require.NoError(err)
			var stage *manifesttest.Stage
			var refPipeline string
			// The binary file comes from the target bootc
			// container. We mount the target as the build env
			// by default but when using a custom build container
			// we setup a special "target" pipeline that points
			// to the real bootc container. Ensure this is honored.
			if withBuildContainer {
				assert.Equal(t, []string{"target", "build", "image", "qcow2"}, mani.PipelineNames()[:4])

				stage = mani.Pipelines[2].Stage("org.osbuild.write-device")
				assert.NotNil(t, stage)
				refPipeline = "name:target"
			} else {
				stage = mani.Pipelines[1].Stage("org.osbuild.write-device")
				assert.NotNil(t, stage)
				assert.Equal(t, []string{"build", "image", "qcow2"}, mani.PipelineNames()[:3])
				refPipeline = "name:build"
			}
			// check write device stage options
			var opts osbuild.WriteDeviceStageOptions
			err = json.Unmarshal(stage.Options, &opts)
			require.NoError(err)
			assert.Equal(t, osbuild.WriteDeviceStageOptions{From: "input://tree/lib/modules/6.17/aboot.img"}, opts)
			// check write device stage inputs
			var inputs osbuild.PipelineTreeInputs
			err = json.Unmarshal(stage.Inputs, &inputs)
			require.NoError(err)
			expected := osbuild.PipelineTreeInputs{
				"tree": *osbuild.NewTreeInput(refPipeline),
			}
			assert.Equal(t, expected, inputs)
		})
	}
}
