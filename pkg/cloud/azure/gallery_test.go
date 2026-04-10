package azure_test

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud/azure"
)

func TestRegisterGalleryImage(t *testing.T) {
	azm := newAZ()

	gi, err := azm.az.RegisterGalleryImage(
		t.Context(),
		"rg",
		"storacc",
		"storcontainer",
		"blobname",
		"img-name",
		"",
		azure.HyperVGenV2,
		arch.ARCH_AARCH64,
	)
	require.NoError(t, err)
	require.Equal(t, "rg", gi.ResourceGroup)
	require.Equal(t, "img_name_gallery", gi.Gallery)
	require.Equal(t, "/subscriptions/test-subscription/resourceGroups/rg/providers/Microsoft.Compute/galleries/img_name_gallery/images/img-name-img/versions/1.0.0", gi.ImageRef)

	// resolving the empty location
	require.Len(t, azm.rgm.get, 1)

	require.Len(t, azm.gm.createOrUpdate, 1)
	require.Equal(t, "rg", azm.gm.createOrUpdate[0].rg)
	require.Equal(t, "img_name_gallery", azm.gm.createOrUpdate[0].name)
	require.Equal(t, armcompute.Gallery{
		Location: common.ToPtr("test-universe"),
	}, azm.gm.createOrUpdate[0].gallery)

	require.Len(t, azm.gim.createOrUpdate, 1)
	require.Equal(t, "rg", azm.gim.createOrUpdate[0].rg)
	require.Equal(t, "img_name_gallery", azm.gim.createOrUpdate[0].gallery)
	require.Equal(t, "img-name-img", azm.gim.createOrUpdate[0].name)
	require.Equal(t, armcompute.GalleryImage{
		Location: common.ToPtr("test-universe"),
		Properties: &armcompute.GalleryImageProperties{
			Identifier: &armcompute.GalleryImageIdentifier{
				Publisher: common.ToPtr("image-builder"),
				Offer:     common.ToPtr("image-builder"),
				SKU:       common.ToPtr("IB-SKU-img-name-img"),
			},
			Architecture:     common.ToPtr(armcompute.ArchitectureArm64),
			HyperVGeneration: common.ToPtr(armcompute.HyperVGenerationV2),
			OSType:           common.ToPtr(armcompute.OperatingSystemTypesLinux),
			OSState:          common.ToPtr(armcompute.OperatingSystemStateTypesGeneralized),
		},
	}, azm.gim.createOrUpdate[0].image)

	require.Len(t, azm.im.createOrUpdate, 1)
	require.Equal(t, "rg", azm.im.createOrUpdate[0].rg)
	require.Equal(t, "img-name-mimg", azm.im.createOrUpdate[0].name)
	require.Equal(t, armcompute.Image{
		Properties: &armcompute.ImageProperties{
			HyperVGeneration:     common.ToPtr(armcompute.HyperVGenerationTypesV2),
			SourceVirtualMachine: nil,
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:  common.ToPtr(armcompute.OperatingSystemTypesLinux),
					BlobURI: common.ToPtr("https://storacc.blob.core.windows.net/storcontainer/blobname"),
					OSState: common.ToPtr(armcompute.OperatingSystemStateTypesGeneralized),
				},
			},
		},
		Location: common.ToPtr("test-universe"),
	}, azm.im.createOrUpdate[0].img)

	require.Len(t, azm.givm.createOrUpdate, 1)
	require.Equal(t, "rg", azm.givm.createOrUpdate[0].rg)
	require.Equal(t, "img_name_gallery", azm.givm.createOrUpdate[0].gallery)
	require.Equal(t, "img-name-img", azm.givm.createOrUpdate[0].img)
	require.Equal(t, "1.0.0", azm.givm.createOrUpdate[0].name)
	require.Equal(t, armcompute.GalleryImageVersion{
		Location: common.ToPtr("test-universe"),
		Properties: &armcompute.GalleryImageVersionProperties{
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
				TargetRegions: []*armcompute.TargetRegion{
					{
						Name: common.ToPtr("test-universe"),
					},
				},
			},
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				Source: &armcompute.GalleryArtifactVersionFullSource{
					ID: common.ToPtr("/subscriptions/test-subscription/resourceGroups/rg/providers/Microsoft.Compute/images/img-name-mimg"),
				},
			},
		},
	}, azm.givm.createOrUpdate[0].version)

	err = azm.az.DeleteGalleryImage(t.Context(), gi)
	require.NoError(t, err)

	require.Len(t, azm.givm.delete, 1)
	require.Equal(t, "rg", azm.givm.delete[0].rg)
	require.Equal(t, "img_name_gallery", azm.givm.delete[0].gallery)
	require.Equal(t, "img-name-img", azm.givm.delete[0].image)
	require.Equal(t, "1.0.0", azm.givm.delete[0].name)

	require.Len(t, azm.gim.delete, 1)
	require.Equal(t, "rg", azm.gim.delete[0].rg)
	require.Equal(t, "img_name_gallery", azm.gim.delete[0].gallery)
	require.Equal(t, "img-name-img", azm.gim.delete[0].name)

	require.Len(t, azm.gm.delete, 1)
	require.Equal(t, "rg", azm.gm.delete[0].rg)
	require.Equal(t, "img_name_gallery", azm.gm.delete[0].name)

	require.Len(t, azm.im.delete, 1)
	require.Equal(t, "rg", azm.im.delete[0].rg)
	require.Equal(t, "img-name-mimg", azm.im.delete[0].name)
}
