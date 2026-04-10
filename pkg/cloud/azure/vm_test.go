package azure_test

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/cloud/azure"
)

func TestCreateLinuxVM(t *testing.T) {
	azm := newAZ()

	vm, err := azm.az.CreateVM(
		t.Context(),
		"rg",
		azure.VMOptions{
			Name:   "vm-name",
			Image:  "test-image",
			Size:   "size",
			User:   "username",
			SSHKey: "ssh-key",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "rg", vm.ResourceGroup)

	require.Equal(t, "vm-name", vm.Name)
	require.Len(t, azm.vmm.createOrUpdate, 1)
	require.Equal(t,
		vmCreateOrUpdateArgs{
			name: "vm-name",
			rg:   "rg",
			vm: armcompute.VirtualMachine{
				Location: common.ToPtr("test-universe"),
				Identity: &armcompute.VirtualMachineIdentity{
					Type: common.ToPtr(armcompute.ResourceIdentityTypeNone),
				},
				Properties: &armcompute.VirtualMachineProperties{
					StorageProfile: &armcompute.StorageProfile{
						ImageReference: &armcompute.ImageReference{
							ID: common.ToPtr("test-image"),
						},
						OSDisk: &armcompute.OSDisk{
							Name:         &vm.DiskName,
							CreateOption: common.ToPtr(armcompute.DiskCreateOptionTypesFromImage),
							Caching:      common.ToPtr(armcompute.CachingTypesReadWrite),
							ManagedDisk: &armcompute.ManagedDiskParameters{
								StorageAccountType: common.ToPtr(armcompute.StorageAccountTypesStandardLRS),
							},
						},
					},
					HardwareProfile: &armcompute.HardwareProfile{
						VMSize: common.ToPtr(armcompute.VirtualMachineSizeTypes("size")),
					},
					OSProfile: &armcompute.OSProfile{
						ComputerName:  common.ToPtr(vm.Name),
						AdminUsername: common.ToPtr("username"),
						LinuxConfiguration: &armcompute.LinuxConfiguration{
							DisablePasswordAuthentication: common.ToPtr(true),
							SSH: &armcompute.SSHConfiguration{
								PublicKeys: []*armcompute.SSHPublicKey{
									{
										Path:    common.ToPtr("/home/username/.ssh/authorized_keys"),
										KeyData: common.ToPtr("ssh-key"),
									},
								},
							},
						},
					},
					NetworkProfile: &armcompute.NetworkProfile{
						NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
							{
								ID: common.ToPtr("intf-id"),
							},
						},
					},
				},
			},
		}, azm.vmm.createOrUpdate[0])

	require.Equal(t, "vm-name-intf", vm.Nic)
	require.Len(t, azm.intfm.createOrUpdate, 1)
	require.Equal(t, intfCreateOrUpdateArgs{
		rg:   "rg",
		name: vm.Nic,
		intf: armnetwork.Interface{
			Location: common.ToPtr("test-universe"),
			Properties: &armnetwork.InterfacePropertiesFormat{
				IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
					{
						Name: common.ToPtr("ipConfig"),
						Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAllocationMethod: common.ToPtr(armnetwork.IPAllocationMethodDynamic),
							Subnet: &armnetwork.Subnet{
								ID: common.ToPtr("sn-id"),
							},
							PublicIPAddress: &armnetwork.PublicIPAddress{
								ID: common.ToPtr("pip-id"),
							},
						},
					},
				},
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: common.ToPtr("sg-id"),
				},
			},
		},
	}, azm.intfm.createOrUpdate[0])

	require.Equal(t, "vm-name-sg", vm.SG)
	require.Len(t, azm.sgm.createOrUpdate, 1)
	require.Equal(t, sgCreateOrUpdateArgs{
		rg:   "rg",
		name: vm.SG,
		sg: armnetwork.SecurityGroup{
			Location: common.ToPtr("test-universe"),
			Properties: &armnetwork.SecurityGroupPropertiesFormat{
				SecurityRules: []*armnetwork.SecurityRule{
					{
						Name: common.ToPtr("ssh"),
						Properties: &armnetwork.SecurityRulePropertiesFormat{
							SourceAddressPrefix:      common.ToPtr("*"),
							SourcePortRange:          common.ToPtr("*"),
							DestinationAddressPrefix: common.ToPtr("*"),
							DestinationPortRange:     common.ToPtr("22"),
							Protocol:                 common.ToPtr(armnetwork.SecurityRuleProtocolTCP),
							Access:                   common.ToPtr(armnetwork.SecurityRuleAccessAllow),
							Priority:                 common.ToPtr[int32](100),
							Description:              common.ToPtr("ssh"),
							Direction:                common.ToPtr(armnetwork.SecurityRuleDirectionInbound),
						},
					},
				},
			},
		},
	}, azm.sgm.createOrUpdate[0])

	require.Equal(t, "vm-name-ip", vm.IPName)
	require.Equal(t, "0.0.0.0", vm.IPAddress)
	require.Len(t, azm.pipm.createOrUpdate, 1)
	require.Equal(t, pipCreateOrUpdateArgs{
		rg:   "rg",
		name: vm.IPName,
		pip: armnetwork.PublicIPAddress{
			Location: common.ToPtr("test-universe"),
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: common.ToPtr(armnetwork.IPAllocationMethodStatic),
			},
		},
	}, azm.pipm.createOrUpdate[0])

	require.Equal(t, "vm-name-subnet", vm.Subnet)
	require.Len(t, azm.snm.createOrUpdate, 1)
	require.Equal(t, subnetCreateOrUpdateArgs{
		rg:   "rg",
		name: vm.Subnet,
		vnet: vm.VNet,
		subnet: armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: common.ToPtr("10.1.10.0/24"),
			},
		},
	}, azm.snm.createOrUpdate[0])

	require.Equal(t, "vm-name-vnet", vm.VNet)
	require.Len(t, azm.vnetm.createOrUpdate, 1)
	require.Equal(t, vnetCreateOrUpdateArgs{
		rg:   "rg",
		name: vm.VNet,
		vnet: armnetwork.VirtualNetwork{
			Location: common.ToPtr("test-universe"),
			Properties: &armnetwork.VirtualNetworkPropertiesFormat{
				AddressSpace: &armnetwork.AddressSpace{
					AddressPrefixes: []*string{
						common.ToPtr("10.1.0.0/16"),
					},
				},
			},
		},
	}, azm.vnetm.createOrUpdate[0])

	require.Equal(t, "vm-name-disk", vm.DiskName)
	require.Empty(t, vm.DiskID)

	require.NoError(t, azm.az.DestroyVM(t.Context(), vm))
	require.Len(t, azm.vmm.delete, 1)
	require.Equal(t, vmDeleteArgs{
		rg:   "rg",
		name: vm.Name,
	}, azm.vmm.delete[0])

	require.Len(t, azm.diskm.delete, 1)
	require.Equal(t, diskDeleteArgs{
		rg:   "rg",
		name: vm.DiskName,
	}, azm.diskm.delete[0])

	require.Len(t, azm.intfm.delete, 1)
	require.Equal(t, intfDeleteArgs{
		rg:   "rg",
		name: vm.Nic,
	}, azm.intfm.delete[0])

	require.Len(t, azm.sgm.delete, 1)
	require.Equal(t, sgDeleteArgs{
		rg:   "rg",
		name: vm.SG,
	}, azm.sgm.delete[0])

	require.Len(t, azm.pipm.delete, 1)
	require.Equal(t, pipDeleteArgs{
		rg:   "rg",
		name: vm.IPName,
	}, azm.pipm.delete[0])

	require.Len(t, azm.snm.delete, 1)
	require.Equal(t, subnetDeleteArgs{
		rg:   "rg",
		name: vm.Subnet,
		vnet: vm.VNet,
	}, azm.snm.delete[0])

	require.Len(t, azm.vnetm.delete, 1)
	require.Equal(t, vnetDeleteArgs{
		rg:   "rg",
		name: vm.VNet,
	}, azm.vnetm.delete[0])
}

func TestCreateWindowsVM(t *testing.T) {
	azm := newAZ()

	vm, err := azm.az.CreateVM(
		t.Context(),
		"rg",
		azure.VMOptions{
			Name:     "vm-name",
			Size:     "size",
			User:     "username",
			SSHKey:   "ssh-key",
			Snapshot: "snapshot",
			Windows:  true,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "rg", vm.ResourceGroup)

	require.Equal(t, "vm-name", vm.Name)
	require.Len(t, azm.vmm.createOrUpdate, 1)
	require.Equal(t,
		vmCreateOrUpdateArgs{
			name: "vm-name",
			rg:   "rg",
			vm: armcompute.VirtualMachine{
				Location: common.ToPtr("test-universe"),
				Identity: &armcompute.VirtualMachineIdentity{
					Type: common.ToPtr(armcompute.ResourceIdentityTypeNone),
				},
				Properties: &armcompute.VirtualMachineProperties{
					StorageProfile: &armcompute.StorageProfile{
						OSDisk: &armcompute.OSDisk{
							CreateOption: common.ToPtr(armcompute.DiskCreateOptionTypesAttach),
							OSType:       common.ToPtr(armcompute.OperatingSystemTypesWindows),
							ManagedDisk: &armcompute.ManagedDiskParameters{
								ID: &vm.DiskID,
							},
						},
					},
					HardwareProfile: &armcompute.HardwareProfile{
						VMSize: common.ToPtr(armcompute.VirtualMachineSizeTypes("size")),
					},
					NetworkProfile: &armcompute.NetworkProfile{
						NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
							{
								ID: common.ToPtr("intf-id"),
							},
						},
					},
					SecurityProfile: &armcompute.SecurityProfile{
						SecurityType: common.ToPtr(armcompute.SecurityTypesTrustedLaunch),
					},
				},
			},
		}, azm.vmm.createOrUpdate[0])

	require.Len(t, azm.diskm.createOrUpdate, 1)
	require.Equal(t, diskCreateOrUpdateArgs{
		rg:   "rg",
		name: "vm-name-disk",
		disk: armcompute.Disk{
			Location: common.ToPtr("test-universe"),
			Properties: &armcompute.DiskProperties{
				CreationData: &armcompute.CreationData{
					CreateOption:     common.ToPtr(armcompute.DiskCreateOptionCopy),
					SourceResourceID: common.ToPtr("snapshot"),
				},
			},
		},
	}, azm.diskm.createOrUpdate[0])
}

func TestCreateVMError(t *testing.T) {
	azm := newAZ()

	_, err := azm.az.CreateVM(
		t.Context(),
		"rg",
		azure.VMOptions{
			Name:     "vm-name",
			Image:    "test-image",
			Size:     "size",
			User:     "username",
			Snapshot: "snapshot",
		},
	)
	require.ErrorContains(t, err, "Either an image or a snapshot must be given to create a VM, not both")
}
