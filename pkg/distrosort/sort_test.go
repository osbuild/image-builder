package distrosort_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/distrosort"
)

func TestSortNames(t *testing.T) {
	for _, tc := range []struct {
		inp    []string
		sorted []string
	}{
		{
			// distro names are sorted by first
			[]string{"foo-2", "bar-1", "foo-1"},
			[]string{"bar-1", "foo-1", "foo-2"},
		}, {
			// multiple "-" are okay
			[]string{"foo-bar-2", "bar-foo-1", "foo-bar-1"},
			[]string{"bar-foo-1", "foo-bar-1", "foo-bar-2"},
		}, {
			// 1.4 is smaller than 1.10, sort.Strings will get this
			// wrong
			[]string{"foo-1.10", "foo-1.4"},
			[]string{"foo-1.4", "foo-1.10"},
		},
	} {
		err := distrosort.Names(tc.inp)
		assert.NoError(t, err, tc.inp)
		assert.Equal(t, tc.sorted, tc.inp)
	}
}

func TestSortNamesInvalidVersion(t *testing.T) {
	for _, tc := range []struct {
		inp         []string
		expectedErr string
	}{
		{
			[]string{"foo-1.x", "foo-2"},
			`error when parsing distro name "foo-1.x": parsing minor version failed, inner error:
strconv.Atoi: parsing "x": invalid syntax`,
		}, {
			// missing "-" is not supported
			[]string{"foo", "bar-1"},
			`error when parsing distro name "foo": A dash is expected to separate distro name and version`,
		}, {
			// foo-1.4-beta is not supported
			[]string{"foo-1.4", "foo-1.4-beta", "foo-1.0"},
			`error when parsing distro name "foo-1.4-beta": parsing major version failed, inner error:
strconv.Atoi: parsing "beta": invalid syntax`,
		},
	} {
		err := distrosort.Names(tc.inp)
		assert.ErrorContains(t, err, tc.expectedErr, tc.inp)
	}
}
