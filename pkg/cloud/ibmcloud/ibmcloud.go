package ibmcloud

import (
	"errors"
	"fmt"
	"io"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"

	"github.com/osbuild/image-builder/pkg/cloud"
)

var _ = cloud.Uploader(&ibmcloudUploader{})

type ibmcloudUploader struct {
	region      string
	bucketName  string
	imageName   string
	credentials *Credentials
}

type Credentials struct {
	AuthEndpoint string

	// Static credentials
	ApiKey string
	Crn    string

	// Config credentials
	Filename    string
	Profilename string

	// Trusted profile credentials
	TrustedProfileID  string
	CrTokenFilePath   string
	ServiceInstanceID string
}

func NewUploader(region string, bucketName string, imageName string, credentials *Credentials) (cloud.Uploader, error) {
	return &ibmcloudUploader{
		region:      region,
		bucketName:  bucketName,
		imageName:   imageName,
		credentials: credentials,
	}, nil
}

func (iu *ibmcloudUploader) Check(status io.Writer) error {
	return nil
}

func (iu *ibmcloudUploader) UploadAndRegister(r io.Reader, uploadSize uint64, status io.Writer) (*cloud.UploadResult, error) {
	fmt.Fprintf(status, "Uploading to IBM Cloud...\n")

	endpoint := fmt.Sprintf("s3.%s.cloud-object-storage.appdomain.cloud", iu.region)
	credentials, err := iu.getCredentials()
	if err != nil {
		return nil, err
	}
	conf := aws.NewConfig().
		WithRegion(iu.region).
		WithEndpoint(endpoint).
		WithCredentials(credentials).
		WithS3ForcePathStyle(true)

	session, err := session.NewSession(conf)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a session: %w", err)
	}

	uploader := s3manager.NewUploader(session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(iu.bucketName),
		Key:    aws.String(iu.imageName),
		Body:   r,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to upload: %w", err)
	}

	return &cloud.UploadResult{
		Provider: "ibmcloud",
	}, nil
}

func (iu *ibmcloudUploader) getCredentials() (*credentials.Credentials, error) {
	if iu.credentials.ApiKey != "" && iu.credentials.Crn != "" {
		return ibmiam.NewStaticCredentials(
			aws.NewConfig(),
			iu.credentials.AuthEndpoint,
			iu.credentials.ApiKey,
			iu.credentials.Crn,
		), nil
	}
	if iu.credentials.Filename != "" && iu.credentials.Profilename != "" {
		return ibmiam.NewConfigCredentials(
			aws.NewConfig(),
			iu.credentials.Filename,
			iu.credentials.Profilename,
		), nil
	}
	if iu.credentials.TrustedProfileID != "" && iu.credentials.CrTokenFilePath != "" && iu.credentials.ServiceInstanceID != "" {
		return ibmiam.NewTrustedProfileCredentialsCR(
			aws.NewConfig(),
			iu.credentials.AuthEndpoint,
			iu.credentials.TrustedProfileID,
			iu.credentials.CrTokenFilePath,
			iu.credentials.ServiceInstanceID,
		), nil
	}
	return nil, errors.New("No usable credentials found")
}
