package datasizes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/datasizes"
)

func TestDataSizeToUint64(t *testing.T) {
	cases := []struct {
		input   string
		success bool
		output  uint64
	}{
		{"123", true, 123},
		{"123 kB", true, 123000},
		{"123 KiB", true, 123 * 1024},
		{"123 MB", true, 123 * 1000 * 1000},
		{"123 MiB", true, 123 * 1024 * 1024},
		{"123 GB", true, 123 * 1000 * 1000 * 1000},
		{"123 GiB", true, 123 * 1024 * 1024 * 1024},
		{"123 TB", true, 123 * 1000 * 1000 * 1000 * 1000},
		{"123 TiB", true, 123 * 1024 * 1024 * 1024 * 1024},
		{"123kB", true, 123000},
		{"123KiB", true, 123 * 1024},
		{" 123  ", true, 123},
		{"  123kB  ", true, 123000},
		{"  123KiB  ", true, 123 * 1024},
		{"123 KB", false, 0},
		{"123 mb", false, 0},
		{"123 PB", false, 0},
		{"123 PiB", false, 0},
		{"123.45", true, 123},
		{"1.0", true, 1},
		{"123.00", true, 123},
		{"1.5 GiB", true, 1610612736},
		{"1.5 GB", true, 1500000000},
		{"0.5 MiB", true, 512 * 1024},
		{"0.5 MB", true, 500000},
		{"1.5", true, 2},             // rounds 1.5 -> 2
		{"1.4", true, 1},             // rounds 1.4 -> 1
		{"0.3 GiB", true, 322122547}, // 322122547.2 -> 322122547
		{"1.0000000596046448 MiB", true, 1048576},
	}

	for _, c := range cases {
		result, err := datasizes.Parse(c.input)
		if c.success {
			require.Nil(t, err)
			assert.EqualValues(t, c.output, result)
		} else {
			require.Error(t, err)
		}
	}
}

func TestParseNegativeSizeError(t *testing.T) {
	_, err := datasizes.Parse("-1 MiB")
	assert.ErrorContains(t, err, "the size string is not a valid positive float number: -1")

	_, err = datasizes.Parse("-1 MB")
	assert.ErrorContains(t, err, "the size string is not a valid positive float number: -1")

	_, err = datasizes.Parse("-1.00 MB")
	assert.ErrorContains(t, err, "the size string is not a valid positive float number: -1.00")

	_, err = datasizes.Parse("-1.0000000596046448 MiB")
	assert.ErrorContains(t, err, "the size string is not a valid positive float number: -1.0000000596046448")
}
