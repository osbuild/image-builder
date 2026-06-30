package azure

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/cloud"
)

const uploaderStorageContainer = "images"

var _ cloud.Uploader = &azureUploader{}

type azureUploader struct {
	client        *Client
	resourceGroup string
	imageName     string
	architecture  arch.Arch
}

func NewUploader(clientID, clientSecret, tenant, subscription, resourceGroup, imageName string, architecture arch.Arch) (cloud.Uploader, error) {
	creds := Credentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	client, err := NewClient(creds, tenant, subscription)
	if err != nil {
		return nil, err
	}

	return &azureUploader{
		client:        client,
		resourceGroup: resourceGroup,
		imageName:     imageName,
		architecture:  architecture,
	}, nil
}

func (au *azureUploader) Check(status io.Writer) error {
	ctx := context.Background()
	fmt.Fprintf(status, "Checking Azure resource group...\n")
	_, err := au.client.GetResourceGroupLocation(ctx, au.resourceGroup)
	if err != nil {
		return fmt.Errorf("cannot access resource group %q: %w", au.resourceGroup, err)
	}
	fmt.Fprintf(status, "Upload conditions met.\n")
	return nil
}

func (au *azureUploader) UploadAndRegister(r io.Reader, uploadSize uint64, status io.Writer) (*cloud.UploadResult, error) {
	// UploadPageBlob requires a file path for seeking (MD5 hash then re-read),
	// so write the reader to a temp file first.
	tmpFile, err := os.CreateTemp("", "azure-upload-*.vhd")
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	fmt.Fprintf(status, "Preparing image for upload...\n")
	if _, err := io.Copy(tmpFile, r); err != nil {
		return nil, fmt.Errorf("cannot write image to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	ctx := context.Background()

	location, err := au.client.GetResourceGroupLocation(ctx, au.resourceGroup)
	if err != nil {
		return nil, err
	}

	staccTag := Tag{
		Name:  "imagesStorageAccount",
		Value: fmt.Sprintf("location=%s", location),
	}
	stacc, err := au.client.GetResourceNameByTag(ctx, au.resourceGroup, staccTag)
	if err != nil {
		return nil, err
	}

	if stacc == "" {
		stacc = RandomStorageAccountName("images")
		fmt.Fprintf(status, "Creating storage account %s...\n", stacc)
		err = au.client.CreateStorageAccount(ctx, au.resourceGroup, stacc, "", staccTag)
		if err != nil {
			return nil, err
		}
	}

	storekey, err := au.client.GetStorageAccountKey(ctx, au.resourceGroup, stacc)
	if err != nil {
		return nil, err
	}

	storeClient, err := NewStorageClient(stacc, storekey)
	if err != nil {
		return nil, err
	}

	err = storeClient.CreateStorageContainerIfNotExist(ctx, stacc, uploaderStorageContainer)
	if err != nil {
		return nil, err
	}

	blobName := EnsureVHDExtension(au.imageName)
	fmt.Fprintf(status, "Uploading %s to Azure...\n", blobName)
	err = storeClient.UploadPageBlob(
		BlobMetadata{
			StorageAccount: stacc,
			ContainerName:  uploaderStorageContainer,
			BlobName:       blobName,
		},
		tmpFile.Name(),
		DefaultUploadThreads,
	)
	if err != nil {
		return nil, err
	}

	switch au.architecture {
	case arch.ARCH_X86_64:
		fmt.Fprintf(status, "Registering image %s...\n", au.imageName)
		err = au.client.RegisterImage(
			ctx,
			au.resourceGroup,
			stacc,
			uploaderStorageContainer,
			blobName,
			au.imageName,
			"",
			HyperVGenV2,
		)
		if err != nil {
			return nil, err
		}
		imageID := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s",
			au.client.SubscriptionID(), au.resourceGroup, au.imageName)
		fmt.Fprintf(status, "Image registered: %s\n", imageID)
		return &cloud.UploadResult{
			Provider: "azure",
			ImageID:  imageID,
		}, nil
	case arch.ARCH_AARCH64:
		fmt.Fprintf(status, "Registering gallery image %s...\n", au.imageName)
		gi, err := au.client.RegisterGalleryImage(
			ctx,
			au.resourceGroup,
			stacc,
			uploaderStorageContainer,
			blobName,
			au.imageName,
			"",
			HyperVGenV2,
			arch.ARCH_AARCH64,
		)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(status, "Gallery image registered: %s\n", gi.ImageRef)
		return &cloud.UploadResult{
			Provider: "azure",
			ImageID:  gi.ImageRef,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture %q for Azure upload", au.architecture)
	}
}
