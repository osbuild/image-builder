package generic_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
	"github.com/osbuild/images/pkg/distro/generic"
)

var rhel9_FamilyDistros = []rhelFamilyDistro{
	{
		name:   "rhel-94",
		distro: generic.DistroFactory("rhel-94"),
	},
}

func TestRhel9_FilenameFromType(t *testing.T) {
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
			name: "ec2",
			args: args{"ec2"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "ec2-ha",
			args: args{"ec2-ha"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "ec2-sap",
			args: args{"ec2-sap"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
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
			name: "openstack",
			args: args{"openstack"},
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
			name: "azure-rhui",
			args: args{"azure-rhui"},
			want: wantResult{
				filename: "disk.vhd.xz",
				mimeType: "application/xz",
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
			name: "image-installer",
			args: args{"image-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{
			name: "edge-commit",
			args: args{"edge-commit"},
			want: wantResult{
				filename: "commit.tar",
				mimeType: "application/x-tar",
			},
		},
		// Alias
		{
			name: "rhel-edge-commit",
			args: args{"rhel-edge-commit"},
			want: wantResult{
				filename: "commit.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "edge-container",
			args: args{"edge-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		// Alias
		{
			name: "rhel-edge-container",
			args: args{"rhel-edge-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "edge-installer",
			args: args{"edge-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		// Alias
		{
			name: "rhel-edge-installer",
			args: args{"rhel-edge-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{
			name: "gce",
			args: args{"gce"},
			want: wantResult{
				filename: "image.tar.gz",
				mimeType: "application/gzip",
			},
		},
		{
			name: "edge-ami",
			args: args{"edge-ami"},
			want: wantResult{
				filename: "image.raw",
				mimeType: "application/octet-stream",
			},
		},
		{
			name: "edge-vsphere",
			args: args{"edge-vsphere"},
			want: wantResult{
				filename: "image.vmdk",
				mimeType: "application/x-vmdk",
			},
		},
		{
			name: "invalid-output-type",
			args: args{"foobar"},
			want: wantResult{wantErr: true},
		},
		{
			name: "minimal-raw",
			args: args{"minimal-raw"},
			want: wantResult{
				filename: "disk.raw.xz",
				mimeType: "application/xz",
			},
		},
	}
	for _, dist := range rhel9_FamilyDistros {
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

func TestRhel9_ImageType_BuildPackages(t *testing.T) {
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
	for _, dist := range rhel9_FamilyDistros {
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

func TestRhel9_ImageType_Name(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
				"openstack",
				"vhd",
				"azure-rhui",
				"vmdk",
				"ova",
				"ami",
				"ec2",
				"ec2-ha",
				"ec2-sap",
				"edge-commit",
				"edge-container",
				"edge-installer",
				"gce",
				"tar",
				"image-installer",
				"minimal-raw",
				"network-installer",
				"pxe-tar-xz",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
				"openstack",
				"ami",
				"ec2",
				"edge-commit",
				"edge-container",
				"tar",
				"image-installer",
				"vhd",
				"azure-rhui",
				"minimal-raw",
				"network-installer",
				"pxe-tar-xz",
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
				"network-installer",
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhel9_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				if mapping.arch == arch.ARCH_S390X.String() && dist.name == "centos" {
					continue
				}
				arch, err := dist.distro.GetArch(mapping.arch)
				if assert.NoError(t, err) {
					for _, imgName := range mapping.imgNames {
						if imgName == "edge-commit" && dist.name == "centos" {
							continue
						}
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

func TestRhel9_ImageTypeAliases(t *testing.T) {
	type args struct {
		imageTypeAliases []string
	}
	type wantResult struct {
		imageTypeName string
	}
	tests := []struct {
		name string
		args args
		want wantResult
	}{
		{
			name: "edge-commit aliases",
			args: args{
				imageTypeAliases: []string{"rhel-edge-commit"},
			},
			want: wantResult{
				imageTypeName: "edge-commit",
			},
		},
		{
			name: "edge-container aliases",
			args: args{
				imageTypeAliases: []string{"rhel-edge-container"},
			},
			want: wantResult{
				imageTypeName: "edge-container",
			},
		},
		{
			name: "edge-installer aliases",
			args: args{
				imageTypeAliases: []string{"rhel-edge-installer"},
			},
			want: wantResult{
				imageTypeName: "edge-installer",
			},
		},
	}
	for _, dist := range rhel9_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
					for _, archName := range dist.ListArches() {
						t.Run(archName, func(t *testing.T) {
							arch, err := dist.GetArch(archName)
							require.Nilf(t, err,
								"failed to get architecture '%s', previously listed as supported for the distro '%s'",
								archName, dist.Name())
							// Test image type aliases only if the aliased image type is supported for the arch
							if _, err = arch.GetImageType(tt.want.imageTypeName); err != nil {
								t.Skipf("aliased image type '%s' is not supported for architecture '%s'",
									tt.want.imageTypeName, archName)
							}
							for _, alias := range tt.args.imageTypeAliases {
								t.Run(fmt.Sprintf("'%s' alias for image type '%s'", alias, tt.want.imageTypeName),
									func(t *testing.T) {
										gotImage, err := arch.GetImageType(alias)
										require.Nilf(t, err, "arch.GetImageType() for image type alias '%s' failed: %v",
											alias, err)
										assert.Equalf(t, tt.want.imageTypeName, gotImage.Name(),
											"got unexpected image type name for alias '%s'. got = %s, want = %s",
											alias, tt.want.imageTypeName, gotImage.Name())
									})
							}
						})
					}
				})
			}
		})
	}
}

func TestRhel9_Architecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
		verTypes map[string][]string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
				"openstack",
				"vhd",
				"azure-rhui",
				"azure-sap-rhui",
				"azure-sapapps-rhui",
				"vmdk",
				"ova",
				"ami",
				"ec2",
				"ec2-ha",
				"ec2-sap",
				"edge-commit",
				"edge-container",
				"edge-installer",
				"edge-raw-image",
				"edge-simplified-installer",
				"edge-ami",
				"edge-vsphere",
				"gce",
				"tar",
				"image-installer",
				"oci",
				"wsl",
				"minimal-raw",
				"network-installer",
				"vagrant-libvirt",
				"vagrant-virtualbox",
				"pxe-tar-xz",
			},
			verTypes: map[string][]string{
				"9.6": {
					"azure-cvm",
				},
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
				"openstack",
				"ami",
				"ec2",
				"edge-commit",
				"edge-container",
				"edge-installer",
				"edge-simplified-installer",
				"edge-raw-image",
				"edge-ami",
				"edge-vsphere",
				"tar",
				"image-installer",
				"vhd",
				"azure-rhui",
				"vagrant-libvirt",
				"wsl",
				"minimal-raw",
				"network-installer",
				"pxe-tar-xz",
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

	for _, dist := range rhel9_FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.distro.GetArch(mapping.arch)
				require.NoError(t, err)
				imageTypes := arch.ListImageTypes()

				var expectedImageTypes []string
				expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)
				if dist.name == "rhel" {
					expectedImageTypes = append(expectedImageTypes, mapping.verTypes[dist.distro.Releasever()]...)
				}

				require.ElementsMatch(t, expectedImageTypes, imageTypes)
			}
		})
	}
}

func TestRhel9_Rhel9_ListArches(t *testing.T) {
	arches := rhel9_FamilyDistros[0].distro.ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRhel9_Rhel9_GetArch(t *testing.T) {
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

	for _, dist := range rhel9_FamilyDistros {
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

func TestRhel9_Rhel9_Name(t *testing.T) {
	distro := rhel9_FamilyDistros[0].distro
	assert.Equal(t, "rhel-9.4", distro.Name())
}

func TestRhel9_Rhel9_ModulePlatformID(t *testing.T) {
	distro := rhel9_FamilyDistros[0].distro
	assert.Equal(t, "platform:el9", distro.ModulePlatformID())
}

func TestRhel9_Rhel9_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel9_FamilyDistros[0].distro)
}

func TestRhel9_Rhel9_OSTreeOptions(t *testing.T) {
	distro_test_common.TestDistro_OSTreeOptions(t, rhel9_FamilyDistros[0].distro)
}

func TestRhel9_DistroFactory(t *testing.T) {
	type testCase struct {
		strID    string
		expected distro.Distro
	}

	testCases := []testCase{
		{
			strID:    "rhel-9.6",
			expected: generic.DistroFactory("rhel-9.6"),
		},
		{
			strID:    "rhel-9.6.1",
			expected: nil,
		},
		{
			strID:    "rhel-9",
			expected: nil,
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
