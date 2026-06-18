package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/image-builder/cmd/image-builder"
	"github.com/osbuild/image-builder/pkg/bootc"
)

func mockBootcInfo() *bootc.Info {
	return &bootc.Info{
		Imgref:  "quay.io/test/bootc:latest",
		ImageID: "sha256:abc123",
		Arch:    "x86_64",
	}
}

func TestBootcInspectDefaultYAML(t *testing.T) {
	restore := main.MockBootcResolveInfo(func(ref string) (*bootc.Info, error) {
		assert.Equal(t, "quay.io/test/bootc:latest", ref)
		return mockBootcInfo(), nil
	})
	defer restore()

	restore = main.MockOsArgs([]string{"bootc", "inspect", "--ref=quay.io/test/bootc:latest"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	output := fakeStdout.String()
	assert.Contains(t, output, "imgref: quay.io/test/bootc:latest")
	assert.Contains(t, output, "imageid: sha256:abc123")
	assert.Contains(t, output, "arch: x86_64")
}

func TestBootcInspectExplicitYAML(t *testing.T) {
	restore := main.MockBootcResolveInfo(func(ref string) (*bootc.Info, error) {
		return mockBootcInfo(), nil
	})
	defer restore()

	restore = main.MockOsArgs([]string{"bootc", "inspect", "--ref=quay.io/test/bootc:latest", "--format=yaml"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	output := fakeStdout.String()
	assert.Contains(t, output, "imgref: quay.io/test/bootc:latest")
}

func TestBootcInspectJSON(t *testing.T) {
	restore := main.MockBootcResolveInfo(func(ref string) (*bootc.Info, error) {
		return mockBootcInfo(), nil
	})
	defer restore()

	restore = main.MockOsArgs([]string{"bootc", "inspect", "--ref=quay.io/test/bootc:latest", "--format=json"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(fakeStdout.Bytes(), &parsed)
	require.NoError(t, err, "output must be valid JSON")
	assert.Equal(t, "quay.io/test/bootc:latest", parsed["Imgref"])
	assert.Equal(t, "sha256:abc123", parsed["ImageID"])
	assert.Equal(t, "x86_64", parsed["Arch"])
}

func TestBootcInspectUnsupportedFormat(t *testing.T) {
	restore := main.MockBootcResolveInfo(func(ref string) (*bootc.Info, error) {
		return mockBootcInfo(), nil
	})
	defer restore()

	restore = main.MockOsArgs([]string{"bootc", "inspect", "--ref=quay.io/test/bootc:latest", "--format=xml"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.EqualError(t, err, `unsupported format "xml", supported formats: yaml, json`)
}

func TestBootcInspectResolveError(t *testing.T) {
	restore := main.MockBootcResolveInfo(func(ref string) (*bootc.Info, error) {
		return nil, fmt.Errorf("cannot resolve %q", ref)
	})
	defer restore()

	restore = main.MockOsArgs([]string{"bootc", "inspect", "--ref=quay.io/bad/ref:latest"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.EqualError(t, err, `cannot resolve "quay.io/bad/ref:latest"`)
}

func TestBootcInspectMissingRef(t *testing.T) {
	restore := main.MockBootcResolveInfo(func(ref string) (*bootc.Info, error) {
		return mockBootcInfo(), nil
	})
	defer restore()

	restore = main.MockOsArgs([]string{"bootc", "inspect"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ref")
}
