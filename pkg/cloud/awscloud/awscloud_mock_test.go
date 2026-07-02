package awscloud_test

import (
	"context"
	"fmt"

	awsSigner "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/osbuild/image-builder/pkg/cloud/awscloud"
)

type fakeEC2Client struct {
	describeRegionsCalls []*ec2.DescribeRegionsInput
	describeRegions      *ec2.DescribeRegionsOutput
	describeRegionsErr   error

	authorizeSecurityGroupIngressCalls []*ec2.AuthorizeSecurityGroupIngressInput
	authorizeSecurityGroupIngress      *ec2.AuthorizeSecurityGroupIngressOutput
	authorizeSecurityGroupIngressErr   error

	createSecurityGroupCalls []*ec2.CreateSecurityGroupInput
	createSecurityGroup      *ec2.CreateSecurityGroupOutput
	createSecurityGroupErr   error

	deleteSecurityGroupCalls []*ec2.DeleteSecurityGroupInput
	deleteSecurityGroup      *ec2.DeleteSecurityGroupOutput
	deleteSecurityGroupErr   error

	describeInstancesCalls []*ec2.DescribeInstancesInput
	describeInstances      *ec2.DescribeInstancesOutput
	describeInstancesErr   error

	runInstancesCalls []*ec2.RunInstancesInput
	runInstances      *ec2.RunInstancesOutput
	runInstancesErr   error

	terminateInstancesCalls []*ec2.TerminateInstancesInput
	terminateInstances      *ec2.TerminateInstancesOutput
	terminateInstancesErr   error

	registerImageCalls []*ec2.RegisterImageInput
	registerImage      *ec2.RegisterImageOutput
	registerImageErr   error

	deregisterImageCalls []*ec2.DeregisterImageInput
	deregisterImage      *ec2.DeregisterImageOutput
	deregisterImageErr   error

	describeImagesCalls []*ec2.DescribeImagesInput
	describeImages      *ec2.DescribeImagesOutput
	describeImagesErr   error

	modifyImageAttributeCalls []*ec2.ModifyImageAttributeInput
	modifyImageAttribute      *ec2.ModifyImageAttributeOutput
	modifyImageAttributeErr   error

	deleteSnapshotCalls []*ec2.DeleteSnapshotInput
	deleteSnapshot      *ec2.DeleteSnapshotOutput
	deleteSnapshotErr   error

	describeImportSnapshotTasksCalls []*ec2.DescribeImportSnapshotTasksInput
	describeImportSnapshotTasks      *ec2.DescribeImportSnapshotTasksOutput
	describeImportSnapshotTasksErr   error

	importSnapshotCalls []*ec2.ImportSnapshotInput
	importSnapshot      *ec2.ImportSnapshotOutput
	importSnapshotErr   error

	modifySnapshotAttributeCalls []*ec2.ModifySnapshotAttributeInput
	modifySnapshotAttribute      *ec2.ModifySnapshotAttributeOutput
	modifySnapshotAttributeErr   error

	createTagsCalls []*ec2.CreateTagsInput
	createTags      *ec2.CreateTagsOutput
	createTagsErr   error
}

var _ awscloud.EC2Client = (*fakeEC2Client)(nil)

func (f *fakeEC2Client) DescribeRegions(ctx context.Context, input *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	f.describeRegionsCalls = append(f.describeRegionsCalls, input)
	if f.describeRegionsErr != nil {
		return nil, f.describeRegionsErr
	}
	return f.describeRegions, nil
}

func (f *fakeEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, input *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	f.authorizeSecurityGroupIngressCalls = append(f.authorizeSecurityGroupIngressCalls, input)
	if f.authorizeSecurityGroupIngressErr != nil {
		return nil, f.authorizeSecurityGroupIngressErr
	}
	return f.authorizeSecurityGroupIngress, nil
}

func (f *fakeEC2Client) CreateSecurityGroup(ctx context.Context, input *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	f.createSecurityGroupCalls = append(f.createSecurityGroupCalls, input)
	if f.createSecurityGroupErr != nil {
		return nil, f.createSecurityGroupErr
	}
	return f.createSecurityGroup, nil
}

func (f *fakeEC2Client) DeleteSecurityGroup(ctx context.Context, input *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	f.deleteSecurityGroupCalls = append(f.deleteSecurityGroupCalls, input)
	if f.deleteSecurityGroupErr != nil {
		return nil, f.deleteSecurityGroupErr
	}
	return f.deleteSecurityGroup, nil
}

func (f *fakeEC2Client) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.describeInstancesCalls = append(f.describeInstancesCalls, input)
	if f.describeInstancesErr != nil {
		return nil, f.describeInstancesErr
	}
	return f.describeInstances, nil
}

func (f *fakeEC2Client) GetConsoleOutput(ctx context.Context, input *ec2.GetConsoleOutputInput, optFns ...func(*ec2.Options)) (*ec2.GetConsoleOutputOutput, error) {
	return &ec2.GetConsoleOutputOutput{}, nil
}

func (f *fakeEC2Client) RunInstances(ctx context.Context, input *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	f.runInstancesCalls = append(f.runInstancesCalls, input)
	if f.runInstancesErr != nil {
		return nil, f.runInstancesErr
	}
	return f.runInstances, nil
}

func (f *fakeEC2Client) TerminateInstances(ctx context.Context, input *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	f.terminateInstancesCalls = append(f.terminateInstancesCalls, input)
	if f.terminateInstancesErr != nil {
		return nil, f.terminateInstancesErr
	}
	return f.terminateInstances, nil
}

func (f *fakeEC2Client) RegisterImage(ctx context.Context, input *ec2.RegisterImageInput, optFns ...func(*ec2.Options)) (*ec2.RegisterImageOutput, error) {
	f.registerImageCalls = append(f.registerImageCalls, input)
	if f.registerImageErr != nil {
		return nil, f.registerImageErr
	}
	return f.registerImage, nil
}

func (f *fakeEC2Client) DeregisterImage(ctx context.Context, input *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
	f.deregisterImageCalls = append(f.deregisterImageCalls, input)
	if f.deregisterImageErr != nil {
		return nil, f.deregisterImageErr
	}
	return f.deregisterImage, nil
}

func (f *fakeEC2Client) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.describeImagesCalls = append(f.describeImagesCalls, input)
	if f.describeImagesErr != nil {
		return nil, f.describeImagesErr
	}
	return f.describeImages, nil
}

func (f *fakeEC2Client) ModifyImageAttribute(ctx context.Context, input *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error) {
	f.modifyImageAttributeCalls = append(f.modifyImageAttributeCalls, input)
	if f.modifyImageAttributeErr != nil {
		return nil, f.modifyImageAttributeErr
	}
	return f.modifyImageAttribute, nil
}

func (f *fakeEC2Client) DeleteSnapshot(ctx context.Context, input *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	f.deleteSnapshotCalls = append(f.deleteSnapshotCalls, input)
	if f.deleteSnapshotErr != nil {
		return nil, f.deleteSnapshotErr
	}
	return f.deleteSnapshot, nil
}

func (f *fakeEC2Client) DescribeImportSnapshotTasks(ctx context.Context, input *ec2.DescribeImportSnapshotTasksInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImportSnapshotTasksOutput, error) {
	f.describeImportSnapshotTasksCalls = append(f.describeImportSnapshotTasksCalls, input)
	if f.describeImportSnapshotTasksErr != nil {
		return nil, f.describeImportSnapshotTasksErr
	}
	return f.describeImportSnapshotTasks, nil
}

func (f *fakeEC2Client) ImportSnapshot(ctx context.Context, input *ec2.ImportSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.ImportSnapshotOutput, error) {
	f.importSnapshotCalls = append(f.importSnapshotCalls, input)
	if f.importSnapshotErr != nil {
		return nil, f.importSnapshotErr
	}
	return f.importSnapshot, nil
}

func (f *fakeEC2Client) ModifySnapshotAttribute(ctx context.Context, input *ec2.ModifySnapshotAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifySnapshotAttributeOutput, error) {
	f.modifySnapshotAttributeCalls = append(f.modifySnapshotAttributeCalls, input)
	if f.modifySnapshotAttributeErr != nil {
		return nil, f.modifySnapshotAttributeErr
	}
	return f.modifySnapshotAttribute, nil
}

func (f *fakeEC2Client) CreateTags(ctx context.Context, input *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	f.createTagsCalls = append(f.createTagsCalls, input)
	if f.createTagsErr != nil {
		return nil, f.createTagsErr
	}
	return f.createTags, nil
}

type fakeS3Client struct {
	deleteObjectCalls []s3.DeleteObjectInput
	deleteObjectErr   error

	listBucketsCalls int
	buckets          []string
	listBucketsErr   error

	putObjectAclCalls []s3.PutObjectAclInput
	putObjectAclErr   error

	getBucketAclCalls []s3.GetBucketAclInput
	bucketAcl         *s3.GetBucketAclOutput
	getBucketAclErr   error
}

var _ awscloud.S3Client = (*fakeS3Client)(nil)

func (f *fakeS3Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleteObjectCalls = append(f.deleteObjectCalls, *input)
	if f.deleteObjectErr != nil {
		return nil, f.deleteObjectErr
	}
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeS3Client) ListBuckets(ctx context.Context, input *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	f.listBucketsCalls++
	if f.listBucketsErr != nil {
		return nil, f.listBucketsErr
	}
	bkts := make([]s3types.Bucket, len(f.buckets))
	for i, b := range f.buckets {
		bkts[i] = s3types.Bucket{
			Name: &b,
		}
	}
	return &s3.ListBucketsOutput{
		Buckets: bkts,
	}, nil
}

func (f *fakeS3Client) PutObjectAcl(ctx context.Context, input *s3.PutObjectAclInput, optFns ...func(*s3.Options)) (*s3.PutObjectAclOutput, error) {
	f.putObjectAclCalls = append(f.putObjectAclCalls, *input)
	if f.putObjectAclErr != nil {
		return nil, f.putObjectAclErr
	}
	return &s3.PutObjectAclOutput{}, nil
}

func (f *fakeS3Client) GetBucketAcl(ctx context.Context, input *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	f.getBucketAclCalls = append(f.getBucketAclCalls, *input)
	if f.getBucketAclErr != nil {
		return nil, f.getBucketAclErr
	}
	return f.bucketAcl, nil
}

type fakeS3Uploader struct {
	uploadCalls []transfermanager.UploadObjectInput
	uploadErr   error
}

var _ awscloud.S3Uploader = (*fakeS3Uploader)(nil)

func (f *fakeS3Uploader) UploadObject(ctx context.Context, input *transfermanager.UploadObjectInput, optFns ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	f.uploadCalls = append(f.uploadCalls, *input)
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	return &transfermanager.UploadObjectOutput{
		Key: input.Key,
	}, nil
}

type fakeS3Presign struct {
	presignGetObjectCalls []s3.GetObjectInput
	presignGetObjectErr   error
}

var _ awscloud.S3Presign = (*fakeS3Presign)(nil)

func (f *fakeS3Presign) PresignGetObject(ctx context.Context, input *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*awsSigner.PresignedHTTPRequest, error) {
	f.presignGetObjectCalls = append(f.presignGetObjectCalls, *input)
	if f.presignGetObjectErr != nil {
		return nil, f.presignGetObjectErr
	}
	return &awsSigner.PresignedHTTPRequest{
		URL: fmt.Sprintf("https://%s.s3.amazonaws.com/%s", *input.Bucket, *input.Key),
	}, nil
}
