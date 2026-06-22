package awscloud_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/cloud/awscloud"
	"github.com/osbuild/image-builder/v73/pkg/platform"
)

func TestRegister(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFilePath := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(tmpFilePath, []byte("test content"), 0600))

	testImageName := "test-image"
	testBucketName := "test-bucket"
	testObjectName := "test-object"

	testCases := []struct {
		testName string

		tags         []awscloud.AWSTag
		shareWith    []string
		architecture arch.Arch
		bootMode     *platform.BootMode
		importRole   *string

		ec2Client  *fakeEC2Client
		s3Client   *fakeS3Client
		s3Uploader *fakeS3Uploader

		snapshotImportedWaiterOutput *ec2.DescribeImportSnapshotTasksOutput
		snapshotImportedWaiterErr    error

		expectErr bool
		errMsg    string
	}{
		{
			testName:     "happy minimal",
			architecture: arch.ARCH_X86_64,
			expectErr:    false,
		},
		{
			testName:     "happy full",
			shareWith:    []string{"123456789012"},
			architecture: arch.ARCH_X86_64,
			bootMode:     common.ToPtr(platform.BOOT_UEFI),
			importRole:   aws.String("arn:aws:iam::123456789012:role/ImportRole"),
			expectErr:    false,
		},
		{
			testName:     "error: invalid architecture",
			architecture: arch.ARCH_S390X, // invalid arch
			expectErr:    true,
			errMsg:       "ec2 doesn't support the following arch: s390x",
		},
		{
			testName:     "error: invalid boot mode",
			architecture: arch.ARCH_X86_64,
			bootMode:     common.ToPtr(platform.BootMode(123456)), // invalid boot mode
			expectErr:    true,
			errMsg:       "ec2 doesn't support the following boot mode: %!s(PANIC=String method: invalid boot mode)",
		},
		{
			testName:     "error: import snapshot failure",
			architecture: arch.ARCH_X86_64,
			ec2Client: &fakeEC2Client{
				importSnapshotErr: fmt.Errorf("import snapshot error"),
			},
			expectErr: true,
			errMsg:    "import snapshot error",
		},
		{
			testName:                     "error: import snapshot waiter failure",
			architecture:                 arch.ARCH_X86_64,
			snapshotImportedWaiterErr:    fmt.Errorf("waiter error"),
			snapshotImportedWaiterOutput: &ec2.DescribeImportSnapshotTasksOutput{},
			expectErr:                    true,
			errMsg:                       "waiter error",
		},
		{
			testName:     "error: import snapshot wait done not completed",
			architecture: arch.ARCH_X86_64,
			snapshotImportedWaiterOutput: &ec2.DescribeImportSnapshotTasksOutput{
				ImportSnapshotTasks: []ec2types.ImportSnapshotTask{
					{
						SnapshotTaskDetail: &ec2types.SnapshotTaskDetail{
							SnapshotId:    aws.String("snap-1234567890abcdef0"),
							Status:        aws.String("pending"),
							StatusMessage: aws.String("Task is still in progress"),
						},
					},
				},
			},
			expectErr: true,
			errMsg:    "Unable to import snapshot, task result: pending, msg: Task is still in progress",
		},
		{
			testName:     "error: delete S3 object failure",
			architecture: arch.ARCH_X86_64,
			s3Client: &fakeS3Client{
				deleteObjectErr: fmt.Errorf("delete object error"),
			},
			expectErr: true,
			errMsg:    "delete object error",
		},
		{
			testName:     "error: create tags failure",
			architecture: arch.ARCH_X86_64,
			ec2Client: &fakeEC2Client{
				importSnapshot: &ec2.ImportSnapshotOutput{
					ImportTaskId: aws.String("import-task-id"),
				},
				registerImage: &ec2.RegisterImageOutput{
					ImageId: aws.String("ami-1234567890abcdef0"),
				},
				createTagsErr: fmt.Errorf("create tags error"),
			},
			expectErr: true,
			errMsg:    "create tags error",
		},
		{
			testName:     "error: register image failure",
			architecture: arch.ARCH_X86_64,
			ec2Client: &fakeEC2Client{
				importSnapshot: &ec2.ImportSnapshotOutput{
					ImportTaskId: aws.String("import-task-id"),
				},
				registerImage: &ec2.RegisterImageOutput{
					ImageId: aws.String("ami-1234567890abcdef0"),
				},
				registerImageErr: fmt.Errorf("register image error"),
			},
			expectErr: true,
			errMsg:    "register image error",
		},
		{
			testName:     "error: share snapshot with accounts failure",
			architecture: arch.ARCH_X86_64,
			shareWith:    []string{"123456789012"},
			ec2Client: &fakeEC2Client{
				importSnapshot: &ec2.ImportSnapshotOutput{
					ImportTaskId: aws.String("import-task-id"),
				},
				registerImage: &ec2.RegisterImageOutput{
					ImageId: aws.String("ami-1234567890abcdef0"),
				},
				modifySnapshotAttributeErr: fmt.Errorf("modify snapshot attribute error"),
			},
			expectErr: true,
			errMsg:    "modify snapshot attribute error",
		},
		{
			testName:     "error: share image with accounts failure",
			architecture: arch.ARCH_X86_64,
			shareWith:    []string{"123456789012"},
			ec2Client: &fakeEC2Client{
				importSnapshot: &ec2.ImportSnapshotOutput{
					ImportTaskId: aws.String("import-task-id"),
				},
				registerImage: &ec2.RegisterImageOutput{
					ImageId: aws.String("ami-1234567890abcdef0"),
				},
				modifyImageAttributeErr: fmt.Errorf("modify image attribute error"),
			},
			expectErr: true,
			errMsg:    "modify image attribute error",
		},
		{
			testName:     "set custom AWS tags",
			architecture: arch.ARCH_X86_64,
			tags: []awscloud.AWSTag{
				{"Debug", "True"},
				{"Production", "False"},
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			// Use happy path defaults if no clients are provided
			fec2 := &fakeEC2Client{
				importSnapshot: &ec2.ImportSnapshotOutput{
					ImportTaskId: aws.String("import-task-id"),
				},
				registerImage: &ec2.RegisterImageOutput{
					ImageId: aws.String("ami-1234567890abcdef0"),
				},
			}
			if tc.ec2Client != nil {
				fec2 = tc.ec2Client
			}
			fs3 := &fakeS3Client{}
			if tc.s3Client != nil {
				fs3 = tc.s3Client
			}
			fs3u := &fakeS3Uploader{}
			if tc.s3Uploader != nil {
				fs3u = tc.s3Uploader
			}
			snapImportedWaiterOutput := tc.snapshotImportedWaiterOutput
			if snapImportedWaiterOutput == nil {
				snapImportedWaiterOutput = &ec2.DescribeImportSnapshotTasksOutput{
					ImportSnapshotTasks: []ec2types.ImportSnapshotTask{
						{
							SnapshotTaskDetail: &ec2types.SnapshotTaskDetail{
								SnapshotId: aws.String("snap-1234567890abcdef0"),
								Status:     aws.String("completed"),
							},
						},
					},
				}
			}

			restore := awscloud.MockNewSnapshotImportedWaiterEC2(snapImportedWaiterOutput, tc.snapshotImportedWaiterErr)
			defer restore()

			awsClient := awscloud.NewAWSForTest(fec2, fs3, fs3u, nil)
			require.NotNil(t, awsClient)

			imageId, snapshotId, err := awsClient.Register(testImageName, testBucketName, testObjectName, tc.tags, tc.shareWith, tc.architecture, tc.bootMode, tc.importRole)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					require.Equal(t, tc.errMsg, err.Error())
				}

				require.Empty(t, imageId)
				require.Empty(t, snapshotId)

				// TODO: check number of calls based on which error is expected
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, imageId)
			require.NotEmpty(t, snapshotId)

			// import snapshot
			require.Len(t, fec2.importSnapshotCalls, 1)
			require.Equal(t, testBucketName, *fec2.importSnapshotCalls[0].DiskContainer.UserBucket.S3Bucket)
			require.Equal(t, testObjectName, *fec2.importSnapshotCalls[0].DiskContainer.UserBucket.S3Key)
			require.Equal(t, tc.importRole, fec2.importSnapshotCalls[0].RoleName)

			// delete S3 object
			require.Len(t, fs3.deleteObjectCalls, 1)
			require.Equal(t, testBucketName, *fs3.deleteObjectCalls[0].Bucket)
			require.Equal(t, testObjectName, *fs3.deleteObjectCalls[0].Key)

			// the image and the snapshot are tagged with the same name
			require.Len(t, fec2.createTagsCalls, 2)
			for _, tagCalls := range fec2.createTagsCalls {
				if len(tc.tags) > 0 {
					require.Len(t, tagCalls.Tags, 1+len(tc.tags))
				} else {
					require.Len(t, tagCalls.Tags, 1)
				}
				require.Equal(t, "Name", *tagCalls.Tags[0].Key)
				require.Equal(t, testImageName, *tagCalls.Tags[0].Value)
			}

			// register image
			require.Len(t, fec2.registerImageCalls, 1)
			require.Equal(t, testImageName, *fec2.registerImageCalls[0].Name)
			require.Equal(t, ec2types.ArchitectureValues(tc.architecture.String()), fec2.registerImageCalls[0].Architecture)
			require.Equal(t, ec2types.BootModeValues(common.Must(awscloud.EC2BootMode(tc.bootMode))), fec2.registerImageCalls[0].BootMode)
			require.Len(t, fec2.registerImageCalls[0].BlockDeviceMappings, 1)
			require.Equal(t, snapImportedWaiterOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId, fec2.registerImageCalls[0].BlockDeviceMappings[0].Ebs.SnapshotId)

			if len(tc.shareWith) > 0 {
				// share snapshot with accounts
				require.Len(t, fec2.modifySnapshotAttributeCalls, 1)
				require.Equal(t, ec2types.SnapshotAttributeNameCreateVolumePermission, fec2.modifySnapshotAttributeCalls[0].Attribute)
				require.Equal(t, ec2types.OperationTypeAdd, fec2.modifySnapshotAttributeCalls[0].OperationType)
				require.Equal(t, snapImportedWaiterOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId, fec2.modifySnapshotAttributeCalls[0].SnapshotId)
				require.Equal(t, tc.shareWith, fec2.modifySnapshotAttributeCalls[0].UserIds)

				// share image with accounts
				require.Len(t, fec2.modifyImageAttributeCalls, 1)
				require.Equal(t, fec2.registerImage.ImageId, fec2.modifyImageAttributeCalls[0].ImageId)
				require.Len(t, fec2.modifyImageAttributeCalls[0].LaunchPermission.Add, len(tc.shareWith))
				for _, userId := range tc.shareWith {
					require.Equal(t, userId, *fec2.modifyImageAttributeCalls[0].LaunchPermission.Add[0].UserId)
				}
			}
		})
	}
}

func TestShareImage(t *testing.T) {
	knownImageId := "ami-1234567890abcdef0"
	knownSnapshotId1 := "snap-1234567890abcdef0"
	knownSnapshotId2 := "snap-0987654321fedcba0"

	testCases := []struct {
		name          string
		snapIDs       []string
		shareWith     []string
		fakeEC2Client *fakeEC2Client
		expectErr     bool
		errMsg        string
	}{
		{
			name:      "happy path - 2 accounts, 1 snapshot",
			shareWith: []string{"123456789012", "098765432109"},
			fakeEC2Client: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: &knownImageId,
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: &knownSnapshotId1,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "happy path - 1 account, 2 snapshots",
			shareWith: []string{"123456789012"},
			fakeEC2Client: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: &knownImageId,
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: &knownSnapshotId1,
									},
								},
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: &knownSnapshotId2,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:          "happy path - 1 account, 1 snapshot, snapshots provided",
			shareWith:     []string{"123456789012"},
			snapIDs:       []string{knownSnapshotId1},
			fakeEC2Client: &fakeEC2Client{},
		},
		{
			name: "no accounts to share with",
			fakeEC2Client: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: &knownImageId,
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: &knownSnapshotId1,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "error: describe images failure",
			expectErr: true,
			errMsg:    "describe images error",
			fakeEC2Client: &fakeEC2Client{
				describeImagesErr: fmt.Errorf("describe images error"),
			},
		},
		{
			name:      "error: image not found",
			expectErr: true,
			errMsg:    fmt.Sprintf("Unable to find image with id: %s", knownImageId),
			fakeEC2Client: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{},
				},
			},
		},
		{
			name:      "error: modify snapshot attribute failure",
			expectErr: true,
			errMsg:    "modify snapshot attribute error",
			fakeEC2Client: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: &knownImageId,
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: &knownSnapshotId1,
									},
								},
							},
						},
					},
				},
				modifySnapshotAttributeErr: fmt.Errorf("modify snapshot attribute error"),
			},
		},
		{
			name:      "error: modify image attribute failure",
			expectErr: true,
			errMsg:    "modify image attribute error",
			fakeEC2Client: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: &knownImageId,
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: &knownSnapshotId1,
									},
								},
							},
						},
					},
				},
				modifyImageAttributeErr: fmt.Errorf("modify image attribute error"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := awscloud.NewAWSForTest(tc.fakeEC2Client, nil, nil, nil)
			require.NotNil(t, client)
			err := client.ShareImage(knownImageId, tc.snapIDs, tc.shareWith)

			if len(tc.snapIDs) == 0 {
				// if no snapshots were provided, the function should describe the image
				require.Len(t, tc.fakeEC2Client.describeImagesCalls, 1)
			} else {
				require.Len(t, tc.fakeEC2Client.describeImagesCalls, 0)
			}
			if tc.fakeEC2Client.describeImagesErr != nil ||
				(tc.fakeEC2Client.describeImages != nil && len(tc.fakeEC2Client.describeImages.Images) == 0) {
				require.Len(t, tc.fakeEC2Client.modifySnapshotAttributeCalls, 0)
				require.Len(t, tc.fakeEC2Client.modifyImageAttributeCalls, 0)
			} else if tc.fakeEC2Client.modifySnapshotAttributeErr != nil {
				require.Len(t, tc.fakeEC2Client.modifySnapshotAttributeCalls, 1)
				require.Len(t, tc.fakeEC2Client.modifyImageAttributeCalls, 0)
			} else if tc.fakeEC2Client.modifyImageAttributeErr != nil {
				require.Len(t, tc.fakeEC2Client.modifySnapshotAttributeCalls, 1)
				require.Len(t, tc.fakeEC2Client.modifyImageAttributeCalls, 1)
			}

			if tc.expectErr {
				require.Error(t, err)
				require.EqualError(t, err, tc.errMsg)
				return
			}

			require.NoError(t, err)

			var snapIDs []string
			if len(tc.snapIDs) > 0 {
				snapIDs = tc.snapIDs
			} else {
				for _, mapping := range tc.fakeEC2Client.describeImages.Images[0].BlockDeviceMappings {
					snapIDs = append(snapIDs, *mapping.Ebs.SnapshotId)
				}
			}

			require.Len(t, tc.fakeEC2Client.modifySnapshotAttributeCalls, len(snapIDs))
			for i, snapID := range snapIDs {
				require.Equal(t, ec2types.SnapshotAttributeNameCreateVolumePermission, tc.fakeEC2Client.modifySnapshotAttributeCalls[i].Attribute)
				require.Equal(t, ec2types.OperationTypeAdd, tc.fakeEC2Client.modifySnapshotAttributeCalls[i].OperationType)
				require.Equal(t, snapID, *tc.fakeEC2Client.modifySnapshotAttributeCalls[i].SnapshotId)
				require.Equal(t, tc.shareWith, tc.fakeEC2Client.modifySnapshotAttributeCalls[i].UserIds)
			}

			require.Len(t, tc.fakeEC2Client.modifyImageAttributeCalls, 1)
			require.Equal(t, &knownImageId, tc.fakeEC2Client.modifyImageAttributeCalls[0].ImageId)
			require.Len(t, tc.fakeEC2Client.modifyImageAttributeCalls[0].LaunchPermission.Add, len(tc.shareWith))
			for idx, userId := range tc.shareWith {
				require.Equal(t, userId, *tc.fakeEC2Client.modifyImageAttributeCalls[0].LaunchPermission.Add[idx].UserId)
			}
		})
	}
}

func TestRegions(t *testing.T) {
	type testCase struct {
		name      string
		fec2      *fakeEC2Client
		expectErr bool
		errMsg    string
	}
	testCases := []testCase{
		{
			name: "happy path",
			fec2: &fakeEC2Client{
				describeRegions: &ec2.DescribeRegionsOutput{
					Regions: []ec2types.Region{
						{RegionName: aws.String("us-east-1")},
						{RegionName: aws.String("us-west-2")},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "error: unable to list regions",
			fec2: &fakeEC2Client{
				describeRegionsErr: fmt.Errorf("unable to list regions"),
			},
			expectErr: true,
			errMsg:    "unable to list regions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			regions, err := awsClient.Regions()
			require.Len(t, tc.fec2.describeRegionsCalls, 1)

			if tc.expectErr {
				require.Empty(t, regions)
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.Len(t, regions, len(tc.fec2.describeRegions.Regions))
			for idx, region := range regions {
				require.Equal(t, *tc.fec2.describeRegions.Regions[idx].RegionName, region)
			}
		})
	}
}

func TestCreateSecurityGroupEC2(t *testing.T) {
	type testCase struct {
		name      string
		sgName    string
		fec2      *fakeEC2Client
		expectErr bool
		errMsg    string
	}
	testCases := []testCase{
		{
			name:   "happy path",
			sgName: "test-group",
			fec2: &fakeEC2Client{
				createSecurityGroup: &ec2.CreateSecurityGroupOutput{
					GroupId: aws.String("sg-12345678"),
				},
			},
			expectErr: false,
		},
		{
			name:   "error: unable to create security group",
			sgName: "test-group",
			fec2: &fakeEC2Client{
				createSecurityGroupErr: fmt.Errorf("unable to create security group"),
			},
			expectErr: true,
			errMsg:    "unable to create security group",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			createSGOut, err := awsClient.CreateSecurityGroupEC2(tc.sgName, "Test security group")
			require.Len(t, tc.fec2.createSecurityGroupCalls, 1)
			require.Equal(t, tc.sgName, *tc.fec2.createSecurityGroupCalls[0].GroupName)

			if tc.expectErr {
				require.Empty(t, createSGOut)
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.fec2.createSecurityGroup.GroupId, createSGOut.GroupId)
		})
	}
}

func TestDeleteSecurityGroupEC2(t *testing.T) {
	type testCase struct {
		name      string
		sgID      string
		fec2      *fakeEC2Client
		expectErr bool
		errMsg    string
	}
	testCases := []testCase{
		{
			name: "happy path",
			sgID: "sg-12345678",
			fec2: &fakeEC2Client{
				deleteSecurityGroup: &ec2.DeleteSecurityGroupOutput{
					GroupId: aws.String("sg-12345678"),
				},
			},
			expectErr: false,
		},
		{
			name: "error: unable to delete security group",
			sgID: "sg-12345678",
			fec2: &fakeEC2Client{
				deleteSecurityGroupErr: fmt.Errorf("unable to delete security group"),
			},
			expectErr: true,
			errMsg:    "unable to delete security group",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			deleteSGOut, err := awsClient.DeleteSecurityGroupEC2(tc.sgID)
			require.Len(t, tc.fec2.deleteSecurityGroupCalls, 1)
			require.Equal(t, tc.sgID, *tc.fec2.deleteSecurityGroupCalls[0].GroupId)

			if tc.expectErr {
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, deleteSGOut)
			require.Equal(t, tc.sgID, *deleteSGOut.GroupId)
		})
	}
}

func TestAuthorizeSecurityGroupIngressEC2(t *testing.T) {
	type testCase struct {
		name      string
		sgID      string
		cidr      string
		fromPort  int32
		toPort    int32
		protocol  string
		fec2      *fakeEC2Client
		expectErr bool
		errMsg    string
	}
	testCases := []testCase{
		{
			name:     "happy path",
			sgID:     "sg-12345678",
			cidr:     "0.0.0.0/0",
			fromPort: 22,
			toPort:   22,
			protocol: "tcp",
			fec2: &fakeEC2Client{
				authorizeSecurityGroupIngress: &ec2.AuthorizeSecurityGroupIngressOutput{
					SecurityGroupRules: []ec2types.SecurityGroupRule{
						{
							GroupId:    aws.String("sg-12345678"),
							CidrIpv4:   common.ToPtr("0.0.0.0/0"),
							FromPort:   common.ToPtr(int32(22)),
							ToPort:     common.ToPtr(int32(22)),
							IpProtocol: common.ToPtr("tcp"),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:     "error: unable to authorize security group ingress",
			sgID:     "sg-12345678",
			cidr:     "0.0.0.0/0",
			fromPort: 22,
			toPort:   22,
			protocol: "tcp",
			fec2: &fakeEC2Client{
				authorizeSecurityGroupIngressErr: fmt.Errorf("unable to authorize security group ingress"),
			},
			expectErr: true,
			errMsg:    "unable to authorize security group ingress",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			output, err := awsClient.AuthorizeSecurityGroupIngressEC2(tc.sgID, tc.cidr, tc.fromPort, tc.toPort, tc.protocol)
			require.Len(t, tc.fec2.authorizeSecurityGroupIngressCalls, 1)

			if tc.expectErr {
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				require.Nil(t, output)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, output)
			require.Equal(t, tc.sgID, *output.SecurityGroupRules[0].GroupId)
			require.Equal(t, tc.cidr, *output.SecurityGroupRules[0].CidrIpv4)
			require.Equal(t, tc.fromPort, *output.SecurityGroupRules[0].FromPort)
			require.Equal(t, tc.toPort, *output.SecurityGroupRules[0].ToPort)
			require.Equal(t, tc.protocol, *output.SecurityGroupRules[0].IpProtocol)
		})
	}
}

func TestRunInstanceEC2(t *testing.T) {
	type testCase struct {
		name                        string
		imageId                     string
		sgID                        string
		userData                    []byte
		instanceType                string
		fec2                        *fakeEC2Client
		newInstanceRunningWaiterErr error
		expectErr                   bool
		errMsg                      string
	}
	testCases := []testCase{
		{
			name:         "happy path",
			imageId:      "ami-1234567890abcdef0",
			sgID:         "sg-12345678",
			userData:     []byte("test user data"),
			instanceType: "t2.micro",
			fec2: &fakeEC2Client{
				runInstances: &ec2.RunInstancesOutput{
					Instances: []ec2types.Instance{
						{
							InstanceId: aws.String("i-1234567890abcdef0"),
							ImageId:    aws.String("ami-1234567890abcdef0"),
							SecurityGroups: []ec2types.GroupIdentifier{
								{
									GroupId: aws.String("sg-12345678"),
								},
							},
							InstanceType: ec2types.InstanceTypeT2Micro,
						},
					},
				},
				describeInstances: &ec2.DescribeInstancesOutput{
					Reservations: []ec2types.Reservation{
						{
							Instances: []ec2types.Instance{
								{
									InstanceId: aws.String("i-1234567890abcdef0"),
									ImageId:    aws.String("ami-1234567890abcdef0"),
									SecurityGroups: []ec2types.GroupIdentifier{
										{
											GroupId: aws.String("sg-12345678"),
										},
									},
									InstanceType: ec2types.InstanceTypeT2Micro,
								},
							},
						},
					},
				},
			},
			newInstanceRunningWaiterErr: nil,
			expectErr:                   false,
		},
		{
			name:         "error: invalid instance type",
			imageId:      "ami-1234567890abcdef0",
			sgID:         "sg-12345678",
			userData:     []byte("test user data"),
			instanceType: "invalid-type",
			fec2:         &fakeEC2Client{},
			expectErr:    true,
			errMsg:       "ec2 doesn't support the following instance type: invalid-type",
		},
		{
			name:         "error: unable to run instance",
			imageId:      "ami-1234567890abcdef0",
			sgID:         "sg-12345678",
			userData:     []byte("test user data"),
			instanceType: "t2.micro",
			fec2: &fakeEC2Client{
				runInstancesErr: fmt.Errorf("unable to run instance"),
			},
			newInstanceRunningWaiterErr: nil,
			expectErr:                   true,
			errMsg:                      "unable to run instance",
		},
		{
			name:         "error: instance running waiter error",
			imageId:      "ami-1234567890abcdef0",
			sgID:         "sg-12345678",
			userData:     []byte("test user data"),
			instanceType: "t2.micro",
			fec2: &fakeEC2Client{
				runInstances: &ec2.RunInstancesOutput{
					Instances: []ec2types.Instance{
						{
							InstanceId: aws.String("i-1234567890abcdef0"),
							ImageId:    aws.String("ami-1234567890abcdef0"),
							SecurityGroups: []ec2types.GroupIdentifier{
								{
									GroupId: aws.String("sg-12345678"),
								},
							},
							InstanceType: ec2types.InstanceTypeT2Micro,
						},
					},
				},
			},
			newInstanceRunningWaiterErr: fmt.Errorf("instance running waiter error"),
			expectErr:                   true,
			errMsg:                      "instance running waiter error",
		},
		{
			name:         "error: fail to create instance",
			imageId:      "ami-1234567890abcdef0",
			sgID:         "sg-12345678",
			userData:     []byte("test user data"),
			instanceType: "t2.micro",
			fec2: &fakeEC2Client{
				runInstances: &ec2.RunInstancesOutput{
					Instances: []ec2types.Instance{
						{
							InstanceId: aws.String("i-1234567890abcdef0"),
							ImageId:    aws.String("ami-1234567890abcdef0"),
							SecurityGroups: []ec2types.GroupIdentifier{
								{
									GroupId: aws.String("sg-12345678"),
								},
							},
							InstanceType: ec2types.InstanceTypeT2Micro,
						},
					},
				},
				describeInstancesErr: fmt.Errorf("unable to describe instances"),
			},
			newInstanceRunningWaiterErr: nil,
			expectErr:                   true,
			errMsg:                      "failed to get reservation for instance i-1234567890abcdef0: unable to describe instances",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			// Mock the waiter for instance running
			restore := awscloud.MockNewInstanceRunningWaiterEC2(tc.newInstanceRunningWaiterErr)
			defer restore()

			output, err := awsClient.RunInstanceEC2(tc.imageId, tc.sgID, string(tc.userData), tc.instanceType)

			if tc.expectErr {
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				require.Nil(t, output)
				return
			}

			require.Len(t, tc.fec2.runInstancesCalls, 1)
			require.Equal(t, tc.imageId, *tc.fec2.runInstancesCalls[0].ImageId)
			require.Equal(t, tc.sgID, tc.fec2.runInstancesCalls[0].SecurityGroupIds[0])
			require.Equal(t, ec2types.InstanceType(tc.instanceType), tc.fec2.runInstancesCalls[0].InstanceType)

			require.NoError(t, err)
			require.NotNil(t, output)
			require.Equal(t, tc.fec2.runInstances.Instances[0].InstanceId, output.Instances[0].InstanceId)
			require.Equal(t, tc.imageId, *output.Instances[0].ImageId)
			require.Equal(t, tc.sgID, *output.Instances[0].SecurityGroups[0].GroupId)
			require.Equal(t, ec2types.InstanceType(tc.instanceType), output.Instances[0].InstanceType)
		})
	}

}

func TestTerminateInstanceEC2(t *testing.T) {
	type testCase struct {
		name                        string
		instanceIDs                 []string
		timeout                     time.Duration
		fec2                        *fakeEC2Client
		instanceTerminatedWaiterErr error
		expectErr                   bool
		errMsg                      string
	}

	testCases := []testCase{
		{
			name:        "happy path",
			instanceIDs: []string{"i-1234567890abcdef0"},
			fec2: &fakeEC2Client{
				terminateInstances: &ec2.TerminateInstancesOutput{
					TerminatingInstances: []ec2types.InstanceStateChange{
						{
							InstanceId: aws.String("i-1234567890abcdef0"),
							CurrentState: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameTerminated,
							},
						},
					},
				},
			},
		},
		{
			name:        "happy path with multiple instance IDs",
			instanceIDs: []string{"i-1234567890abcdef0", "i-0987654321fedcba0"},
			fec2: &fakeEC2Client{
				terminateInstances: &ec2.TerminateInstancesOutput{
					TerminatingInstances: []ec2types.InstanceStateChange{
						{
							InstanceId: aws.String("i-1234567890abcdef0"),
							CurrentState: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameTerminated,
							},
						},
						{
							InstanceId: aws.String("i-0987654321fedcba0"),
							CurrentState: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameTerminated,
							},
						},
					},
				},
			},
		},
		{
			name:        "error: unable to terminate instance",
			instanceIDs: []string{"i-1234567890abcdef0"},
			fec2: &fakeEC2Client{
				terminateInstancesErr: fmt.Errorf("unable to terminate instance"),
			},
			instanceTerminatedWaiterErr: nil,
			expectErr:                   true,
			errMsg:                      "unable to terminate instance",
		},
		{
			name:        "error: instance terminated waiter error",
			instanceIDs: []string{"i-1234567890abcdef0"},
			timeout:     30 * time.Second,
			fec2: &fakeEC2Client{
				terminateInstances: &ec2.TerminateInstancesOutput{
					TerminatingInstances: []ec2types.InstanceStateChange{
						{
							InstanceId: aws.String("i-1234567890abcdef0"),
							CurrentState: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameTerminated,
							},
						},
					},
				},
			},
			instanceTerminatedWaiterErr: fmt.Errorf("instance terminated waiter error"),
			expectErr:                   true,
			errMsg:                      "instance terminated waiter error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			// Mock the waiter for instance terminated
			restore := awscloud.MockNewTerminateInstancesWaiterEC2(tc.instanceTerminatedWaiterErr)
			defer restore()

			out, err := awsClient.TerminateInstancesEC2(tc.instanceIDs, tc.timeout)

			if tc.expectErr {
				require.Error(t, err)
				require.Nil(t, out)
				require.Equal(t, tc.errMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, out)
			require.Len(t, tc.fec2.terminateInstancesCalls, 1)
			require.Equal(t, tc.instanceIDs, tc.fec2.terminateInstancesCalls[0].InstanceIds)
		})
	}
}

func TestGetInstanceAddress(t *testing.T) {
	type testCase struct {
		name            string
		instanceId      string
		expectedAddress string
		fec2            *fakeEC2Client
		expectErr       bool
		errMsg          string
	}
	testCases := []testCase{
		{
			name:            "happy path",
			instanceId:      "i-1234567890abcdef0",
			expectedAddress: "192.168.1.1",
			fec2: &fakeEC2Client{
				describeInstances: &ec2.DescribeInstancesOutput{
					Reservations: []ec2types.Reservation{
						{
							Instances: []ec2types.Instance{
								{
									InstanceId:      aws.String("i-1234567890abcdef0"),
									PublicIpAddress: aws.String("192.168.1.1"),
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:       "error: instance not found",
			instanceId: "i-1234567890abcdef0",
			fec2: &fakeEC2Client{
				describeInstances: &ec2.DescribeInstancesOutput{
					Reservations: []ec2types.Reservation{},
				},
			},
			expectErr: true,
			errMsg:    "no reservation found for instance i-1234567890abcdef0",
		},
		{
			name:       "error: describe instances failure",
			instanceId: "i-1234567890abcdef0",
			fec2: &fakeEC2Client{
				describeInstancesErr: fmt.Errorf("describe instances error"),
			},
			expectErr: true,
			errMsg:    "describe instances error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			address, err := awsClient.GetInstanceAddress(tc.instanceId)

			require.Len(t, tc.fec2.describeInstancesCalls, 1)
			require.Equal(t, tc.instanceId, tc.fec2.describeInstancesCalls[0].InstanceIds[0])

			if tc.expectErr {
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedAddress, address)
		})
	}
}

func TestDeleteEC2Image(t *testing.T) {
	type testCase struct {
		name      string
		imageId   string
		fec2      *fakeEC2Client
		expectErr bool
		errMsg    string
	}
	testCases := []testCase{
		{
			name:    "happy path",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				deregisterImage: &ec2.DeregisterImageOutput{},
				deleteSnapshot:  &ec2.DeleteSnapshotOutput{},
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: aws.String("ami-1234567890abcdef0"),
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-1234567890abcdef0"),
									},
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:    "happy path - 2 snapshots",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				deregisterImage: &ec2.DeregisterImageOutput{},
				deleteSnapshot:  &ec2.DeleteSnapshotOutput{},
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: aws.String("ami-1234567890abcdef0"),
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-1234567890abcdef0"),
									},
								},
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-0987654321fedcba0"),
									},
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:    "error: image not found",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{},
				},
			},
			expectErr: true,
			errMsg:    "image ami-1234567890abcdef0 not found",
		},
		{
			name:    "error: describe images failure",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				describeImagesErr: fmt.Errorf("failed to describe image"),
			},
			expectErr: true,
			errMsg:    "failed to describe image",
		},
		{
			name:    "error: unable to deregister image",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				deregisterImageErr: fmt.Errorf("unable to deregister image"),
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: aws.String("ami-1234567890abcdef0"),
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-1234567890abcdef0"),
									},
								},
							},
						},
					},
				},
			},
			expectErr: true,
			errMsg:    "failed to deregister image ami-1234567890abcdef0: unable to deregister image",
		},
		{
			name:    "error: unable to delete snapshot",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				deregisterImage:   &ec2.DeregisterImageOutput{},
				deleteSnapshotErr: fmt.Errorf("unable to delete snapshot"),
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: aws.String("ami-1234567890abcdef0"),
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-1234567890abcdef0"),
									},
								},
							},
						},
					},
				},
			},
			expectErr: true,
			errMsg:    "failed to delete snapshot snap-1234567890abcdef0: unable to delete snapshot",
		},
		{
			name:    "error: unable to delete 2 snapshots",
			imageId: "ami-1234567890abcdef0",
			fec2: &fakeEC2Client{
				deregisterImage:   &ec2.DeregisterImageOutput{},
				deleteSnapshotErr: fmt.Errorf("unable to delete snapshot"),
				describeImages: &ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							ImageId: aws.String("ami-1234567890abcdef0"),
							BlockDeviceMappings: []ec2types.BlockDeviceMapping{
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-1234567890abcdef0"),
									},
								},
								{
									Ebs: &ec2types.EbsBlockDevice{
										SnapshotId: aws.String("snap-0987654321fedcba0"),
									},
								},
							},
						},
					},
				},
			},
			expectErr: true,
			errMsg:    "failed to delete snapshot snap-1234567890abcdef0: unable to delete snapshot; failed to delete snapshot snap-0987654321fedcba0: unable to delete snapshot",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := awscloud.NewAWSForTest(tc.fec2, nil, nil, nil)
			require.NotNil(t, awsClient)

			err := awsClient.DeleteEC2Image(tc.imageId)

			require.Len(t, tc.fec2.describeImagesCalls, 1)
			require.Equal(t, tc.imageId, tc.fec2.describeImagesCalls[0].ImageIds[0])

			if tc.expectErr {
				require.Error(t, err)
				require.Equal(t, tc.errMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.Len(t, tc.fec2.deregisterImageCalls, 1)
			require.Equal(t, tc.imageId, *tc.fec2.deregisterImageCalls[0].ImageId)

			require.Len(t, tc.fec2.deleteSnapshotCalls, len(tc.fec2.describeImages.Images[0].BlockDeviceMappings))
		})
	}
}

func TestS3MarkObjectAsPublic(t *testing.T) {
	fc := &fakeS3Client{}
	aws := awscloud.NewAWSForTest(nil, fc, nil, nil)
	require.NotNil(t, aws)

	require.NoError(t, aws.MarkS3ObjectAsPublic("bucket", "object-key"))
	require.Len(t, fc.putObjectAclCalls, 1)
	require.Equal(t, "bucket", *fc.putObjectAclCalls[0].Bucket)
	require.Equal(t, "object-key", *fc.putObjectAclCalls[0].Key)
	require.Equal(t, s3types.ObjectCannedACLPublicRead, fc.putObjectAclCalls[0].ACL)
}

func TestS3MarkObjectAsPublicError(t *testing.T) {
	fc := &fakeS3Client{
		putObjectAclErr: fmt.Errorf("error marking object as public"),
	}
	aws := awscloud.NewAWSForTest(nil, fc, nil, nil)
	require.NotNil(t, aws)

	err := aws.MarkS3ObjectAsPublic("bucket", "object-key")
	require.Error(t, err)
	require.Len(t, fc.putObjectAclCalls, 1)
	require.Equal(t, "bucket", *fc.putObjectAclCalls[0].Bucket)
	require.Equal(t, "object-key", *fc.putObjectAclCalls[0].Key)
}

func TestS3Upload(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFilePath := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(tmpFilePath, []byte("test content"), 0600))
	fm := &fakeS3Uploader{}
	aws := awscloud.NewAWSForTest(nil, nil, fm, nil)
	require.NotNil(t, aws)

	uo, err := aws.Upload(tmpFilePath, "bucket", "object-key")
	require.NoError(t, err)
	require.Len(t, fm.uploadCalls, 1)
	require.Equal(t, "bucket", *fm.uploadCalls[0].Bucket)
	require.Equal(t, "object-key", *fm.uploadCalls[0].Key)
	require.Equal(t, "object-key", *uo.Key)
}

func TestS3UploadError(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFilePath := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(tmpFilePath, []byte("test content"), 0600))
	fm := &fakeS3Uploader{
		uploadErr: fmt.Errorf("upload error"),
	}
	aws := awscloud.NewAWSForTest(nil, nil, fm, nil)
	require.NotNil(t, aws)

	_, err := aws.Upload(tmpFilePath, "bucket", "object-key")
	require.Error(t, err)
	require.Len(t, fm.uploadCalls, 1)
	require.Equal(t, "bucket", *fm.uploadCalls[0].Bucket)
	require.Equal(t, "object-key", *fm.uploadCalls[0].Key)
}

func TestS3ObjectPresignedURL(t *testing.T) {
	fs := &fakeS3Presign{}
	aws := awscloud.NewAWSForTest(nil, nil, nil, fs)
	require.NotNil(t, aws)

	url, err := aws.S3ObjectPresignedURL("bucket", "object-key")
	require.NoError(t, err)
	require.Len(t, fs.presignGetObjectCalls, 1)
	require.Equal(t, "bucket", *fs.presignGetObjectCalls[0].Bucket)
	require.Equal(t, "object-key", *fs.presignGetObjectCalls[0].Key)
	require.Equal(t, "https://bucket.s3.amazonaws.com/object-key", url)
}

func TestS3ObjectPresignedURLError(t *testing.T) {
	fs := &fakeS3Presign{
		presignGetObjectErr: fmt.Errorf("presign error"),
	}
	aws := awscloud.NewAWSForTest(nil, nil, nil, fs)
	require.NotNil(t, aws)

	_, err := aws.S3ObjectPresignedURL("bucket", "object-key")
	require.Error(t, err)
	require.Len(t, fs.presignGetObjectCalls, 1)
	require.Equal(t, "bucket", *fs.presignGetObjectCalls[0].Bucket)
	require.Equal(t, "object-key", *fs.presignGetObjectCalls[0].Key)
}

func TestDeleteObject(t *testing.T) {
	fc := &fakeS3Client{}
	aws := awscloud.NewAWSForTest(nil, fc, nil, nil)
	require.NotNil(t, aws)

	require.NoError(t, aws.DeleteObject("bucket", "object-key"))
	require.Len(t, fc.deleteObjectCalls, 1)
	require.Equal(t, "bucket", *fc.deleteObjectCalls[0].Bucket)
	require.Equal(t, "object-key", *fc.deleteObjectCalls[0].Key)
}

func TestDeleteObjectError(t *testing.T) {
	fc := &fakeS3Client{
		deleteObjectErr: fmt.Errorf("error deleting object"),
	}
	aws := awscloud.NewAWSForTest(nil, fc, nil, nil)
	require.NotNil(t, aws)

	err := aws.DeleteObject("bucket", "object-key")
	require.Error(t, err)
	require.Len(t, fc.deleteObjectCalls, 1)
	require.Equal(t, "bucket", *fc.deleteObjectCalls[0].Bucket)
	require.Equal(t, "object-key", *fc.deleteObjectCalls[0].Key)
}

func TestBuckets(t *testing.T) {
	fc := &fakeS3Client{
		buckets: []string{"bucket1", "bucket2"},
	}
	aws := awscloud.NewAWSForTest(nil, fc, nil, nil)
	require.NotNil(t, aws)

	buckets, err := aws.Buckets()
	require.NoError(t, err)
	require.Len(t, buckets, 2)
	require.Equal(t, "bucket1", buckets[0])
	require.Equal(t, "bucket2", buckets[1])
	require.Equal(t, 1, fc.listBucketsCalls)
}

func TestBucketsError(t *testing.T) {
	fc := &fakeS3Client{
		listBucketsErr: fmt.Errorf("error listing buckets"),
	}
	aws := awscloud.NewAWSForTest(nil, fc, nil, nil)
	require.NotNil(t, aws)

	_, err := aws.Buckets()
	require.Error(t, err)
	require.Equal(t, 1, fc.listBucketsCalls)
}

func TestCheckBucketPermission(t *testing.T) {
	type testCase struct {
		name       string
		fc         *fakeS3Client
		permission s3types.Permission
		expected   bool
		expectErr  bool
	}
	testCases := []testCase{
		{
			name: "happy path",
			fc: &fakeS3Client{
				bucketAcl: &s3.GetBucketAclOutput{
					Grants: []s3types.Grant{
						{
							Permission: s3types.PermissionRead,
						},
					},
				},
			},
			permission: s3types.PermissionRead,
			expected:   true,
		},
		{
			name: "permission not granted",
			fc: &fakeS3Client{
				bucketAcl: &s3.GetBucketAclOutput{
					Grants: []s3types.Grant{
						{
							Permission: s3types.PermissionRead,
						},
					},
				},
			},
			permission: s3types.PermissionWrite,
			expected:   false,
		},
		{
			name: "permissions covered by higher level permissions",
			fc: &fakeS3Client{
				bucketAcl: &s3.GetBucketAclOutput{
					Grants: []s3types.Grant{
						{
							Permission: s3types.PermissionFullControl,
						},
					},
				},
			},
			permission: s3types.PermissionWrite,
			expected:   true,
		},
		{
			name: "invalid permission",
			fc: &fakeS3Client{
				bucketAcl: &s3.GetBucketAclOutput{
					Grants: []s3types.Grant{
						{
							Permission: s3types.PermissionRead,
						},
					},
				},
			},
			permission: s3types.Permission("invalid-permission"),
			expected:   false,
		},
		{
			name: "error retrieving bucket ACL",
			fc: &fakeS3Client{
				getBucketAclErr: fmt.Errorf("error retrieving bucket ACL"),
			},
			permission: s3types.PermissionRead,
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aws := awscloud.NewAWSForTest(nil, tc.fc, nil, nil)
			require.NotNil(t, aws)

			result, err := aws.CheckBucketPermission("bucket", tc.permission)

			require.Len(t, tc.fc.getBucketAclCalls, 1)
			require.Equal(t, "bucket", *tc.fc.getBucketAclCalls[0].Bucket)

			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}
