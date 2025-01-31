package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	testrepos "github.com/osbuild/images/test/data/repositories"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func TestGetOneImageHappy(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, tc := range []struct {
		distro, imgType, arch string
	}{
		{"centos-9", "qcow2", "x86_64"},
		{"distro:centos-9", "qcow2", "x86_64"},
		{"distro:centos-9", "type:qcow2", "x86_64"},
		{"distro:centos-9", "type:qcow2", "arch:x86_64"},
	} {
		res, err := main.GetOneImage(tc.distro, tc.imgType, tc.arch, nil)
		assert.NoError(t, err)
		assert.Equal(t, "centos-9", res.Distro.Name())
		assert.Equal(t, "qcow2", res.ImgType.Name())
		assert.Equal(t, "x86_64", res.Arch.Name())
	}
}

func TestGetOneImageError(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	for _, tc := range []struct {
		distro, imgType, arch string
		expectedErr           string
	}{
		{
			"unknown", "qcow2", "x86_64",
			`cannot find image for: distro:"unknown" type:"qcow2" arch:"x86_64"`,
		},
		{
			"centos*", "qcow2", "x86_64",
			`cannot use globs in "centos*" when getting a single image`,
		},
	} {
		_, err := main.GetOneImage(tc.distro, tc.imgType, tc.arch, nil)
		assert.EqualError(t, err, tc.expectedErr)
	}
}
