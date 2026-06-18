package distrofactory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/distro/defs"
)

func TestGetDistroDefaultList(t *testing.T) {
	type testCase struct {
		strID              string
		expectedDistroName string
	}

	testCases := []testCase{
		{
			strID:              "rhel-7.9",
			expectedDistroName: "rhel-7.9",
		},
		{
			strID:              "rhel-89",
			expectedDistroName: "rhel-8.9",
		},
		{
			strID:              "rhel-8.9",
			expectedDistroName: "rhel-8.9",
		},
		{
			strID:              "rhel-810",
			expectedDistroName: "rhel-8.10",
		},
		{
			strID:              "rhel-8.10",
			expectedDistroName: "rhel-8.10",
		},
		{
			strID:              "rhel-91",
			expectedDistroName: "rhel-9.1",
		},
		{
			strID:              "rhel-9.1",
			expectedDistroName: "rhel-9.1",
		},
		{
			strID:              "rhel-910",
			expectedDistroName: "rhel-9.10",
		},
		{
			strID:              "rhel-9.10",
			expectedDistroName: "rhel-9.10",
		},
		{
			strID:              "rhel-10.1",
			expectedDistroName: "rhel-10.1",
		},
		{
			strID:              "rhel-10.10",
			expectedDistroName: "rhel-10.10",
		},
		{
			strID:              "fedora-42",
			expectedDistroName: "fedora-42",
		},
	}

	df := NewDefault()

	for _, tc := range testCases {
		t.Run(tc.strID, func(t *testing.T) {
			d := df.GetDistro(tc.strID)
			assert.NotNil(t, d)
			assert.Equal(t, tc.expectedDistroName, d.Name())
		})
	}

}

func TestNewDefaultWithLoader(t *testing.T) {
	loader := defs.BuiltinLoader()
	df := NewDefaultWithLoader(loader)

	d := df.GetDistro("rhel-10.1")
	assert.NotNil(t, d)
	assert.Equal(t, "rhel-10.1", d.Name())

	d = df.GetDistro("fedora-42")
	assert.NotNil(t, d)
	assert.Equal(t, "fedora-42", d.Name())

	d = df.GetDistro("nonexistent-1")
	assert.Nil(t, d)
}

func TestGetDistroDefaultListWithAliases(t *testing.T) {
	type testCase struct {
		aliases            map[string]string
		strID              string
		expectedDistroName string
		fail               bool
		errorMsg           string
	}

	testCases := []testCase{
		{
			aliases: map[string]string{
				"rhel-9": "rhel-9.1",
			},
			strID:              "rhel-9",
			expectedDistroName: "rhel-9.1",
		},
		{
			aliases: map[string]string{
				"best_distro-123": "rhel-9.1",
			},
			strID:              "best_distro-123",
			expectedDistroName: "rhel-9.1",
		},
		{
			aliases: map[string]string{
				"rhel-9.3": "rhel-9.1",
				"rhel-9.2": "rhel-9.1",
			},
			fail:     true,
			errorMsg: `invalid aliases: ["alias 'rhel-9.2' masks an existing distro" "alias 'rhel-9.3' masks an existing distro"]`,
		},
		{
			aliases: map[string]string{
				"rhel-12": "rhel-12.12",
				"rhel-13": "rhel-13.13",
			},
			fail:     true,
			errorMsg: `invalid aliases: ["alias 'rhel-12' targets a non-existing distro 'rhel-12.12'" "alias 'rhel-13' targets a non-existing distro 'rhel-13.13'"]`,
		},
	}

	df := NewDefault()
	for _, tc := range testCases {
		t.Run(tc.strID, func(t *testing.T) {
			err := df.RegisterAliases(tc.aliases)

			if tc.fail {
				assert.Error(t, err)
				assert.Equal(t, tc.errorMsg, err.Error())
				return
			}

			assert.NoError(t, err)
			d := df.GetDistro(tc.strID)
			assert.NotNil(t, d)
			assert.Equal(t, tc.expectedDistroName, d.Name())
		})
	}

}
