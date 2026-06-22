package azure_test

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"

	"github.com/osbuild/image-builder/v73/internal/common"
)

type mockPollerHandler[T any] struct {
	result *T
}

func (mp *mockPollerHandler[T]) Done() bool {
	return true
}

func (mp *mockPollerHandler[T]) Poll(ctx context.Context) (*http.Response, error) {
	return nil, nil
}

func (mp *mockPollerHandler[T]) Result(ctx context.Context, out *T) error {
	return nil
}

func makePoller[T any](result *T) (*runtime.Poller[T], error) {
	return runtime.NewPoller(
		&http.Response{},
		runtime.NewPipeline("", "", runtime.PipelineOptions{}, nil),
		&runtime.NewPollerOptions[T]{
			Handler: &mockPollerHandler[T]{
				result: result,
			},
			Response: result,
		},
	)
}

type resourcesMock struct {
	list []rmListArgs
}

type rmListArgs struct {
	rg      string
	options *armresources.ClientListByResourceGroupOptions
}

func (rm *resourcesMock) NewListByResourceGroupPager(
	rg string,
	options *armresources.ClientListByResourceGroupOptions) *runtime.Pager[armresources.ClientListByResourceGroupResponse] {
	rm.list = append(rm.list, rmListArgs{rg, options})

	return runtime.NewPager(
		runtime.PagingHandler[armresources.ClientListByResourceGroupResponse]{
			More: func(current armresources.ClientListByResourceGroupResponse) bool {
				return false
			},
			Fetcher: func(ctx context.Context, current *armresources.ClientListByResourceGroupResponse) (armresources.ClientListByResourceGroupResponse, error) {
				return armresources.ClientListByResourceGroupResponse{
					ResourceListResult: armresources.ResourceListResult{
						Value: []*armresources.GenericResourceExpanded{
							&armresources.GenericResourceExpanded{
								Name: common.ToPtr("storage-account"),
							},
						},
					},
				}, nil
			},
		},
	)
}

type resourceGroupsMock struct {
	get []rgmGetArgs
}

type rgmGetArgs struct {
	rg      string
	options *armresources.ResourceGroupsClientGetOptions
}

func (rgm *resourceGroupsMock) Get(
	ctx context.Context,
	rg string,
	options *armresources.ResourceGroupsClientGetOptions) (armresources.ResourceGroupsClientGetResponse, error) {
	rgm.get = append(rgm.get, rgmGetArgs{rg, options})

	return armresources.ResourceGroupsClientGetResponse{
		ResourceGroup: armresources.ResourceGroup{
			Location: common.ToPtr("test-universe"),
		},
	}, nil
}

type accountsMock struct {
	beginCreate []acmBeginCreateArgs
	listKeys    []acmListKeysArgs
}

type acmBeginCreateArgs struct {
	rg      string
	account string
	params  armstorage.AccountCreateParameters
	options *armstorage.AccountsClientBeginCreateOptions
}

func (acm *accountsMock) BeginCreate(
	ctx context.Context,
	rg string,
	account string,
	params armstorage.AccountCreateParameters,
	options *armstorage.AccountsClientBeginCreateOptions) (*runtime.Poller[armstorage.AccountsClientCreateResponse], error) {
	acm.beginCreate = append(acm.beginCreate, acmBeginCreateArgs{rg, account, params, options})

	return makePoller[armstorage.AccountsClientCreateResponse](
		&armstorage.AccountsClientCreateResponse{
			Account: armstorage.Account{
				Name: &account,
			},
		},
	)
}

type acmListKeysArgs struct {
	rg      string
	account string
	options *armstorage.AccountsClientListKeysOptions
}

func (acm *accountsMock) ListKeys(
	ctx context.Context,
	rg string,
	account string,
	options *armstorage.AccountsClientListKeysOptions) (armstorage.AccountsClientListKeysResponse, error) {
	acm.listKeys = append(acm.listKeys, acmListKeysArgs{rg, account, options})

	return armstorage.AccountsClientListKeysResponse{
		AccountListKeysResult: armstorage.AccountListKeysResult{
			Keys: []*armstorage.AccountKey{
				&armstorage.AccountKey{
					Value: common.ToPtr("real key"),
				},
			},
		},
	}, nil
}

type imagesMock struct {
	createOrUpdate []imBeginCreateOrUpdateArgs
	delete         []imDeleteArgs
}

type imBeginCreateOrUpdateArgs struct {
	rg      string
	name    string
	img     armcompute.Image
	options *armcompute.ImagesClientBeginCreateOrUpdateOptions
}

type imDeleteArgs struct {
	rg      string
	name    string
	options *armcompute.ImagesClientBeginDeleteOptions
}

func (im *imagesMock) BeginCreateOrUpdate(ctx context.Context, rg string, name string, img armcompute.Image, options *armcompute.ImagesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.ImagesClientCreateOrUpdateResponse], error) {
	im.createOrUpdate = append(im.createOrUpdate, imBeginCreateOrUpdateArgs{rg, name, img, options})

	return makePoller[armcompute.ImagesClientCreateOrUpdateResponse](
		&armcompute.ImagesClientCreateOrUpdateResponse{
			Image: img,
		},
	)
}

func (im *imagesMock) BeginDelete(ctx context.Context, rg string, name string, options *armcompute.ImagesClientBeginDeleteOptions) (*runtime.Poller[armcompute.ImagesClientDeleteResponse], error) {
	im.delete = append(im.delete, imDeleteArgs{rg, name, options})

	return makePoller[armcompute.ImagesClientDeleteResponse](
		&armcompute.ImagesClientDeleteResponse{},
	)
}

type vnetMock struct {
	createOrUpdate []vnetCreateOrUpdateArgs
	delete         []vnetDeleteArgs
}

type vnetCreateOrUpdateArgs struct {
	rg      string
	name    string
	vnet    armnetwork.VirtualNetwork
	options *armnetwork.VirtualNetworksClientBeginCreateOrUpdateOptions
}

type vnetDeleteArgs struct {
	rg      string
	name    string
	options *armnetwork.VirtualNetworksClientBeginDeleteOptions
}

func (vnetm *vnetMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, vnet armnetwork.VirtualNetwork, options *armnetwork.VirtualNetworksClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse], error) {
	vnetm.createOrUpdate = append(vnetm.createOrUpdate, vnetCreateOrUpdateArgs{rg, name, vnet, options})

	vnet.ID = common.ToPtr("vnet-id")
	vnet.Name = &name
	return makePoller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse](
		&armnetwork.VirtualNetworksClientCreateOrUpdateResponse{
			VirtualNetwork: vnet,
		},
	)
}

func (vnetm *vnetMock) BeginDelete(ctx context.Context, rg, name string, options *armnetwork.VirtualNetworksClientBeginDeleteOptions) (*runtime.Poller[armnetwork.VirtualNetworksClientDeleteResponse], error) {
	vnetm.delete = append(vnetm.delete, vnetDeleteArgs{rg, name, options})
	return makePoller[armnetwork.VirtualNetworksClientDeleteResponse](
		&armnetwork.VirtualNetworksClientDeleteResponse{},
	)
}

type subnetMock struct {
	createOrUpdate []subnetCreateOrUpdateArgs
	delete         []subnetDeleteArgs
}

type subnetCreateOrUpdateArgs struct {
	rg      string
	vnet    string
	name    string
	subnet  armnetwork.Subnet
	options *armnetwork.SubnetsClientBeginCreateOrUpdateOptions
}

type subnetDeleteArgs struct {
	rg      string
	vnet    string
	name    string
	options *armnetwork.SubnetsClientBeginDeleteOptions
}

func (snm *subnetMock) BeginCreateOrUpdate(ctx context.Context, rg, vnet, name string, sn armnetwork.Subnet, options *armnetwork.SubnetsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SubnetsClientCreateOrUpdateResponse], error) {
	snm.createOrUpdate = append(snm.createOrUpdate, subnetCreateOrUpdateArgs{rg, vnet, name, sn, options})

	sn.ID = common.ToPtr("sn-id")
	sn.Name = &name
	return makePoller[armnetwork.SubnetsClientCreateOrUpdateResponse](
		&armnetwork.SubnetsClientCreateOrUpdateResponse{
			Subnet: sn,
		},
	)
}

func (snm *subnetMock) BeginDelete(ctx context.Context, rg, vnet, name string, options *armnetwork.SubnetsClientBeginDeleteOptions) (*runtime.Poller[armnetwork.SubnetsClientDeleteResponse], error) {
	snm.delete = append(snm.delete, subnetDeleteArgs{rg, vnet, name, options})
	return makePoller[armnetwork.SubnetsClientDeleteResponse](
		&armnetwork.SubnetsClientDeleteResponse{},
	)
}

type pipMock struct {
	createOrUpdate []pipCreateOrUpdateArgs
	delete         []pipDeleteArgs
}

type pipCreateOrUpdateArgs struct {
	rg      string
	name    string
	pip     armnetwork.PublicIPAddress
	options *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions
}

type pipDeleteArgs struct {
	rg      string
	name    string
	options *armnetwork.PublicIPAddressesClientBeginDeleteOptions
}

func (pipm *pipMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, pip armnetwork.PublicIPAddress, options *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error) {
	pipm.createOrUpdate = append(pipm.createOrUpdate, pipCreateOrUpdateArgs{rg, name, pip, options})

	return makePoller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse](
		&armnetwork.PublicIPAddressesClientCreateOrUpdateResponse{
			PublicIPAddress: armnetwork.PublicIPAddress{
				Location: pip.Location,
				ID:       common.ToPtr("pip-id"),
				Name:     &name,
				Properties: &armnetwork.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: pip.Properties.PublicIPAllocationMethod,
					IPAddress:                common.ToPtr("0.0.0.0"),
				},
			},
		},
	)
}

func (pipm *pipMock) BeginDelete(ctx context.Context, rg, name string, options *armnetwork.PublicIPAddressesClientBeginDeleteOptions) (*runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse], error) {
	pipm.delete = append(pipm.delete, pipDeleteArgs{rg, name, options})
	return makePoller[armnetwork.PublicIPAddressesClientDeleteResponse](
		&armnetwork.PublicIPAddressesClientDeleteResponse{},
	)
}

type sgMock struct {
	createOrUpdate []sgCreateOrUpdateArgs
	delete         []sgDeleteArgs
}

type sgCreateOrUpdateArgs struct {
	rg      string
	name    string
	sg      armnetwork.SecurityGroup
	options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions
}

type sgDeleteArgs struct {
	rg      string
	name    string
	options *armnetwork.SecurityGroupsClientBeginDeleteOptions
}

func (sgm *sgMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, sg armnetwork.SecurityGroup, options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse], error) {
	sgm.createOrUpdate = append(sgm.createOrUpdate, sgCreateOrUpdateArgs{rg, name, sg, options})

	sg.ID = common.ToPtr("sg-id")
	sg.Name = &name
	return makePoller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse](
		&armnetwork.SecurityGroupsClientCreateOrUpdateResponse{
			SecurityGroup: sg,
		},
	)
}

func (sgm *sgMock) BeginDelete(ctx context.Context, rg, name string, options *armnetwork.SecurityGroupsClientBeginDeleteOptions) (*runtime.Poller[armnetwork.SecurityGroupsClientDeleteResponse], error) {
	sgm.delete = append(sgm.delete, sgDeleteArgs{rg, name, options})
	return makePoller[armnetwork.SecurityGroupsClientDeleteResponse](
		&armnetwork.SecurityGroupsClientDeleteResponse{},
	)
}

type intfMock struct {
	createOrUpdate []intfCreateOrUpdateArgs
	delete         []intfDeleteArgs
}

type intfCreateOrUpdateArgs struct {
	rg      string
	name    string
	intf    armnetwork.Interface
	options *armnetwork.InterfacesClientBeginCreateOrUpdateOptions
}

type intfDeleteArgs struct {
	rg      string
	name    string
	options *armnetwork.InterfacesClientBeginDeleteOptions
}

func (intfm *intfMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, intf armnetwork.Interface, options *armnetwork.InterfacesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.InterfacesClientCreateOrUpdateResponse], error) {
	intfm.createOrUpdate = append(intfm.createOrUpdate, intfCreateOrUpdateArgs{rg, name, intf, options})

	intf.ID = common.ToPtr("intf-id")
	intf.Name = &name
	return makePoller[armnetwork.InterfacesClientCreateOrUpdateResponse](
		&armnetwork.InterfacesClientCreateOrUpdateResponse{
			Interface: intf,
		},
	)
}

func (intfm *intfMock) BeginDelete(ctx context.Context, rg, name string, options *armnetwork.InterfacesClientBeginDeleteOptions) (*runtime.Poller[armnetwork.InterfacesClientDeleteResponse], error) {
	intfm.delete = append(intfm.delete, intfDeleteArgs{rg, name, options})
	return makePoller[armnetwork.InterfacesClientDeleteResponse](
		&armnetwork.InterfacesClientDeleteResponse{},
	)
}

type vmMock struct {
	createOrUpdate []vmCreateOrUpdateArgs
	delete         []vmDeleteArgs
}

type vmCreateOrUpdateArgs struct {
	rg      string
	name    string
	vm      armcompute.VirtualMachine
	options *armcompute.VirtualMachinesClientBeginCreateOrUpdateOptions
}

type vmDeleteArgs struct {
	rg      string
	name    string
	options *armcompute.VirtualMachinesClientBeginDeleteOptions
}

func (vmm *vmMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, vm armcompute.VirtualMachine, options *armcompute.VirtualMachinesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse], error) {
	vmm.createOrUpdate = append(vmm.createOrUpdate, vmCreateOrUpdateArgs{rg, name, vm, options})

	vm.ID = common.ToPtr("vm-id")
	vm.Name = &name
	return makePoller[armcompute.VirtualMachinesClientCreateOrUpdateResponse](
		&armcompute.VirtualMachinesClientCreateOrUpdateResponse{
			VirtualMachine: vm,
		},
	)
}

func (vmm *vmMock) BeginDelete(ctx context.Context, rg, name string, options *armcompute.VirtualMachinesClientBeginDeleteOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeleteResponse], error) {
	vmm.delete = append(vmm.delete, vmDeleteArgs{rg, name, options})
	return makePoller[armcompute.VirtualMachinesClientDeleteResponse](
		&armcompute.VirtualMachinesClientDeleteResponse{},
	)
}

type diskMock struct {
	createOrUpdate []diskCreateOrUpdateArgs
	delete         []diskDeleteArgs
}

type diskCreateOrUpdateArgs struct {
	rg      string
	name    string
	disk    armcompute.Disk
	options *armcompute.DisksClientBeginCreateOrUpdateOptions
}

type diskDeleteArgs struct {
	rg      string
	name    string
	options *armcompute.DisksClientBeginDeleteOptions
}

func (diskm *diskMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, disk armcompute.Disk, options *armcompute.DisksClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.DisksClientCreateOrUpdateResponse], error) {
	diskm.createOrUpdate = append(diskm.createOrUpdate, diskCreateOrUpdateArgs{rg, name, disk, options})
	disk.Name = &name
	return makePoller[armcompute.DisksClientCreateOrUpdateResponse](
		&armcompute.DisksClientCreateOrUpdateResponse{
			Disk: disk,
		},
	)
}

func (diskm *diskMock) BeginDelete(ctx context.Context, rg, name string, options *armcompute.DisksClientBeginDeleteOptions) (*runtime.Poller[armcompute.DisksClientDeleteResponse], error) {
	diskm.delete = append(diskm.delete, diskDeleteArgs{rg, name, options})
	return makePoller[armcompute.DisksClientDeleteResponse](
		&armcompute.DisksClientDeleteResponse{},
	)
}

type galleriesMock struct {
	createOrUpdate []galleriesCreateOrUpdateArgs
	delete         []galleriesDeleteArgs
}

type galleriesCreateOrUpdateArgs struct {
	rg      string
	name    string
	gallery armcompute.Gallery
	options *armcompute.GalleriesClientBeginCreateOrUpdateOptions
}

type galleriesDeleteArgs struct {
	rg      string
	name    string
	options *armcompute.GalleriesClientBeginDeleteOptions
}

func (gm *galleriesMock) BeginCreateOrUpdate(ctx context.Context, rg, name string, gallery armcompute.Gallery, options *armcompute.GalleriesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.GalleriesClientCreateOrUpdateResponse], error) {
	gm.createOrUpdate = append(gm.createOrUpdate, galleriesCreateOrUpdateArgs{rg, name, gallery, options})
	gallery.Name = &name
	return makePoller[armcompute.GalleriesClientCreateOrUpdateResponse](
		&armcompute.GalleriesClientCreateOrUpdateResponse{
			Gallery: gallery,
		},
	)
}

func (gm *galleriesMock) BeginDelete(ctx context.Context, rg, name string, options *armcompute.GalleriesClientBeginDeleteOptions) (*runtime.Poller[armcompute.GalleriesClientDeleteResponse], error) {
	gm.delete = append(gm.delete, galleriesDeleteArgs{rg, name, options})
	return makePoller[armcompute.GalleriesClientDeleteResponse](
		&armcompute.GalleriesClientDeleteResponse{},
	)
}

type galleryImagesMock struct {
	createOrUpdate []galleryImagesCreateOrUpdateArgs
	delete         []galleryImagesDeleteArgs
}

type galleryImagesCreateOrUpdateArgs struct {
	rg      string
	gallery string
	name    string
	image   armcompute.GalleryImage
	options *armcompute.GalleryImagesClientBeginCreateOrUpdateOptions
}

type galleryImagesDeleteArgs struct {
	rg      string
	gallery string
	name    string
	options *armcompute.GalleryImagesClientBeginDeleteOptions
}

func (gim *galleryImagesMock) BeginCreateOrUpdate(ctx context.Context, rg, gallery, name string, image armcompute.GalleryImage, options *armcompute.GalleryImagesClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.GalleryImagesClientCreateOrUpdateResponse], error) {
	gim.createOrUpdate = append(gim.createOrUpdate, galleryImagesCreateOrUpdateArgs{rg, gallery, name, image, options})
	image.Name = &name
	return makePoller[armcompute.GalleryImagesClientCreateOrUpdateResponse](
		&armcompute.GalleryImagesClientCreateOrUpdateResponse{
			GalleryImage: image,
		},
	)
}

func (gim *galleryImagesMock) BeginDelete(ctx context.Context, rg, gallery, name string, options *armcompute.GalleryImagesClientBeginDeleteOptions) (*runtime.Poller[armcompute.GalleryImagesClientDeleteResponse], error) {
	gim.delete = append(gim.delete, galleryImagesDeleteArgs{rg, gallery, name, options})
	return makePoller[armcompute.GalleryImagesClientDeleteResponse](
		&armcompute.GalleryImagesClientDeleteResponse{},
	)
}

type galleryImageVersionsMock struct {
	createOrUpdate []galleryImageVersionsCreateOrUpdateArgs
	delete         []galleryImageVersionsDeleteArgs
}

type galleryImageVersionsCreateOrUpdateArgs struct {
	rg      string
	gallery string
	img     string
	name    string
	version armcompute.GalleryImageVersion
	options *armcompute.GalleryImageVersionsClientBeginCreateOrUpdateOptions
}

type galleryImageVersionsDeleteArgs struct {
	rg      string
	gallery string
	image   string
	name    string
	options *armcompute.GalleryImageVersionsClientBeginDeleteOptions
}

func (givm *galleryImageVersionsMock) BeginCreateOrUpdate(ctx context.Context, rg, gallery, img, name string, version armcompute.GalleryImageVersion, options *armcompute.GalleryImageVersionsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.GalleryImageVersionsClientCreateOrUpdateResponse], error) {
	givm.createOrUpdate = append(givm.createOrUpdate, galleryImageVersionsCreateOrUpdateArgs{rg, gallery, img, name, version, options})
	version.Name = &name
	return makePoller[armcompute.GalleryImageVersionsClientCreateOrUpdateResponse](
		&armcompute.GalleryImageVersionsClientCreateOrUpdateResponse{
			GalleryImageVersion: version,
		},
	)
}

func (givm *galleryImageVersionsMock) BeginDelete(ctx context.Context, rg, gallery, img, name string, options *armcompute.GalleryImageVersionsClientBeginDeleteOptions) (*runtime.Poller[armcompute.GalleryImageVersionsClientDeleteResponse], error) {
	givm.delete = append(givm.delete, galleryImageVersionsDeleteArgs{rg, gallery, img, name, options})
	return makePoller[armcompute.GalleryImageVersionsClientDeleteResponse](
		&armcompute.GalleryImageVersionsClientDeleteResponse{},
	)
}
