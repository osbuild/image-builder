package main_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	testrepos "github.com/osbuild/images/test/data/repositories"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func TestDescribeImage(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	res, err := main.GetOneImage("centos-9", "tar", "x86_64", nil)
	assert.NoError(t, err)

	var buf bytes.Buffer
	err = main.DescribeImage(res, &buf)
	assert.NoError(t, err)

	expectedOutput := `@WARNING - the output format is not stable yet and may change
distro: centos-9
type: tar
arch: x86_64
os_version: 9-stream
bootmode: none
partition_type: ""
default_filename: root.tar.xz
build_pipelines:
  - build
payload_pipelines:
  - os
  - archive
packages:
  build:
    include:
      - coreutils
      - glibc
      - platform-python
      - policycoreutils
      - python3
      - rpm
      - selinux-policy-targeted
      - systemd
      - tar
      - xz
    exclude: []
  os:
    include:
      - policycoreutils
      - selinux-policy-targeted
    exclude:
      - rng-tools
`
	assert.Equal(t, expectedOutput, buf.String())
}

func TestDescribeImageAll(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	allImages, err := main.GetAllImages(nil)
	require.NoError(t, err)
	require.NotEmpty(t, allImages)

	for _, res := range allImages {
		t.Run(fmt.Sprintf("%s/%s/%s", res.Distro.Name(), res.Arch.Name(), res.ImgType.Name()), func(t *testing.T) {
			var buf bytes.Buffer
			err = main.DescribeImage(&res, &buf)
			require.NoError(t, err)

			// check that the first line of the output contains the "@WARNING" message
			lines := strings.Split(buf.String(), "\n")
			require.NotEmpty(t, lines)
			require.Equal(t, "@WARNING - the output format is not stable yet and may change", lines[0])

			// the rest of the output should contain a valid YAML representation of the image
			describeOutput := strings.Join(lines[1:], "\n")
			var imgDef main.DescribeImgYAML
			err := yaml.Unmarshal([]byte(describeOutput), &imgDef)
			require.NoError(t, err)
			require.Equal(t, res.Distro.Name(), imgDef.Distro)
			require.Equal(t, res.Arch.Name(), imgDef.Arch)
			require.Equal(t, res.ImgType.Name(), imgDef.Type)
		})
	}
}
