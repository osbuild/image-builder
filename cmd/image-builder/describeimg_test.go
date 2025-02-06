package main_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	testrepos "github.com/osbuild/images/test/data/repositories"

	"github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func TestDescribeImage(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	res, err := main.GetOneImage("", "centos-9", "tar", "x86_64")
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
