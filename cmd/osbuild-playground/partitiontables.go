package main

import "github.com/osbuild/image-builder/pkg/disk"

var basePT = disk.PartitionTable{
	UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
	Type: disk.PT_GPT,
	Partitions: []disk.Partition{
		{
			Size:     1048576, // 1MB
			Bootable: true,
			Type:     disk.BIOSBootPartitionGUID,
			UUID:     disk.BIOSBootPartitionUUID,
		},
		{
			Size: 209715200, // 200 MB
			Type: disk.EFISystemPartitionGUID,
			UUID: disk.EFISystemPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "vfat",
				UUID:         disk.EFIFilesystemUUID,
				Mountpoint:   "/boot/efi",
				Label:        "ESP",
				FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
				FSTabFreq:    0,
				FSTabPassNo:  2,
			},
		},
		{
			Size: 524288000, // 500 MB
			Type: disk.FilesystemDataGUID,
			UUID: disk.DataPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "ext4",
				Mountpoint:   "/boot",
				Label:        "boot",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		},
		{
			Size: 2147483648, // 2GiB
			Type: disk.FilesystemDataGUID,
			UUID: disk.RootPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "ext4",
				Label:        "root",
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		},
	},
}
