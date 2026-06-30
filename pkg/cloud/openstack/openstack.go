package openstack

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/osbuild/image-builder/pkg/cloud"

	"github.com/gophercloud/gophercloud/v2"
	ostack "github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/imagedata"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
)

var _ = cloud.Uploader(&openstackUploader{})

type openstackUploader struct {
	image           string
	diskFormat      string
	containerFormat string
}

type UploaderOptions struct {
	DiskFormat      string
	ContainerFormat string
}

func NewUploader(image string, opts *UploaderOptions) (cloud.Uploader, error) {
	return &openstackUploader{
		image:           image,
		diskFormat:      opts.DiskFormat,
		containerFormat: opts.ContainerFormat,
	}, nil
}

func (ou *openstackUploader) Check(status io.Writer) error {
	return nil
}

func (ou *openstackUploader) UploadAndRegister(r io.Reader, uploadSize uint64, status io.Writer) (*cloud.UploadResult, error) {
	fmt.Fprintf(status, "Uploading to OpenStack...\n")

	opts, err := ostack.AuthOptionsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("Failed to read OpenStack ENV variables. Please source the OpenStack RC file: %w", err)
	}

	// This is needed otherwise we get the following error when authenticating:
	//	   You must provide exactly one of DomainID or DomainName to
	//	   authenticate by Username
	// Even with an RC file that works perfectly fine with `openstack token issue`
	// See https://github.com/gophercloud/gophercloud/issues/3440
	// See https://github.com/gophercloud/gophercloud/issues/3240
	if opts.DomainName == "" {
		opts.DomainName = os.Getenv("OS_USER_DOMAIN_NAME")
	}

	ctx := context.Background()
	provider, err := ostack.AuthenticatedClient(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate to OpenStack: %w", err)
	}

	client, err := ostack.NewImageV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize the client: %w", err)
	}

	createOpts := images.CreateOpts{
		Name:            ou.image,
		DiskFormat:      ou.diskFormat,
		ContainerFormat: ou.containerFormat,
	}
	img, err := images.Create(ctx, client, createOpts).Extract()
	if err != nil {
		return nil, fmt.Errorf("Failed to create the image metadata: %w", err)
	}

	err = imagedata.Upload(ctx, client, img.ID, r).ExtractErr()
	if err != nil {
		return nil, fmt.Errorf("Failed to upload the image: %w", err)
	}

	return &cloud.UploadResult{
		Provider: "openstack",
		ImageID:  img.ID,
	}, nil
}
