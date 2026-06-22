package image_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/v73/pkg/disk"
	"github.com/osbuild/image-builder/v73/pkg/image"
	"github.com/osbuild/image-builder/v73/pkg/manifest"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
	"github.com/osbuild/image-builder/v73/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootType(t *testing.T) {
	ibl := image.ISOBootloaders{
		InstallerCustomizations: &manifest.InstallerCustomizations{Product: "Fedora", OSVersion: "44"},
		ISOCustomizations:       &manifest.ISOCustomizations{Label: "Fedora-44-test"},
	}

	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	runner := &runner.Fedora{Version: 44}
	pt := disk.EFIBootPartitionTable(rng)

	type results struct {
		stages []string
		paths  []string
	}

	type testCase struct {
		bootType manifest.ISOBootType
		custom   []manifest.ISOGrub2MenuEntry
		expected []results
	}

	var noCustomMenus []manifest.ISOGrub2MenuEntry
	customMenus := []manifest.ISOGrub2MenuEntry{
		manifest.ISOGrub2MenuEntry{
			Name: "Boot Linux", Linux: "vmlinuz", Initrd: "initrd.img",
		},
		manifest.ISOGrub2MenuEntry{
			Name: "Boot Linux Debug", Linux: "vmlinuz debug", Initrd: "initrd.img",
		},
	}

	// Check the stages and files for each bootloader type
	tests := []testCase{
		testCase{manifest.Grub2UEFIOnlyISOBoot, noCustomMenus, []results{{
			stages: []string{
				"org.osbuild.truncate",
				"org.osbuild.mkfs.fat",
				"org.osbuild.copy",
				"org.osbuild.copy"},
			paths: []string{}},
		}},
		testCase{manifest.SyslinuxISOBoot, noCustomMenus, []results{
			{stages: []string{"org.osbuild.isolinux"}, paths: []string{}}, {
				stages: []string{
					"org.osbuild.truncate",
					"org.osbuild.mkfs.fat",
					"org.osbuild.copy",
					"org.osbuild.copy"},
				paths: []string{}},
		}},
		testCase{manifest.Grub2ISOBoot, noCustomMenus, []results{{
			stages: []string{
				"org.osbuild.grub2.iso.legacy",
				"org.osbuild.grub2.inst"},
			paths: []string{}}, {
			stages: []string{
				"org.osbuild.truncate",
				"org.osbuild.mkfs.fat",
				"org.osbuild.copy",
				"org.osbuild.copy"},
			paths: []string{}},
		}},
		testCase{manifest.Grub2ISOBoot, customMenus, []results{{
			stages: []string{
				"org.osbuild.grub2.iso.legacy",
				"org.osbuild.grub2.inst"},
			paths: []string{}}, {
			stages: []string{
				"org.osbuild.truncate",
				"org.osbuild.mkfs.fat",
				"org.osbuild.copy",
				"org.osbuild.copy"},
			paths: []string{}},
		}},
		testCase{manifest.Grub2PPCISOBoot, noCustomMenus, []results{{
			stages: []string{
				"org.osbuild.grub2.iso.legacy",
				"org.osbuild.mkdir",
				"org.osbuild.copy"},
			paths: []string{"/ppc/bootinfo.txt"}},
		}},
		testCase{manifest.Grub2PPCISOBoot, customMenus, []results{{
			stages: []string{
				"org.osbuild.grub2.iso.legacy",
				"org.osbuild.mkdir",
				"org.osbuild.copy"},
			paths: []string{"/ppc/bootinfo.txt"}},
		}},
		testCase{manifest.S390ISOBoot, noCustomMenus, []results{{
			stages: []string{
				"org.osbuild.copy",
				"org.osbuild.copy",
				"org.osbuild.copy",
				"org.osbuild.copy",
				"org.osbuild.copy",
				"org.osbuild.createaddrsize",
				"org.osbuild.mks390image"},
			paths: []string{
				"/images/redhat.exec",
				"/images/generic.prm",
				"/images/genericdvd.prm",
				"/images/generic.ins",
				"/images/cdboot.prm"}},
		}},
	}

	for _, tc := range tests {
		mf := manifest.New()
		buildPipeline := image.AddBuildBootstrapPipelines(&mf, runner, nil, nil)
		ibl.ISOCustomizations.BootType = tc.bootType
		ibl.Custom = tc.custom
		bootloaders := ibl.Bootloaders(buildPipeline, testPlatform, []string{})
		require.Len(t, bootloaders, len(tc.expected))

		for i := range tc.expected {
			stages, files, err := bootloaders[i].GetISOBootStages("iso-bootloaders", pt)
			require.NoError(t, err)
			assert.Len(t, stages, len(tc.expected[i].stages), fmt.Sprintf("%v", stageNames(stages)))
			assert.Equal(t, stageNames(stages), tc.expected[i].stages)
			assert.Len(t, files, len(tc.expected[i].paths), fmt.Sprintf("%v", filePaths(files)))
			assert.Equal(t, filePaths(files), tc.expected[i].paths)

			// Check grub2 custom menus when set
			grub2Stage := findStage("org.osbuild.grub2.iso.legacy", stages)
			if grub2Stage == nil {
				continue
			}
			options := grub2Stage.Options.(*osbuild.Grub2ISOLegacyStageOptions)
			require.NotNil(t, options)
			checkCustomMenus(t, tc.custom, options.Custom)
		}
	}
}

func checkCustomMenus(t *testing.T,
	expected []manifest.ISOGrub2MenuEntry,
	menus []osbuild.Grub2ISOLegacyCustomEntryOptions) {
	require.Equal(t, len(expected), len(menus))
	for i := range expected {
		require.Equal(t, expected[i].Name, menus[i].Name)
		require.Equal(t, expected[i].Linux, menus[i].Linux)
		require.Equal(t, expected[i].Initrd, menus[i].Initrd)
	}
}

func stageNames(stages []*osbuild.Stage) []string {
	names := []string{}
	for _, s := range stages {
		names = append(names, s.Type)
	}
	return names
}

func filePaths(files []*fsnode.File) []string {
	paths := []string{}
	for _, f := range files {
		paths = append(paths, f.Path())
	}
	return paths
}

func findStage(name string, stages []*osbuild.Stage) *osbuild.Stage {
	for _, s := range stages {
		if s.Type == name {
			return s
		}
	}
	return nil
}
