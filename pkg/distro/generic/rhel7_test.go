package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/distro/distro_test_common"
	"github.com/osbuild/image-builder/pkg/distro/generic"
)

type rhelFamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhel7_FamilyDistros = []rhelFamilyDistro{
	{
		name:   "rhel-79",
		distro: generic.DistroFactory("rhel-7.9"),
	},
}

func TestRhel7FilenameFromType(t *testing.T) {
	type args struct {
		outputFormat string
	}
	type wantResult struct {
		filename string
		mimeType string
		wantErr  bool
	}
	tests := []struct {
		name string
		args args
		want wantResult
	}{
		{
			name: "qcow2",
			args: args{"qcow2"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "azure-rhui",
			args: args{"azure-rhui"},
			want: wantResult{
				filename: "disk.vhd.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "ec2",
			args: args{"ec2"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "invalid-output-type",
			args: args{"foobar"},
			want: wantResult{wantErr: true},
		},
	}
	for _, dist := range rhel7_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
					require.NotNil(t, dist)
					arch, err := dist.GetArch("x86_64")
					require.NoError(t, err)
					imgType, err := arch.GetImageType(tt.args.outputFormat)
					if tt.want.wantErr {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						require.NotNil(t, imgType)
						gotFilename := imgType.Filename()
						gotMIMEType := imgType.MIMEType()
						if gotFilename != tt.want.filename {
							t.Errorf("ImageType.Filename()  got = %v, want %v", gotFilename, tt.want.filename)
						}
						if gotMIMEType != tt.want.mimeType {
							t.Errorf("ImageType.MIMEType() got1 = %v, want %v", gotMIMEType, tt.want.mimeType)
						}
					}
				})
			}
		})
	}
}

func TestRhel7ImageType_BuildPackages(t *testing.T) {
	x8664BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"grub2-efi-x64",
		"grub2-pc",
		"policycoreutils",
		"shim-x64",
		"systemd",
		"tar",
		"qemu-img",
		"xz",
	}
	buildPackages := map[string][]string{
		"x86_64": x8664BuildPackages,
	}
	for _, dist := range rhel7_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			d := dist.distro
			for _, archLabel := range d.ListArches() {
				archStruct, err := d.GetArch(archLabel)
				if assert.NoErrorf(t, err, "d.GetArch(%v) returned err = %v; expected nil", archLabel, err) {
					continue
				}
				for _, itLabel := range archStruct.ListImageTypes() {
					itStruct, err := archStruct.GetImageType(itLabel)
					if assert.NoErrorf(t, err, "d.GetArch(%v) returned err = %v; expected nil", archLabel, err) {
						continue
					}
					manifest, _, err := itStruct.Manifest(&blueprint.Blueprint{}, distro.ImageOptions{}, nil, nil)
					assert.NoError(t, err)
					pkgSetChain, err := manifest.GetPackageSetChains()
					assert.NoError(t, err)
					buildPkgs := pkgSetChain["build"]
					assert.NotNil(t, buildPkgs)
					assert.Len(t, buildPkgs, 1)
					assert.ElementsMatch(t, buildPackages[archLabel], buildPkgs[0].Include)
				}
			}
		})
	}
}

func TestRhel7ImageType_Name(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"ec2",
				"qcow2",
				"azure-rhui",
			},
		},
	}

	for _, dist := range rhel7_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.distro.GetArch(mapping.arch)
				if assert.NoError(t, err) {
					for _, imgName := range mapping.imgNames {
						imgType, err := arch.GetImageType(imgName)
						if assert.NoError(t, err) {
							assert.Equalf(t, imgName, imgType.Name(), "arch: %s", mapping.arch)
						}
					}
				}
			}
		})
	}
}

func TestRhel7Architecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch                     string
		imgNames                 []string
		rhelAdditionalImageTypes []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"ec2",
				"qcow2",
				"azure-rhui",
			},
		},
	}

	for _, dist := range rhel7_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.distro.GetArch(mapping.arch)
				require.NoError(t, err)
				imageTypes := arch.ListImageTypes()

				var expectedImageTypes []string
				expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)
				if dist.name == "rhel" {
					expectedImageTypes = append(expectedImageTypes, mapping.rhelAdditionalImageTypes...)
				}

				require.ElementsMatch(t, expectedImageTypes, imageTypes)
			}
		})
	}
}

func TestRhel7Rhel7_ListArches(t *testing.T) {
	arches := rhel7_FamilyDistros[0].distro.ListArches()
	assert.Equal(t, []string{"x86_64"}, arches)
}

func TestRhel7Rhel7_GetArch(t *testing.T) {
	arches := []struct {
		name                  string
		errorExpected         bool
		errorExpectedInCentos bool
	}{
		{
			name: "x86_64",
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, dist := range rhel7_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, a := range arches {
				actualArch, err := dist.distro.GetArch(a.name)
				if a.errorExpected || (a.errorExpectedInCentos && dist.name == "centos") {
					assert.Nil(t, actualArch)
					assert.Error(t, err)
				} else {
					assert.Equal(t, a.name, actualArch.Name())
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestRhel7Rhel7_Name(t *testing.T) {
	distro := rhel7_FamilyDistros[0].distro
	assert.Equal(t, "rhel-7.9", distro.Name())
}

func TestRhel7Rhel7_ModulePlatformID(t *testing.T) {
	distro := rhel7_FamilyDistros[0].distro
	assert.Equal(t, "platform:el7", distro.ModulePlatformID())
}

func TestRhel7Rhel7_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel7_FamilyDistros[0].distro)
}

func TestRhel7DistroFactory(t *testing.T) {
	type testCase struct {
		strID    string
		expected distro.Distro
	}

	testCases := []testCase{
		{
			strID:    "rhel-7",
			expected: nil,
		},
		{
			// the latest RHEL-7 is 7.9 and there won't be any newer one
			strID:    "rhel-7.10",
			expected: nil,
		},
		{
			// this is intentionally not supported for el7
			strID:    "rhel-79",
			expected: nil,
		},
		{
			strID:    "rhel-7.9",
			expected: generic.DistroFactory("rhel-7.9"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.strID, func(t *testing.T) {
			d := generic.DistroFactory(tc.strID)
			if tc.expected == nil {
				assert.Nil(t, d)
			} else {
				assert.NotNil(t, d)
				assert.Equal(t, tc.expected.Name(), d.Name())
			}
		})
	}
}
