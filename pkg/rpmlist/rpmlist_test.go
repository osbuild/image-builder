package rpmlist

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodePackages(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		b, err := EncodePackages(nil)
		require.NoError(t, err)
		assert.JSONEq(t, "[]", b.String())
	})

	t.Run("one package", func(t *testing.T) {
		pkgs := rpmmd.PackageList{
			{
				Name: "bash", Version: "5.2", Release: "1.fc43", Epoch: 0, Arch: "x86_64",
				DownloadSize:   12345,
				BuildTime:      time.Unix(1700000000, 0).UTC(),
				Checksum:       rpmmd.Checksum{Type: "sha256", Value: "a250e0b22938f0630f64b0b534141ba0"},
				HeaderChecksum: rpmmd.Checksum{Type: "sha256", Value: "478549d2e9fde87833938316d695a36d"},
			},
		}
		b, err := EncodePackages(pkgs)
		require.NoError(t, err)

		var decoded []kojiRpmListEntry
		require.NoError(t, json.Unmarshal(b.Bytes(), &decoded))
		require.Len(t, decoded, 1)
		assert.Equal(t, "bash", decoded[0].Name)
		assert.Equal(t, "5.2", decoded[0].Version)
		assert.Equal(t, "1.fc43", decoded[0].Release)
		assert.Equal(t, uint(0), decoded[0].Epoch)
		assert.Equal(t, "x86_64", decoded[0].Arch)
		assert.Equal(t, int64(1700000000), decoded[0].BuildTime)
		assert.Equal(t, uint64(12345), decoded[0].Size)
		assert.Equal(t, "a250e0b22938f0630f64b0b534141ba0", decoded[0].PayloadHash)
	})

	t.Run("nonzero epoch", func(t *testing.T) {
		pkgs := rpmmd.PackageList{
			{Name: "dnf5", Version: "1.12", Release: "1.fc43", Epoch: 1, Arch: "riscv64"},
		}
		b, err := EncodePackages(pkgs)
		require.NoError(t, err)
		var decoded []kojiRpmListEntry
		require.NoError(t, json.Unmarshal(b.Bytes(), &decoded))
		require.Len(t, decoded, 1)
		require.NotNil(t, decoded[0].Epoch)
		assert.Equal(t, uint(1), decoded[0].Epoch)
	})

	t.Run("more packages", func(t *testing.T) {
		pkgs := rpmmd.PackageList{
			{
				Name: "bash", Version: "5.2", Release: "1.fc43", Epoch: 0, Arch: "x86_64",
				DownloadSize:   12345,
				BuildTime:      time.Unix(1700000000, 0).UTC(),
				Checksum:       rpmmd.Checksum{Type: "sha256", Value: "a250e0b22938f0630f64b0b534141ba0"},
				HeaderChecksum: rpmmd.Checksum{Type: "sha256", Value: "478549d2e9fde87833938316d695a36d"},
			},
			{Name: "dnf5", Version: "1.12", Release: "1.fc43", Epoch: 1, Arch: "riscv64"},
		}
		b, err := EncodePackages(pkgs)
		require.NoError(t, err)

		var decoded []kojiRpmListEntry
		require.NoError(t, json.Unmarshal(b.Bytes(), &decoded))
		require.Len(t, decoded, 2)
		assert.Equal(t, "bash", decoded[0].Name)
		assert.Equal(t, "5.2", decoded[0].Version)
		assert.Equal(t, "1.fc43", decoded[0].Release)
		assert.Equal(t, uint(0), decoded[0].Epoch)
		assert.Equal(t, "x86_64", decoded[0].Arch)
		assert.Equal(t, int64(1700000000), decoded[0].BuildTime)
		assert.Equal(t, uint64(12345), decoded[0].Size)
		assert.Equal(t, "a250e0b22938f0630f64b0b534141ba0", decoded[0].PayloadHash)
		require.NotNil(t, decoded[1].Epoch)
		assert.Equal(t, uint(1), decoded[1].Epoch)
	})

}
