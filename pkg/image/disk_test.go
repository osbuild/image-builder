package image_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/image"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestDiskImageSysextStandalone(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		name := "reuse OS"
		if standalone {
			name = "standalone"
		}
		t.Run(name, func(t *testing.T) {
			img := image.NewDiskImage(&platform.Data{Arch: arch.ARCH_X86_64, ImageFormat: platform.FORMAT_RAW}, "disk.raw")
			img.DiskCustomizations = manifest.NewDiskCustomizations()
			img.PartitionTable = &disk.PartitionTable{
				Type: disk.PT_GPT, Size: 1024 * 1024 * 1024,
				Partitions: []disk.Partition{{
					Start: 1024 * 1024, Size: 1023 * 1024 * 1024,
					Payload: &disk.Filesystem{Type: "ext4", Mountpoint: "/", UUID: "11111111-1111-1111-1111-111111111111"},
				}},
			}
			img.OSCustomizations.BasePackages = []string{"kernel"}
			img.OSCustomizations.BaseRPMOptions = osbuild.RPMStageOptions{GPGKeysFromTree: []string{"/base-key"}}
			img.Sysexts = []image.SysextConfig{{
				Name: "test", Format: "erofs", Standalone: standalone,
				PackageSet: rpmmd.PackageSet{Include: []string{"extension"}},
			}}
			mf := manifest.New()
			_, err := img.InstantiateManifest(&mf, nil, &runner.Fedora{}, nil)
			require.NoError(t, err)
			chains, err := mf.GetPackageSetChains()
			require.NoError(t, err)
			if standalone {
				require.Len(t, chains["sysext-test-tree"], 1)
				assert.Equal(t, []string{"extension"}, chains["sysext-test-tree"][0].Include)
			} else {
				require.Len(t, chains["sysext-test-tree"], len(chains["os"])+1)
				assert.Equal(t, chains["os"], chains["sysext-test-tree"][:len(chains["os"])])
				assert.Equal(t, []string{"extension"}, chains["sysext-test-tree"][len(chains["os"])].Include)
			}

			depsolved := mockPackageSets()
			installed := depsolved["os"].Transactions[0][0]
			installed.Files = []string{"/base-key"}
			depsolved["os"] = depsolvednf.DepsolveResult{Transactions: depsolvednf.TransactionList{{installed}}}
			added := installed
			added.Name = "extension"
			added.Checksum.Value = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			added.Files = nil
			transactions := depsolvednf.TransactionList{{installed}, {added}}
			if standalone {
				transactions = depsolvednf.TransactionList{{installed, added}}
			}
			depsolved["sysext-test-tree"] = depsolvednf.DepsolveResult{Transactions: transactions}
			data, err := mf.Serialize(depsolved, nil, nil, nil, nil)
			require.NoError(t, err)
			pipeline := findPipelineFromOsbuildManifest(t, data, "sysext-test-tree")
			require.NotNil(t, pipeline)
			var want []*osbuild.Stage
			packages := rpmmd.PackageList{added}
			opts := &osbuild.RPMStageOptions{}
			if standalone {
				packages = rpmmd.PackageList{installed, added}
				opts.GPGKeysFromTree = []string{"/base-key"}
			} else {
				want = append(want, osbuild.NewCopyStageSimple(
					&osbuild.CopyStageOptions{Paths: []osbuild.CopyStagePath{{From: "input://tree/", To: "tree:///"}}},
					osbuild.NewPipelineTreeInputs("tree", "os"),
				))
			}
			want = append(want, osbuild.NewRPMStage(opts, osbuild.NewRpmStageSourceFilesInputs(packages)))
			wantJSON, err := json.Marshal(want)
			require.NoError(t, err)
			gotJSON, err := json.Marshal(pipeline["stages"])
			require.NoError(t, err)
			assert.JSONEq(t, string(wantJSON), string(gotJSON))
		})
	}
}
