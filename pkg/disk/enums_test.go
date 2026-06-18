package disk_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/pkg/disk"
)

var partitionTypeEnumMap = map[string]disk.PartitionTableType{
	"":    disk.PT_NONE,
	"dos": disk.PT_DOS,
	"gpt": disk.PT_GPT,
}

func TestEnumPartitionTableType(t *testing.T) {
	assert := assert.New(t)
	for name, num := range partitionTypeEnumMap {
		ptt, err := disk.NewPartitionTableType(name)
		expected := disk.PartitionTableType(num)

		assert.NoError(err)
		assert.Equal(expected, ptt)

		assert.Equal(name, ptt.String())
	}

	// error test: bad value
	badPtt := disk.PartitionTableType(3)
	assert.PanicsWithValue("unknown or unsupported partition table type with enum value 3", func() { _ = badPtt.String() })

	// error test: bad name
	_, err := disk.NewPartitionTableType("not-a-type")
	assert.EqualError(err, "unknown or unsupported partition table type name: not-a-type")
}

func TestEnumPartitionTableTypeJSON(t *testing.T) {
	for name, num := range partitionTypeEnumMap {
		jsData, err := json.Marshal(num)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf(`"%s"`, name), string(jsData))

		var unmarshaledNum disk.PartitionTableType
		err = json.Unmarshal(jsData, &unmarshaledNum)
		assert.NoError(t, err)
		assert.Equal(t, unmarshaledNum, num)
	}

	// bad unmarshal
	var unmarshaledNum disk.PartitionTableType
	err := json.Unmarshal([]byte(`"bad"`), &unmarshaledNum)
	assert.EqualError(t, err, `unknown or unsupported partition table type name: bad`)
}

func TestEnumPartitionTableTypeYAML(t *testing.T) {
	for name, num := range partitionTypeEnumMap {
		yamlData, err := yaml.Marshal(num)
		assert.NoError(t, err)
		if name == "" {
			assert.Equal(t, `""`+"\n", string(yamlData))
		} else {
			assert.Equal(t, fmt.Sprintf("%s\n", name), string(yamlData))
		}

		var unmarshaledNum disk.PartitionTableType
		err = yaml.Unmarshal(yamlData, &unmarshaledNum)
		assert.NoError(t, err)
		assert.Equal(t, unmarshaledNum, num)
	}

	// bad unmarshal
	var unmarshaledNum disk.PartitionTableType
	err := yaml.Unmarshal([]byte(`"bad"`), &unmarshaledNum)
	assert.EqualError(t, err, `unmarshal yaml via json for "bad" failed: unknown or unsupported partition table type name: bad`)
}

func TestEnumFSType(t *testing.T) {
	enumMap := map[string]disk.FSType{
		"":      disk.FS_NONE,
		"vfat":  disk.FS_VFAT,
		"ext4":  disk.FS_EXT4,
		"xfs":   disk.FS_XFS,
		"btrfs": disk.FS_BTRFS,
	}

	assert := assert.New(t)
	for name, num := range enumMap {
		fst, err := disk.NewFSType(name)
		expected := disk.FSType(num)

		assert.NoError(err)
		assert.Equal(expected, fst)

		assert.Equal(name, fst.String())
	}

	// error test: bad value
	badFst := disk.FSType(5)
	assert.PanicsWithValue("unknown or unsupported filesystem type with enum value 5", func() { _ = badFst.String() })

	// error test: bad name
	_, err := disk.NewFSType("not-a-type")
	assert.EqualError(err, "unknown or unsupported filesystem type name: not-a-type")
}
