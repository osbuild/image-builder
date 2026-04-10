package awscloud_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/platform"
)

// XXX: put into a new "cloudtest" package?
type fakeAWSClient struct {
	regions      []string
	regionsErr   error
	regionsCalls int

	buckets      []string
	bucketsErr   error
	bucketsCalls int

	checkBucketPermission      bool
	checkBucketPermissionErr   error
	checkBucketPermissionCalls int

	uploadFromReader      *s3manager.UploadOutput
	uploadFromReaderErr   error
	uploadFromReaderCalls int

	registerErr        error
	registerImageId    string
	registerSnapshotId string
	registerBootMode   *platform.BootMode
	registerCalls      int

	deleteObjectErr   error
	deleteObjectCalls int
}

func (fa *fakeAWSClient) Regions() ([]string, error) {
	fa.regionsCalls++
	return fa.regions, fa.regionsErr
}

func (fa *fakeAWSClient) Buckets() ([]string, error) {
	fa.bucketsCalls++
	return fa.buckets, fa.bucketsErr
}

func (fa *fakeAWSClient) CheckBucketPermission(string, s3types.Permission) (bool, error) {
	fa.checkBucketPermissionCalls++
	return fa.checkBucketPermission, fa.checkBucketPermissionErr
}

func (fa *fakeAWSClient) UploadFromReader(io.Reader, string, string) (*s3manager.UploadOutput, error) {
	fa.uploadFromReaderCalls++
	return fa.uploadFromReader, fa.uploadFromReaderErr
}

func (fa *fakeAWSClient) Register(name, bucket, key string, tags []awscloud.AWSTag, shareWith []string, architecture arch.Arch, bootMode *platform.BootMode, importRole *string) (string, string, error) {
	fa.registerCalls++
	fa.registerBootMode = bootMode
	return fa.registerImageId, fa.registerSnapshotId, fa.registerErr
}

func (fa *fakeAWSClient) DeleteObject(string, string) error {
	fa.deleteObjectCalls++
	return fa.deleteObjectErr
}

func TestUploaderCheckHappy(t *testing.T) {
	fa := &fakeAWSClient{
		regions:               []string{"region"},
		buckets:               []string{"bucket"},
		checkBucketPermission: true,
	}
	restore := awscloud.MockNewAwsClient(func(string, string) (awscloud.AwsClient, error) {
		return fa, nil
	})
	defer restore()

	uploader, err := awscloud.NewUploader("region", "bucket", "ami", nil)
	assert.NoError(t, err)
	var statusLog bytes.Buffer
	err = uploader.Check(&statusLog)
	assert.NoError(t, err)
	assert.Equal(t, 1, fa.regionsCalls)
	assert.Equal(t, 1, fa.bucketsCalls)
	assert.Equal(t, 1, fa.checkBucketPermissionCalls)
	expectedStatusLog := `Checking AWS region access...
Checking AWS bucket...
Checking AWS bucket permissions...
Upload conditions met.
`
	assert.Equal(t, expectedStatusLog, statusLog.String())
}

type repeatReader struct{}

func (r *repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0x1
	}
	return len(p), nil
}

func TestUploaderUploadHappy(t *testing.T) {
	testCases := []struct {
		name     string
		opts     *awscloud.UploaderOptions
		check_fn func(*testing.T, *fakeAWSClient)
	}{
		{
			name: "default",
			opts: nil,
			check_fn: func(t *testing.T, fa *fakeAWSClient) {
				assert.Nil(t, fa.registerBootMode)
			},
		},
		{
			name: "ec2-boot-mode-uefi",
			opts: &awscloud.UploaderOptions{
				BootMode: common.ToPtr(platform.BOOT_UEFI),
			},
			check_fn: func(t *testing.T, fa *fakeAWSClient) {
				assert.NotNil(t, fa.registerBootMode)
				assert.Equal(t, platform.BOOT_UEFI, *fa.registerBootMode)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uuid.SetRand(&repeatReader{})

			fa := &fakeAWSClient{
				uploadFromReader: &s3manager.UploadOutput{
					Location: "some-location",
				},
				registerImageId:    "image-id",
				registerSnapshotId: "snapshot-id",
			}
			restore := awscloud.MockNewAwsClient(func(string, string) (awscloud.AwsClient, error) {
				return fa, nil
			})
			defer restore()

			fakeImage := bytes.NewBufferString("fake-aws-image")
			uploader, err := awscloud.NewUploader("region", "bucket", "ami", tc.opts)
			assert.NoError(t, err)
			var uploadLog bytes.Buffer
			err = uploader.UploadAndRegister(fakeImage, 0, &uploadLog)
			assert.NoError(t, err)
			assert.Equal(t, 1, fa.uploadFromReaderCalls)
			assert.Equal(t, 1, fa.registerCalls)
			assert.Equal(t, 1, fa.deleteObjectCalls)
			expectedUploadLog := `Uploading ami to bucket:01010101-0101-4101-8101-010101010101-ami
File uploaded to some-location
Registering AMI ami
Deleted S3 object bucket:01010101-0101-4101-8101-010101010101-ami
AMI registered: image-id
Snapshot ID: snapshot-id
`
			assert.Equal(t, expectedUploadLog, uploadLog.String())
			tc.check_fn(t, fa)
		})
	}
}

func TestUploaderUploadButRegisterError(t *testing.T) {
	uuid.SetRand(&repeatReader{})

	fa := &fakeAWSClient{
		uploadFromReader: &s3manager.UploadOutput{
			Location: "some-location",
		},
		registerErr: fmt.Errorf("fake-register-err"),
	}
	restore := awscloud.MockNewAwsClient(func(string, string) (awscloud.AwsClient, error) {
		return fa, nil
	})
	defer restore()

	fakeImage := bytes.NewBufferString("fake-aws-image")
	uploader, err := awscloud.NewUploader("region", "bucket", "ami", nil)
	assert.NoError(t, err)
	var uploadLog bytes.Buffer
	err = uploader.UploadAndRegister(fakeImage, 0, &uploadLog)
	// XXX: this should probably have a context
	assert.EqualError(t, err, "fake-register-err")
	assert.Equal(t, 1, fa.uploadFromReaderCalls)
	assert.Equal(t, 1, fa.registerCalls)
	assert.Equal(t, 1, fa.deleteObjectCalls)
	expectedUploadLog := `Uploading ami to bucket:01010101-0101-4101-8101-010101010101-ami
File uploaded to some-location
Registering AMI ami
Deleted S3 object bucket:01010101-0101-4101-8101-010101010101-ami
`
	assert.Equal(t, expectedUploadLog, uploadLog.String())
}

func TestUploaderUploadButRegisterErrorAndDeleteError(t *testing.T) {
	fa := &fakeAWSClient{
		uploadFromReader: &s3manager.UploadOutput{
			Location: "some-location",
		},
		registerErr:     fmt.Errorf("fake-register-err"),
		deleteObjectErr: fmt.Errorf("fake-delete-object-err"),
	}
	restore := awscloud.MockNewAwsClient(func(string, string) (awscloud.AwsClient, error) {
		return fa, nil
	})
	defer restore()

	fakeImage := bytes.NewBufferString("fake-aws-image")
	uploader, err := awscloud.NewUploader("region", "bucket", "ami", nil)
	assert.NoError(t, err)
	var uploadLog bytes.Buffer
	err = uploader.UploadAndRegister(fakeImage, 0, &uploadLog)
	// XXX: this should probably have a context
	assert.EqualError(t, err, "fake-register-err\nfake-delete-object-err")
}
