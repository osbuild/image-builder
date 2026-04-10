package azure_test

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/cloud/azure"
)

type azm struct {
	az    *azure.Client
	rcm   *resourcesMock
	rgm   *resourceGroupsMock
	acm   *accountsMock
	im    *imagesMock
	vnetm *vnetMock
	snm   *subnetMock
	pipm  *pipMock
	sgm   *sgMock
	intfm *intfMock
	vmm   *vmMock
	diskm *diskMock
	gm    *galleriesMock
	gim   *galleryImagesMock
	givm  *galleryImageVersionsMock
}

func newAZ() azm {
	rcm := &resourcesMock{}
	rgm := &resourceGroupsMock{}
	acm := &accountsMock{}
	im := &imagesMock{}
	vnetm := &vnetMock{}
	snm := &subnetMock{}
	pipm := &pipMock{}
	sgm := &sgMock{}
	intfm := &intfMock{}
	vmm := &vmMock{}
	diskm := &diskMock{}
	gm := &galleriesMock{}
	gim := &galleryImagesMock{}
	givm := &galleryImageVersionsMock{}
	return azm{
		azure.NewTestclient(rcm, rgm, acm, im, vnetm, snm, pipm, sgm, intfm, vmm, diskm, gm, gim, givm),
		rcm,
		rgm,
		acm,
		im,
		vnetm,
		snm,
		pipm,
		sgm,
		intfm,
		vmm,
		diskm,
		gm,
		gim,
		givm,
	}
}

func TestGetResourceNameByTag(t *testing.T) {
	azm := newAZ()

	res, err := azm.az.GetResourceNameByTag(t.Context(), "rg", azure.Tag{
		Name:  "tag name",
		Value: "tag value",
	})
	require.NoError(t, err)
	require.Equal(t, "storage-account", res)

	require.Len(t, azm.rcm.list, 1)
	require.Equal(t, "rg", azm.rcm.list[0].rg)
	require.Equal(t, "tagName eq 'tag name' and tagValue eq 'tag value'", common.ValueOrEmpty(azm.rcm.list[0].options.Filter))
}

func TestCreateStorageAccount(t *testing.T) {
	azm := newAZ()
	err := azm.az.CreateStorageAccount(t.Context(), "rg", "name", "loc", azure.Tag{
		Name:  "tag name",
		Value: "tag value",
	})
	require.NoError(t, err)

	require.Len(t, azm.acm.beginCreate, 1)
	require.Equal(t, "rg", azm.acm.beginCreate[0].rg)
	require.Equal(t, "name", azm.acm.beginCreate[0].account)
	require.Equal(t, "loc", common.ValueOrEmpty(azm.acm.beginCreate[0].params.Location))
	require.Equal(t, "tag value", common.ValueOrEmpty(azm.acm.beginCreate[0].params.Tags["tag name"]))

	require.Len(t, azm.rgm.get, 0)
}

func TestCreateStorageAccountEmptyLocation(t *testing.T) {
	azm := newAZ()
	err := azm.az.CreateStorageAccount(t.Context(), "rg", "name", "", azure.Tag{
		Name:  "tag name",
		Value: "tag value",
	})
	require.NoError(t, err)

	require.Len(t, azm.rgm.get, 1)
	require.Equal(t, "rg", azm.rgm.get[0].rg)
	require.Nil(t, azm.rgm.get[0].options)

	require.Len(t, azm.acm.beginCreate, 1)
	require.Equal(t, "rg", azm.acm.beginCreate[0].rg)
	require.Equal(t, "name", azm.acm.beginCreate[0].account)
	require.Equal(t, "test-universe", common.ValueOrEmpty(azm.acm.beginCreate[0].params.Location))
	require.Equal(t, "tag value", common.ValueOrEmpty(azm.acm.beginCreate[0].params.Tags["tag name"]))
}

func TestGetResourceGroups(t *testing.T) {
	azm := newAZ()
	group, err := azm.az.GetResourceGroupLocation(t.Context(), "group-test")
	require.NoError(t, err)
	require.Equal(t, "test-universe", group)

	require.Len(t, azm.rgm.get, 1)
	require.Equal(t, "group-test", azm.rgm.get[0].rg)
	require.Nil(t, azm.rgm.get[0].options)
}

func TestGetStorageAccountKey(t *testing.T) {
	azm := newAZ()
	acc, err := azm.az.GetStorageAccountKey(t.Context(), "rg", "storacc")
	require.NoError(t, err)
	require.Equal(t, "real key", acc)

	require.Len(t, azm.acm.listKeys, 1)
	require.Equal(t, "storacc", azm.acm.listKeys[0].account)
	require.Equal(t, "rg", azm.acm.listKeys[0].rg)
}

func TestRegisterImage(t *testing.T) {
	azm := newAZ()
	err := azm.az.RegisterImage(t.Context(), "rg", "storacc", "storcontainer", "blobname", "imgname", "", azure.HyperVGenV2)
	require.NoError(t, err)

	// resolving the empty location
	require.Len(t, azm.rgm.get, 1)

	require.Len(t, azm.im.createOrUpdate, 1)
	require.Equal(t, "rg", azm.im.createOrUpdate[0].rg)
	require.Equal(t, "imgname", azm.im.createOrUpdate[0].name)
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
}

func TestDeleteImage(t *testing.T) {
	azm := newAZ()
	err := azm.az.DeleteImage(t.Context(), "rg", "imgname")
	require.NoError(t, err)

	require.Len(t, azm.im.delete, 1)
	require.Equal(t, imDeleteArgs{
		rg:   "rg",
		name: "imgname",
	}, azm.im.delete[0])
}
