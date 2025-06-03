package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/platform"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
	"github.com/osbuild/image-builder-cli/internal/testutil"
)

type fakeAwsUploader struct {
	checkCalls int

	uploadAndRegisterRead  bytes.Buffer
	uploadAndRegisterCalls int
	uploadAndRegisterErr   error
}

var _ = cloud.Uploader(&fakeAwsUploader{})

func (fa *fakeAwsUploader) Check(status io.Writer) error {
	fa.checkCalls++
	return nil
}

func (fa *fakeAwsUploader) UploadAndRegister(r io.Reader, status io.Writer) error {
	fa.uploadAndRegisterCalls++
	_, err := io.Copy(&fa.uploadAndRegisterRead, r)
	if err != nil {
		panic(err)
	}
	return fa.uploadAndRegisterErr
}

func TestUploadWithAWSMock(t *testing.T) {
	fakeDiskContent := "fake-raw-img"

	fakeImageFilePath := filepath.Join(t.TempDir(), "disk.raw")
	err := os.WriteFile(fakeImageFilePath, []byte(fakeDiskContent), 0644)
	assert.NoError(t, err)

	var regionName, bucketName, amiName string
	var uploadOpts *awscloud.UploaderOptions

	for _, tc := range []struct {
		targetArchArg      string
		expectedUploadArch string
	}{
		{"", ""},
		{"aarch64", "aarch64"},
	} {
		var fa fakeAwsUploader
		restore := main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
			regionName = region
			bucketName = bucket
			amiName = ami
			uploadOpts = opts
			return &fa, nil
		})
		defer restore()

		var fakeStdout bytes.Buffer
		restore = main.MockOsStdout(&fakeStdout)
		defer restore()

		restore = main.MockOsArgs([]string{
			"upload",
			"--to=aws",
			"--aws-region=aws-region-1",
			"--aws-bucket=aws-bucket-2",
			"--aws-ami-name=aws-ami-3",
			"--arch=" + tc.targetArchArg,
			fakeImageFilePath,
		})
		defer restore()

		err = main.Run()
		require.NoError(t, err)

		assert.Equal(t, regionName, "aws-region-1")
		assert.Equal(t, bucketName, "aws-bucket-2")
		assert.Equal(t, amiName, "aws-ami-3")
		expectedBootMode := platform.BOOT_HYBRID
		assert.Equal(t, &awscloud.UploaderOptions{TargetArch: tc.expectedUploadArch, BootMode: &expectedBootMode}, uploadOpts)

		assert.Equal(t, 0, fa.checkCalls)
		assert.Equal(t, 1, fa.uploadAndRegisterCalls)
		assert.Equal(t, fakeDiskContent, fa.uploadAndRegisterRead.String())
		// progress was rendered
		assert.Contains(t, fakeStdout.String(), "--] 100.00%")
	}
}

func TestUploadCmdlineErrors(t *testing.T) {
	for _, tc := range []struct {
		cmdline     []string
		expectedErr string
	}{
		{
			nil,
			`missing --to parameter, try --to=aws`,
		}, {
			[]string{"--to=aws"},
			`missing all upload configuration: ["--aws-ami-name" "--aws-bucket" "--aws-region"]`,
		},
		{
			[]string{"--to=aws", "--aws-ami-name=1"},
			`missing upload configuration: ["--aws-bucket" "--aws-region"]`,
		},
		{
			[]string{"--to=aws", "--aws-ami-name=1", "--aws-bucket=2"},
			`missing upload configuration: ["--aws-region"]`,
		},
	} {
		t.Run(strings.Join(tc.cmdline, ","), func(t *testing.T) {
			cmd := append([]string{"upload"}, tc.cmdline...)
			cmd = append(cmd, "/path/to/some/image")
			restore := main.MockOsArgs(cmd)
			defer restore()

			err := main.Run()
			require.EqualError(t, err, tc.expectedErr)
		})
	}
}

func TestBuildAndUploadWithAWSMock(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	var regionName, bucketName, amiName string
	var fa fakeAwsUploader
	var uploadOpts *awscloud.UploaderOptions
	restore := main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
		regionName = region
		bucketName = bucket
		amiName = ami
		uploadOpts = opts
		return &fa, nil
	})
	defer restore()

	outputDir := t.TempDir()
	fakeOsbuildScript := makeFakeOsbuildScript()
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", fakeOsbuildScript)
	defer fakeOsbuildCmd.Restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	restore = main.MockOsArgs([]string{
		"build",
		"--output-dir", outputDir,
		"--aws-region=aws-region-1",
		"--aws-bucket=aws-bucket-2",
		"--aws-ami-name=aws-ami-3",
		"ami",
		"--distro=centos-9",
	})
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	assert.Equal(t, regionName, "aws-region-1")
	assert.Equal(t, bucketName, "aws-bucket-2")
	assert.Equal(t, amiName, "aws-ami-3")
	expectedBootMode := platform.BOOT_HYBRID
	assert.Equal(t, &awscloud.UploaderOptions{BootMode: &expectedBootMode}, uploadOpts)
	assert.Equal(t, 1, fa.checkCalls)
	assert.Equal(t, 1, fa.uploadAndRegisterCalls)
	assert.Equal(t, "fake-img-raw\n", fa.uploadAndRegisterRead.String())
}

func TestBuildAmiButNotUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	fa := fakeAwsUploader{
		uploadAndRegisterErr: fmt.Errorf("upload should not be called"),
	}
	restore := main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
		return &fa, nil
	})
	defer restore()

	outputDir := t.TempDir()
	fakeOsbuildScript := makeFakeOsbuildScript()
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", fakeOsbuildScript)
	defer fakeOsbuildCmd.Restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	restore = main.MockOsArgs([]string{
		"build",
		"--output-dir", outputDir,
		"ami",
		"--distro=centos-9",
	})
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	assert.Equal(t, 0, fa.uploadAndRegisterCalls)
}

func TestBuildAndUploadWithAWSPartialCmdlineErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	outputDir := t.TempDir()
	fakeOsbuildScript := makeFakeOsbuildScript()
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", fakeOsbuildScript)
	defer fakeOsbuildCmd.Restore()

	restore := main.MockOsArgs([]string{
		"build",
		"--output-dir", outputDir,
		// note that --aws-{ami-name,bucket} is missing
		"--aws-region=aws-region-1",
		"ami",
		"--distro=centos-9",
	})
	defer restore()

	err := main.Run()
	assert.EqualError(t, err, `missing upload configuration: ["--aws-ami-name" "--aws-bucket"]`)
}
