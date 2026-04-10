package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/disk"
)

func TestNewSfdiskStage(t *testing.T) {

	partition := SfdiskPartition{
		Bootable: true,
		Name:     "root",
		Size:     2097152,
		Start:    0,
		Type:     "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
		UUID:     "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
	}

	options := SfdiskStageOptions{
		UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Partitions: []SfdiskPartition{partition},
	}

	device := NewLoopbackDevice(&LoopbackDeviceOptions{Filename: "disk.raw"})
	devices := map[string]Device{"device": *device}

	expectedStage := &Stage{
		Type:    "org.osbuild.sfdisk",
		Options: &options,
		Devices: devices,
	}

	// test with gpt
	options.Label = "gpt"
	actualStageGPT := NewSfdiskStage(&options, device)
	assert.Equal(t, expectedStage, actualStageGPT)

	// test again with dos
	options.Label = "dos"
	actualStageDOS := NewSfdiskStage(&options, device)
	assert.Equal(t, expectedStage, actualStageDOS)

	// test with attributes
	options.Partitions[0].Attrs = []uint{50, 51}
	actualStageAttr := NewSfdiskStage(&options, device)
	assert.Equal(t, expectedStage, actualStageAttr)
}

func TestNewSfdiskStageInvalid(t *testing.T) {

	partition := SfdiskPartition{
		// doesn't really matter
	}

	options := SfdiskStageOptions{
		Label:      "dos",
		UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Partitions: []SfdiskPartition{partition, partition, partition, partition, partition}, // 5 partitions
	}

	device := NewLoopbackDevice(&LoopbackDeviceOptions{Filename: "disk.raw"})

	assert.Panics(t, func() {
		NewSfdiskStage(&options, device)
	})
}

func TestNewSfdiskStageExtended(t *testing.T) {
	partition := SfdiskPartition{
		// doesn't really matter
	}
	extended := SfdiskPartition{
		Type: disk.ExtendedPartitionDOSID,
	}

	options := SfdiskStageOptions{
		Label:      "dos",
		UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Partitions: []SfdiskPartition{partition, partition, partition, extended, partition}, // 5 partitions, one is extended
	}

	device := NewLoopbackDevice(&LoopbackDeviceOptions{Filename: "disk.raw"})

	assert.NotPanics(t, func() {
		NewSfdiskStage(&options, device)
	})

	// Test with extended partition outside the first 4 slots (should fail)
	options.Partitions = []SfdiskPartition{partition, partition, partition, partition, extended}
	assert.Panics(t, func() {
		NewSfdiskStage(&options, device)
	})
}
