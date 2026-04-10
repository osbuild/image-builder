package generic_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
	"github.com/osbuild/images/pkg/distro/generic"
)

var fedoraFamilyDistros = []distro.Distro{
	generic.DistroFactory("fedora-40"),
	generic.DistroFactory("fedora-41"),
	generic.DistroFactory("fedora-42"),
}

func TestFedoraFilenameFromType(t *testing.T) {
	type args struct {
		outputFormat string
	}
	type wantResult struct {
		filename string
		mimeType string
		wantErr  bool
	}
	type testCfg struct {
		name string
		args args
		want wantResult
	}
	tests := []testCfg{
		{
			name: "generic-ami",
			args: args{"generic-ami"},
			want: wantResult{
				filename: "image.raw",
				mimeType: "application/octet-stream",
			},
		},
		{
			name: "generic-qcow2",
			args: args{"generic-qcow2"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "generic-vagrant-libvirt",
			args: args{"generic-vagrant-libvirt"},
			want: wantResult{
				filename: "vagrant-libvirt.box",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "generic-vagrant-virtualbox",
			args: args{"generic-vagrant-virtualbox"},
			want: wantResult{
				filename: "vagrant-virtualbox.box",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "generic-openstack",
			args: args{"generic-openstack"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "generic-vhd",
			args: args{"generic-vhd"},
			want: wantResult{
				filename: "disk.vhd",
				mimeType: "application/x-vhd",
			},
		},
		{
			name: "generic-vmdk",
			args: args{"generic-vmdk"},
			want: wantResult{
				filename: "disk.vmdk",
				mimeType: "application/x-vmdk",
			},
		},
		{
			name: "generic-ova",
			args: args{"generic-ova"},
			want: wantResult{
				filename: "image.ova",
				mimeType: "application/ovf",
			},
		},
		{
			name: "generic-container",
			args: args{"generic-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "generic-wsl",
			args: args{"generic-wsl"},
			want: wantResult{
				filename: "image.wsl",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "iot-commit",
			args: args{"iot-commit"},
			want: wantResult{
				filename: "commit.tar",
				mimeType: "application/x-tar",
			},
		},
		{ // Alias
			name: "fedora-iot-commit",
			args: args{"fedora-iot-commit"},
			want: wantResult{
				filename: "commit.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "iot-container",
			args: args{"iot-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		{ // Alias
			name: "fedora-iot-container",
			args: args{"fedora-iot-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "iot-installer",
			args: args{"iot-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{ // Alias
			name: "fedora-iot-installer",
			args: args{"fedora-iot-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{
			name: "live-installer",
			args: args{"live-installer"},
			want: wantResult{
				filename: "live-installer.iso",
				mimeType: "application/x-iso9660-image",
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
		{ // Alias
			name: "fedora-image-installer",
			args: args{"fedora-image-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{
			name: "invalid-output-type",
			args: args{"foobar"},
			want: wantResult{wantErr: true},
		},
		{
			name: "minimal-raw-xz",
			args: args{"minimal-raw-xz"},
			want: wantResult{
				filename: "disk.raw.xz",
				mimeType: "application/xz",
			},
		},
	}
	verTypes := map[string][]testCfg{
		"40": {
			{
				name: "iot-bootable-container",
				args: args{"iot-bootable-container"},
				want: wantResult{
					filename: "iot-bootable-container.tar",
					mimeType: "application/x-tar",
				},
			},
			{
				name: "iot-simplified-installer",
				args: args{"iot-simplified-installer"},
				want: wantResult{
					filename: "simplified-installer.iso",
					mimeType: "application/x-iso9660-image",
				},
			},
		},
		"41": {
			{
				name: "iot-bootable-container",
				args: args{"iot-bootable-container"},
				want: wantResult{
					filename: "iot-bootable-container.tar",
					mimeType: "application/x-tar",
				},
			},
			{
				name: "iot-simplified-installer",
				args: args{"iot-simplified-installer"},
				want: wantResult{
					filename: "simplified-installer.iso",
					mimeType: "application/x-iso9660-image",
				},
			},
		},
	}
	for _, dist := range fedoraFamilyDistros {
		t.Run(dist.Name(), func(t *testing.T) {
			allTests := append(tests, verTypes[dist.Releasever()]...)
			for _, tt := range allTests {
				t.Run(tt.name, func(t *testing.T) {
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

func TestFedoraImageType_BuildPackages(t *testing.T) {
	x8664BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"selinux-policy-targeted",
		"systemd",
		"tar",
		"xz",
		"grub2-pc",
	}
	aarch64BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"selinux-policy-targeted",
		"systemd",
		"tar",
		"xz",
	}
	buildPackages := map[string][]string{
		"x86_64":  x8664BuildPackages,
		"aarch64": aarch64BuildPackages,
	}
	for _, d := range fedoraFamilyDistros {
		t.Run(d.Name(), func(t *testing.T) {
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

func TestFedoraImageType_Name(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
		verTypes map[string][]string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"generic-ami",
				"minimal-installer",
				"iot-commit",
				"iot-container",
				"iot-installer",
				"iot-qcow2",
				"iot-raw-xz",
				"workstation-live-installer",
				"minimal-raw-xz",
				"minimal-raw-zst",
				"generic-oci",
				"generic-openstack",
				"generic-ova",
				"generic-qcow2",
				"generic-vhd",
				"generic-vmdk",
				"generic-vagrant-libvirt",
				"generic-vagrant-virtualbox",
				"generic-wsl",
				"server-qcow2",
				"kinoite-installer",
				"kinoite-qcow2",
				"silverblue-installer",
				"silverblue-qcow2",
				"sway-atomic-installer",
				"budgie-atomic-installer",
				"cosmic-atomic-installer",
			},
			verTypes: map[string][]string{
				"40": {
					"iot-bootable-container",
					"iot-simplified-installer",
				},
				"41": {
					"iot-bootable-container",
					"iot-simplified-installer",
				},
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"generic-ami",
				"minimal-installer",
				"iot-commit",
				"iot-container",
				"iot-installer",
				"iot-qcow2",
				"iot-raw-xz",
				"minimal-raw-xz",
				"minimal-raw-zst",
				"generic-oci",
				"generic-openstack",
				"generic-qcow2",
				"generic-vagrant-libvirt",
				"server-qcow2",
				"kinoite-installer",
				"kinoite-qcow2",
				"silverblue-installer",
				"silverblue-qcow2",
				"sway-atomic-installer",
				"budgie-atomic-installer",
				"cosmic-atomic-installer",
			},
			verTypes: map[string][]string{
				"40": {
					"iot-bootable-container",
					"iot-simplified-installer",
				},
				"41": {
					"iot-bootable-container",
					"iot-simplified-installer",
				},
			},
		},
	}

	for _, dist := range fedoraFamilyDistros {
		t.Run(dist.Name(), func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.GetArch(mapping.arch)
				if assert.NoError(t, err) {
					imgTypes := append(mapping.imgNames, mapping.verTypes[dist.Releasever()]...)
					for _, imgName := range imgTypes {
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

func TestFedoraImageTypeAliases(t *testing.T) {
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
			name: "iot-commit aliases",
			args: args{
				imageTypeAliases: []string{"fedora-iot-commit"},
			},
			want: wantResult{
				imageTypeName: "iot-commit",
			},
		},
		{
			name: "iot-container aliases",
			args: args{
				imageTypeAliases: []string{"fedora-iot-container"},
			},
			want: wantResult{
				imageTypeName: "iot-container",
			},
		},
		{
			name: "iot-installer aliases",
			args: args{
				imageTypeAliases: []string{"fedora-iot-installer"},
			},
			want: wantResult{
				imageTypeName: "iot-installer",
			},
		},
	}
	for _, dist := range fedoraFamilyDistros {
		t.Run(dist.Name(), func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
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

func TestFedoraArchitecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
		verTypes map[string][]string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"generic-ami",
				"generic-container",
				"minimal-installer",
				"iot-commit",
				"iot-container",
				"iot-installer",
				"iot-qcow2",
				"iot-raw-xz",
				"workstation-live-installer",
				"minimal-raw-xz",
				"minimal-raw-zst",
				"generic-oci",
				"generic-openstack",
				"generic-ova",
				"generic-qcow2",
				"generic-vhd",
				"generic-vmdk",
				"generic-vagrant-libvirt",
				"generic-vagrant-virtualbox",
				"generic-wsl",
				"server-qcow2",
				"cloud-azure",
				"cloud-ec2",
				"cloud-gce",
				"cloud-qcow2",
				"iot-bootable-container",
				"iot-simplified-installer",
				"everything-network-installer",
				"server-network-installer",
				"pxe-tar-xz",
				"kinoite-installer",
				"kinoite-qcow2",
				"silverblue-installer",
				"silverblue-qcow2",
				"sway-atomic-installer",
				"budgie-atomic-installer",
				"cosmic-atomic-installer",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"generic-ami",
				"generic-container",
				"minimal-installer",
				"iot-commit",
				"iot-container",
				"iot-installer",
				"iot-qcow2",
				"iot-raw-xz",
				"workstation-live-installer",
				"minimal-raw-xz",
				"minimal-raw-zst",
				"generic-oci",
				"generic-openstack",
				"generic-qcow2",
				"generic-vagrant-libvirt",
				"server-qcow2",
				"cloud-azure",
				"cloud-ec2",
				"cloud-gce",
				"cloud-qcow2",
				"iot-bootable-container",
				"iot-simplified-installer",
				"everything-network-installer",
				"server-network-installer",
				"pxe-tar-xz",
				"kinoite-installer",
				"kinoite-qcow2",
				"silverblue-installer",
				"silverblue-qcow2",
				"sway-atomic-installer",
				"budgie-atomic-installer",
				"cosmic-atomic-installer",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"generic-container",
				"generic-qcow2",
				"server-qcow2",
				"cloud-qcow2",
				"iot-bootable-container",
				"everything-network-installer",
				"server-network-installer",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"generic-container",
				"generic-qcow2",
				"everything-network-installer",
				"server-network-installer",
				"server-qcow2",
				"cloud-qcow2",
				"iot-bootable-container",
			},
		},
		{
			arch: "riscv64",
			imgNames: []string{
				"generic-container",
				"minimal-raw-xz",
				"minimal-raw-zst",
			},
		},
	}

	for _, dist := range fedoraFamilyDistros {
		t.Run(dist.Name(), func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.GetArch(mapping.arch)
				require.NoError(t, err)
				imageTypes := arch.ListImageTypes()

				var expectedImageTypes []string
				expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)
				expectedImageTypes = append(expectedImageTypes, mapping.verTypes[dist.Releasever()]...)

				slices.Sort(expectedImageTypes)
				slices.Sort(imageTypes)
				require.Equal(t, expectedImageTypes, imageTypes, "extra images for arch %v", arch.Name())
			}
		})
	}
}

func TestFedoraFedora_ListArches(t *testing.T) {
	for _, fedoraDistro := range fedoraFamilyDistros {
		t.Run(fedoraDistro.Name(), func(t *testing.T) {
			arches := fedoraDistro.ListArches()
			assert.Equal(t, []string{"aarch64", "ppc64le", "riscv64", "s390x", "x86_64"}, arches)
		})
	}
}

func TestFedoraFedora38_GetArch(t *testing.T) {
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
			name: "s390x",
		},
		{
			name: "ppc64le",
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, dist := range fedoraFamilyDistros {
		t.Run(dist.Name(), func(t *testing.T) {
			for _, a := range arches {
				actualArch, err := dist.GetArch(a.name)
				if a.errorExpected {
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

func TestFedoraFedora_KernelOption(t *testing.T) {
	for _, fedoraDistro := range fedoraFamilyDistros {
		t.Run(fedoraDistro.Name(), func(t *testing.T) {
			distro_test_common.TestDistro_KernelOption(t, fedoraDistro)
		})
	}
}

func TestFedoraFedora_OSTreeOptions(t *testing.T) {
	for _, fedoraDistro := range fedoraFamilyDistros {
		t.Run(fedoraDistro.Name(), func(t *testing.T) {
			distro_test_common.TestDistro_OSTreeOptions(t, fedoraDistro)
		})
	}
}

func TestFedoraDistroFactory(t *testing.T) {
	type testCase struct {
		strID    string
		expected distro.Distro
	}

	testCases := []testCase{
		{
			strID:    "fedora-40",
			expected: generic.DistroFactory("fedora-40"),
		},
		{
			strID:    "fedora-40.1",
			expected: nil,
		},
		{
			strID:    "fedora",
			expected: nil,
		},
		{
			strID:    "fedora-043",
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

func TestFedoraESP(t *testing.T) {
	distro_test_common.TestESP(t, fedoraFamilyDistros, func(it distro.ImageType) (*disk.PartitionTable, error) {
		return generic.GetPartitionTable(it)
	})
}

func TestFedoraDistroBootstrapRef(t *testing.T) {
	for _, fedoraDistro := range fedoraFamilyDistros {
		for _, archName := range fedoraDistro.ListArches() {
			distroArch, err := fedoraDistro.GetArch(archName)
			require.NoError(t, err)
			for _, imgTypeName := range distroArch.ListImageTypes() {
				imgType, err := distroArch.GetImageType(imgTypeName)
				require.NoError(t, err)
				if distroArch.Name() == "riscv64" {
					bootstrapRef, err := imgType.Arch().Distro().BootstrapContainer(distroArch.Name())
					require.NoError(t, err)
					require.Equal(t, "ghcr.io/mvo5/fedora-buildroot:"+fedoraDistro.OsVersion(), bootstrapRef)
				} else {
					bootstrapRef, err := imgType.Arch().Distro().BootstrapContainer(distroArch.Name())
					require.NoError(t, err)
					require.Equal(t, "registry.fedoraproject.org/fedora-toolbox:"+fedoraDistro.OsVersion(), bootstrapRef)
				}
			}
		}
	}
}
