package generic_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
	"github.com/osbuild/images/pkg/distro/generic"
)

type rhel10FamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhel10FamilyDistros = []rhel10FamilyDistro{
	{
		name:   "rhel-10.0",
		distro: generic.DistroFactory("rhel-10.0"),
	},
}

func TestRH10FilenameFromType(t *testing.T) {
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
			name: "ami",
			args: args{"ami"},
			want: wantResult{
				filename: "image.raw",
				mimeType: "application/octet-stream",
			},
		},
		{
			name: "qcow2",
			args: args{"qcow2"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "vhd",
			args: args{"vhd"},
			want: wantResult{
				filename: "disk.vhd",
				mimeType: "application/x-vhd",
			},
		},
		{
			name: "vmdk",
			args: args{"vmdk"},
			want: wantResult{
				filename: "disk.vmdk",
				mimeType: "application/x-vmdk",
			},
		},
		{
			name: "ova",
			args: args{"ova"},
			want: wantResult{
				filename: "image.ova",
				mimeType: "application/ovf",
			},
		},
		{
			name: "tar",
			args: args{"tar"},
			want: wantResult{
				filename: "root.tar.xz",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "invalid-output-type",
			args: args{"foobar"},
			want: wantResult{wantErr: true},
		},
	}
	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
					arch, _ := dist.GetArch("x86_64")
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

func TestRH10ImageType_BuildPackages(t *testing.T) {
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
	aarch64BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
		"xz",
	}
	buildPackages := map[string][]string{
		"x86_64":  x8664BuildPackages,
		"aarch64": aarch64BuildPackages,
	}
	for _, dist := range rhel10FamilyDistros {
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

func TestRH10ImageType_Name(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
				"vhd",
				"vmdk",
				"ova",
				"ami",
				"tar",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
				"ami",
				"tar",
				"vhd",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				if mapping.arch == arch.ARCH_S390X.String() && dist.name == "centos" {
					continue
				}
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

func TestRH10Architecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch                     string
		imgNames                 []string
		rhelAdditionalImageTypes []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
				"oci",
				"vhd",
				"vmdk",
				"ova",
				"ami",
				"tar",
				"wsl",
				"gce",
				"image-installer",
				"network-installer",
				"azure-cvm",
				"azure-rhui",
				"azure-sap-rhui",
				"azure-sapapps-rhui",
				"ec2",
				"ec2-cvm",
				"ec2-ha",
				"ec2-sap",
				"vagrant-libvirt",
				"vagrant-virtualbox",
				"pxe-tar-xz",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"ami",
				"azure-rhui",
				"ec2",
				"image-installer",
				"network-installer",
				"pxe-tar-xz",
				"qcow2",
				"tar",
				"vagrant-libvirt",
				"vhd",
				"wsl",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"network-installer",
				"qcow2",
				"tar",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"network-installer",
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhel10FamilyDistros {
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

				slices.Sort(expectedImageTypes)
				slices.Sort(imageTypes)
				require.Equal(t, expectedImageTypes, imageTypes)
			}
		})
	}
}

func TestRH10Rhel10_ListArches(t *testing.T) {
	arches := rhel10FamilyDistros[0].distro.ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRH10Rhel10_GetArch(t *testing.T) {
	arches := []struct {
		name                  string
		errorExpected         bool
		errorExpectedInCentos bool
	}{
		{
			name: "x86_64",
		},
		{
			name: "aarch64",
		},
		{
			name: "ppc64le",
		},
		{
			name: "s390x",
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, dist := range rhel10FamilyDistros {
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

func TestRH10Rhel10_Name(t *testing.T) {
	distro := rhel10FamilyDistros[0].distro
	assert.Equal(t, "rhel-10.0", distro.Name())
}

func TestRH10Rhel10_ModulePlatformID(t *testing.T) {
	distro := rhel10FamilyDistros[0].distro
	assert.Equal(t, "platform:el10", distro.ModulePlatformID())
}

func TestRH10Rhel10_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel10FamilyDistros[0].distro)
}

func TestRH10Rhel10_KernelOption_NoIfnames(t *testing.T) {
	for _, distroName := range []string{"rhel-10.0", "centos-10"} {
		distro := generic.DistroFactory(distroName)
		for _, archName := range distro.ListArches() {
			arch, err := distro.GetArch(archName)
			assert.NoError(t, err)
			for _, imgTypeName := range arch.ListImageTypes() {
				imgType, err := arch.GetImageType(imgTypeName)
				assert.NoError(t, err)
				imgCfg := imgType.(*generic.ImageType).GetDefaultImageConfig()
				if imgCfg != nil {
					assert.NotContains(t, imgCfg.KernelOptions, "net.ifnames=0", "type %s contains unwanted net.ifnames=0", imgType.Name())
				}
			}
		}
	}
}
