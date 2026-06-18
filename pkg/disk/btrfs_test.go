package disk

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/stretchr/testify/assert"
)

func TestBtrfsSubvolume_GetFSTabOptions(t *testing.T) {
	for _, tc := range []struct {
		subvol          BtrfsSubvolume
		expectedMntOpts string
	}{
		{BtrfsSubvolume{Name: "name"}, "subvol=name"},
		{BtrfsSubvolume{Name: "name", Compress: "gzip"}, "subvol=name,compress=gzip"},
		{BtrfsSubvolume{Name: "root", Compress: "zstd:1", ReadOnly: true},
			"subvol=root,compress=zstd:1,ro"},
	} {
		actual, err := tc.subvol.GetFSTabOptions()
		assert.NoError(t, err)

		assert.Equal(t, FSTabOptions{MntOps: tc.expectedMntOpts}, actual)
	}
}

func TestBtrfsSubvolume_GetFSTabOptionsPanics(t *testing.T) {
	subvol := &BtrfsSubvolume{}
	_, err := subvol.GetFSTabOptions()
	assert.EqualError(t, err, `internal error: BtrfsSubvolume.GetFSTabOptions() for &{Name: Size:0 Mountpoint: GroupID:0 Compress: ReadOnly:false UUID:} called without a name`)
}

func TestImplementsInterfacesCompileTimeCheckBtrfs(t *testing.T) {
	var _ = Container(&Btrfs{})
	var _ = UniqueEntity(&Btrfs{})
	var _ = Mountable(&BtrfsSubvolume{})
	var _ = Sizeable(&BtrfsSubvolume{})
	var _ = FSTabEntity(&BtrfsSubvolume{})
}

func TestUnmarshalSizeUnitStringBtrfsSubvolume(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected datasizes.Size
		err      error
	}{
		{
			name:     "valid size with unit",
			input:    `{"size": "1 GiB"}`,
			expected: 1 * datasizes.GiB,
			err:      nil,
		},
		{
			name:     "valid size without unit",
			input:    `{"size": 1073741824}`,
			expected: 1 * datasizes.GiB,
			err:      nil,
		},
		{
			name:     "valid size without unit as string",
			input:    `{"size": "123"}`,
			expected: 123,
			err:      nil,
		},
		{
			name:     "invalid size with unit",
			input:    `{"size": "1 GGB"}`,
			expected: 0,
			err:      fmt.Errorf("error decoding size: unknown data size units in string: 1 GGB"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bs BtrfsSubvolume
			err := json.Unmarshal([]byte(tc.input), &bs)
			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, bs.Size)
		})
	}
}
