package disk_test

import (
	"encoding/json"
	"fmt"
	"go/types"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/internal/testdisk"
	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/osbuild/image-builder/pkg/disk"

	"golang.org/x/tools/go/packages"
)

func TestMarshalUnmarshalSimple(t *testing.T) {
	fakePt := testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")

	js, err := json.Marshal(fakePt)
	assert.NoError(t, err)

	var ptFromJS disk.PartitionTable
	err = json.Unmarshal(js, &ptFromJS)
	assert.NoError(t, err)
	assert.Equal(t, fakePt, &ptFromJS)
}

func TestUnmarshalSizeUnitStringPartition(t *testing.T) {
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
			var part disk.Partition
			err := json.Unmarshal([]byte(tc.input), &part)
			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, part.Size)
		})
	}
}

func TestMarshalUnmarshalSad(t *testing.T) {
	var part disk.Partition
	err := json.Unmarshal([]byte(`{"random": "json"}`), &part)
	assert.ErrorContains(t, err, `json: unknown field "random"`)
}

func TestMarshalUnmarshalPayloadButNoPayloadTypeSad(t *testing.T) {
	var part disk.Partition
	err := json.Unmarshal([]byte(`{"payload": "some-payload-but-no-payload-type-field"}`), &part)
	assert.ErrorContains(t, err, `cannot build payload: empty payload type but payload is`)
}

func TestMarshalUnmarshalPartitionHappy(t *testing.T) {
	part := &disk.Partition{}

	for _, ent := range []disk.PayloadEntity{
		&disk.Filesystem{Type: "ext2"},
		&disk.LUKSContainer{Passphrase: "secret"},
		&disk.Btrfs{Label: "foo"},
		&disk.LVMVolumeGroup{Name: "bar"},
	} {
		part.Payload = ent
		js, err := json.Marshal(part)
		assert.NoError(t, err)

		var partFromJS disk.Partition
		err = json.Unmarshal(js, &partFromJS)
		assert.NoError(t, err)
		assert.Equal(t, part, &partFromJS)
	}
}

func TestUnmarshalNullPayload(t *testing.T) {
	part := &disk.Partition{}
	part.Payload = nil

	js, err := json.Marshal(part)
	assert.NoError(t, err)

	var partFromJS disk.Partition
	err = json.Unmarshal(js, &partFromJS)
	assert.NoError(t, err)
	assert.Equal(t, part, &partFromJS)
}

func TestAllPayloadEntityExported(t *testing.T) {
	modulePath := "."
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, modulePath)
	assert.NoError(t, err)

	var entityNameImpl []string
	for _, pkg := range pkgs {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj.Exported() {
				named, ok := obj.Type().(*types.Named)
				if !ok {
					continue
				}
				for i := 0; i < named.NumMethods(); i++ {
					method := named.Method(i)
					if method.Name() == "EntityName" {
						entityNameImpl = append(entityNameImpl, obj.Name())
					}
				}
			}
		}
	}
	// precondition check, ensure the test is working
	assert.True(t, len(entityNameImpl) >= 4)
	assert.Contains(t, entityNameImpl, "Btrfs")
	assert.Contains(t, entityNameImpl, "Filesystem")
	// check that when a new PayloadEntity is created it is part of the
	// payloadEntityMap so that the json marshaling will work
	assert.Equal(t, len(entityNameImpl), len(disk.PayloadEntityMap), fmt.Sprintf("the EntityName() function is implemented by %q but only %v are registered in %v, was a new PayloadEntity added but not registered?", entityNameImpl, len(disk.PayloadEntityMap), disk.PayloadEntityMap))
}

func TestImplementsInterfacesCompileTimeCheckPartition(t *testing.T) {
	var _ = disk.Container(&disk.Partition{})
	var _ = disk.Sizeable(&disk.Partition{})
}
func TestIsBIOSBoot(t *testing.T) {
	tests := []struct {
		name      string
		partition *disk.Partition
		expected  bool
	}{
		{
			name:      "nil",
			partition: nil,
			expected:  false,
		},
		{
			name: "gpt-bios-boot",
			partition: &disk.Partition{
				Type: disk.BIOSBootPartitionGUID,
			},
			expected: true,
		},
		{
			name: "dos-bios-boot",
			partition: &disk.Partition{
				Type: disk.BIOSBootPartitionDOSID,
			},
			expected: true,
		},
		{
			name: "non-bios-boot",
			partition: &disk.Partition{
				Type: disk.EFISystemPartitionGUID,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.partition.IsBIOSBoot()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPartitionClone(t *testing.T) {
	p1 := &disk.Partition{
		Start:    123,
		Size:     456,
		Type:     "0x83",
		Bootable: true,
		UUID:     "7a23f22e-3da3-4b3c-a1b7-cc3ebb995307",
		Label:    "part-label",
		Payload:  &disk.Raw{SourcePath: "/source/path"},
		Attrs:    []uint{7, 8, 9},
	}
	p2 := p1.Clone().(*disk.Partition)
	assert.Equal(t, p1, p2)
	// payload/attrs got cloned as well
	assert.False(t, reflect.ValueOf(p1.Payload).Pointer() == reflect.ValueOf(p2.Payload).Pointer())
	assert.False(t, reflect.ValueOf(p1.Attrs).Pointer() == reflect.ValueOf(p2.Attrs).Pointer())
}
