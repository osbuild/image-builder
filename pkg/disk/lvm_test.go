package disk

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/datasizes"
)

func TestLVMVCreateMountpoint(t *testing.T) {

	assert := assert.New(t)

	vg := &LVMVolumeGroup{
		Name:        "root",
		Description: "root volume group",
	}

	entity, err := vg.CreateMountpoint("/", "", 0)
	assert.NoError(err)
	rootlv := entity.(*LVMLogicalVolume)
	assert.Equal("rootlv", rootlv.Name)

	_, err = vg.CreateMountpoint("/home_test", "", 0)
	assert.NoError(err)

	entity, err = vg.CreateMountpoint("/home/test", "", 0)
	assert.NoError(err)

	dedup := entity.(*LVMLogicalVolume)
	assert.Equal("home_testlv00", dedup.Name)

	// Lets collide it
	for i := 0; i < 99; i++ {
		_, err = vg.CreateMountpoint("/home/test", "", 0)
		assert.NoError(err)
	}

	_, err = vg.CreateMountpoint("/home/test", "", 0)
	assert.Error(err)
}

func TestLVMVCreateLogicalVolumeSwap(t *testing.T) {
	vg := &LVMVolumeGroup{
		Name:        "root",
		Description: "root volume group",
	}
	swap := &Swap{}
	lv, err := vg.CreateLogicalVolume("", 12345, swap)
	assert.NoError(t, err)
	assert.Equal(t, "swaplv", lv.Name)
	// one more
	lv2, err := vg.CreateLogicalVolume("", 12345, swap)
	assert.NoError(t, err)
	assert.Equal(t, "swaplv00", lv2.Name)
}

func TestLVMVCreateLogicalVolumeWrongType(t *testing.T) {
	vg := &LVMVolumeGroup{
		Name: "root",
	}
	_, err := vg.CreateLogicalVolume("", 12345, &LUKSContainer{})
	assert.EqualError(t, err, `could not create logical volume: no name provided and payload *disk.LUKSContainer is not mountable or swap`)
}

func TestImplementsInterfacesCompileTimeCheckLVM(t *testing.T) {
	var _ = Container(&LVMVolumeGroup{})
	var _ = Sizeable(&LVMLogicalVolume{})
}

func TestLVMLogicalVolumeEnsureSize(t *testing.T) {
	lv := &LVMLogicalVolume{
		Size: 1024 * 1024,
	}
	resized := lv.EnsureSize(1024*1024 + 17)
	assert.True(t, resized)
	assert.Equal(t, datasizes.Size(4*datasizes.MiB), lv.Size)
}

func TestUnmarshalSizeUnitString(t *testing.T) {
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
			var lv LVMLogicalVolume
			err := json.Unmarshal([]byte(tc.input), &lv)
			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, lv.Size)
		})
	}
}
