package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/cloud"
	"github.com/osbuild/image-builder/pkg/cloud/awscloud"
	"github.com/osbuild/image-builder/pkg/cloud/azure"
	"github.com/osbuild/image-builder/pkg/cloud/ibmcloud"
	"github.com/osbuild/image-builder/pkg/cloud/libvirt"
	"github.com/osbuild/image-builder/pkg/cloud/openstack"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/progress"
)

// ErrMissingUploadConfig is returned when the upload configuration is missing
var ErrMissingUploadConfig = errors.New("missing upload configuration")

// ErrUploadConfigNotProvided is returned when all the upload configuration is missing
var ErrUploadConfigNotProvided = errors.New("missing all upload configuration")

// ErrUploadTypeUnsupported is returned when the upload type is not supported
var ErrUploadTypeUnsupported = errors.New("unsupported type")

// uploader constructors that are mocked in tests
var (
	awscloudNewUploader  = awscloud.NewUploader
	azureNewUploader     = azure.NewUploader
	libvirtNewUploader   = libvirt.NewUploader
	openstackNewUploader = openstack.NewUploader
	ibmNewUploader       = ibmcloud.NewUploader
)

func uploadImageWithProgress(uploader cloud.Uploader, imagePath string) (*cloud.UploadResult, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// setup basic progress
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat upload: %v", err)
	}
	sizei64 := st.Size()
	if sizei64 < 0 {
		return nil, fmt.Errorf("invalid size read for %s: %d", imagePath, sizei64)
	}
	size := uint64(sizei64)
	pbar := pb.New64(st.Size())
	pbar.Set(pb.Bytes, true)
	pbar.SetWriter(osStderr)
	r := pbar.NewProxyReader(f)
	pbar.Start()
	defer pbar.Finish()

	return uploader.UploadAndRegister(r, size, osStderr)
}

func uploaderCheckWithProgress(pbar progress.ProgressBar, uploader cloud.Uploader) error {
	pr, pw := io.Pipe()
	defer pw.Close()

	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			pbar.SetMessagef("%s", scanner.Text())
		}
	}()
	return uploader.Check(pw)
}

func uploaderFor(cmd *cobra.Command, typeOrCloud string, targetArch string, bootMode *platform.BootMode, imagePath string) (cloud.Uploader, error) {
	switch typeOrCloud {
	case "ami", "generic-ami", "aws":
		return uploaderForCmdAWS(cmd, targetArch, bootMode)
	case "libvirt":
		return uploaderForLibvirt(cmd, targetArch, bootMode)
	case "openstack":
		return uploaderForCmdOpenstack(cmd, targetArch, bootMode)
	case "ibmcloud":
		return uploaderForCmdIbmCloud(cmd, targetArch, bootMode)
	case "azure":
		return uploaderForCmdAzure(cmd, targetArch, bootMode, imagePath)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUploadTypeUnsupported, typeOrCloud)
	}

}

func uploaderForCmdAWS(cmd *cobra.Command, targetArchStr string, bootMode *platform.BootMode) (cloud.Uploader, error) {
	amiName, err := cmd.Flags().GetString("aws-ami-name")
	if err != nil {
		return nil, err
	}
	bucketName, err := cmd.Flags().GetString("aws-bucket")
	if err != nil {
		return nil, err
	}
	region, err := cmd.Flags().GetString("aws-region")
	if err != nil {
		return nil, err
	}
	profile, err := cmd.Flags().GetString("aws-profile")
	if err != nil {
		return nil, err
	}
	tags, err := cmd.Flags().GetStringArray("aws-tag")
	if err != nil {
		return nil, err
	}
	var slicedTags []awscloud.AWSTag
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("Invalid tag format: %s (expected key=value)", tag)
		}
		slicedTags = append(slicedTags, awscloud.AWSTag{
			Name:  parts[0],
			Value: parts[1],
		})
	}
	if bootMode == nil {
		// If unset, default to BOOT_HYBIRD which translated
		// to "uefi-prefered" when registering the image.
		// This should give us wide compatibility. Ideally
		// we would introspect the image but we have no
		// metadata there right now.
		// XXX: move this into the "images" library itself?
		bootModeHybrid := platform.BOOT_HYBRID
		bootMode = &bootModeHybrid
	}
	targetArch, err := arch.FromString(targetArchStr)
	if err != nil {
		return nil, err
	}

	var missing []string
	requiredArgs := []string{"aws-ami-name", "aws-bucket", "aws-region"}
	for _, argName := range requiredArgs {
		arg, err := cmd.Flags().GetString(argName)
		if err != nil {
			return nil, err
		}
		if arg == "" {
			missing = append(missing, fmt.Sprintf("--%s", argName))
		}
	}
	if len(missing) > 0 {
		if len(missing) == len(requiredArgs) {
			return nil, fmt.Errorf("%w: %q", ErrUploadConfigNotProvided, missing)
		}

		return nil, fmt.Errorf("%w: %q", ErrMissingUploadConfig, missing)
	}
	opts := &awscloud.UploaderOptions{
		TargetArch: targetArch,
		BootMode:   bootMode,
		Tags:       slicedTags,
		Profile:    profile,
	}

	return awscloudNewUploader(region, bucketName, amiName, opts)
}

func uploaderForLibvirt(cmd *cobra.Command, targetArchStr string, bootMode *platform.BootMode) (cloud.Uploader, error) {
	connection, err := cmd.Flags().GetString("libvirt-connection")
	if err != nil {
		return nil, err
	}
	pool, err := cmd.Flags().GetString("libvirt-pool")
	if err != nil {
		return nil, err
	}
	volume, err := cmd.Flags().GetString("libvirt-volume")
	if err != nil {
		return nil, err
	}
	return libvirtNewUploader(connection, pool, volume)
}

func uploaderForCmdOpenstack(cmd *cobra.Command, targetArchStr string, bootMode *platform.BootMode) (cloud.Uploader, error) {
	image, err := cmd.Flags().GetString("openstack-image")
	if err != nil {
		return nil, err
	}
	if image == "" {
		return nil, fmt.Errorf("%w: %q", ErrUploadConfigNotProvided, []string{"--openstack-image"})
	}
	diskFormat, err := cmd.Flags().GetString("openstack-disk-format")
	if err != nil {
		return nil, err
	}
	containerFormat, err := cmd.Flags().GetString("openstack-container-format")
	if err != nil {
		return nil, err
	}
	opts := &openstack.UploaderOptions{
		DiskFormat:      diskFormat,
		ContainerFormat: containerFormat,
	}
	return openstackNewUploader(image, opts)
}

func uploaderForCmdIbmCloud(cmd *cobra.Command, targetArchStr string, bootMode *platform.BootMode) (cloud.Uploader, error) {
	bucketName, err := cmd.Flags().GetString("ibmcloud-bucket")
	if err != nil {
		return nil, err
	}
	region, err := cmd.Flags().GetString("ibmcloud-region")
	if err != nil {
		return nil, err
	}
	imageName, err := cmd.Flags().GetString("ibmcloud-image-name")
	if err != nil {
		return nil, err
	}
	apiKey := os.Getenv("IBMCLOUD_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("Please set your IBM Cloud API key as $IBMCLOUD_API_KEY")
	}
	crn := os.Getenv("IBMCLOUD_CRN")
	if crn == "" {
		return nil, fmt.Errorf("Please set your IBM Cloud Resource Name as $IBMCLOUD_CRN")
	}
	credentials := &ibmcloud.Credentials{
		AuthEndpoint: "https://iam.cloud.ibm.com/identity/token",
		ApiKey:       apiKey,
		Crn:          crn,
	}
	return ibmNewUploader(region, bucketName, imageName, credentials)
}

func uploaderForCmdAzure(cmd *cobra.Command, targetArchStr string, bootMode *platform.BootMode, imagePath string) (cloud.Uploader, error) {
	clientID, err := cmd.Flags().GetString("azure-client-id")
	if err != nil {
		return nil, err
	}
	clientSecret, err := cmd.Flags().GetString("azure-client-secret")
	if err != nil {
		return nil, err
	}
	tenant, err := cmd.Flags().GetString("azure-tenant")
	if err != nil {
		return nil, err
	}
	subscription, err := cmd.Flags().GetString("azure-subscription")
	if err != nil {
		return nil, err
	}
	resourceGroup, err := cmd.Flags().GetString("azure-resource-group")
	if err != nil {
		return nil, err
	}
	imageName, err := cmd.Flags().GetString("azure-image-name")
	if err != nil {
		return nil, err
	}

	var missing []string
	requiredArgs := []string{"azure-client-id", "azure-client-secret", "azure-tenant", "azure-subscription", "azure-resource-group", "azure-image-name"}
	for _, argName := range requiredArgs {
		arg, err := cmd.Flags().GetString(argName)
		if err != nil {
			return nil, err
		}
		if arg == "" {
			missing = append(missing, fmt.Sprintf("--%s", argName))
		}
	}
	if len(missing) > 0 {
		if len(missing) == len(requiredArgs) {
			return nil, fmt.Errorf("%w: %q", ErrUploadConfigNotProvided, missing)
		}
		return nil, fmt.Errorf("%w: %q", ErrMissingUploadConfig, missing)
	}

	targetArch, err := arch.FromString(targetArchStr)
	if err != nil {
		return nil, err
	}

	return azureNewUploader(clientID, clientSecret, tenant, subscription, resourceGroup, imageName, imagePath, targetArch)
}

func detectArchFromImagePath(imagePath string) string {
	// This detection is currently rather naive, we just look for
	// the file name and try to infer from that. We could extend
	// this to smartz like inspect the image via libguestfs or
	// add extra metadata to the image. But for now this is what
	// we got.

	// imagePath by default looks like
	//   /path/to/<disro>-<imgtype>-<arch>.img.xz
	// so try to infer the arch
	baseName := filepath.Base(imagePath)
	nameNoEx := strings.Split(baseName, ".")[0]
	frags := strings.Split(nameNoEx, "-")
	maybeArch := frags[len(frags)-1]
	if a, err := arch.FromString(maybeArch); err == nil {
		return a.String()
	}
	return ""
}

func bootModeFromFlag(cmd *cobra.Command) (*platform.BootMode, error) {
	bootModeFlag, err := cmd.Flags().GetString("aws-boot-mode")
	if err != nil {
		return nil, err
	}
	switch bootModeFlag {
	case "":
		return nil, nil
	case "legacy-bios":
		return common.ToPtr(platform.BOOT_LEGACY), nil
	case "uefi":
		return common.ToPtr(platform.BOOT_UEFI), nil
	case "uefi-preferred":
		return common.ToPtr(platform.BOOT_HYBRID), nil
	default:
		return nil, fmt.Errorf("invalid boot mode: %q, supported: legacy-bios, uefi, uefi-preferred", bootModeFlag)
	}
}

func cmdUpload(cmd *cobra.Command, args []string) error {
	imagePath := args[0]

	uploadTo, err := cmd.Flags().GetString("to")
	if err != nil {
		return err
	}
	if uploadTo == "" {
		return fmt.Errorf("missing --to parameter, try --to=aws")
	}

	targetArch, err := cmd.Flags().GetString("arch")
	if err != nil {
		return err
	}
	if targetArch == "" {
		targetArch = detectArchFromImagePath(imagePath)
		if targetArch != "" {
			fmt.Fprintf(osStderr, "Note: using architecture %q based on image filename (use --arch to override)\n", targetArch)
		}
		if targetArch == "" {
			targetArch = arch.Current().String()
			fmt.Fprintf(osStderr, "WARNING: no upload architecture specified, using %q (use --arch to override)\n", targetArch)
		}
	}

	bootMode, err := bootModeFromFlag(cmd)
	if err != nil {
		return err
	}

	uploader, err := uploaderFor(cmd, uploadTo, targetArch, bootMode, imagePath)
	if err != nil {
		return err
	}

	result, err := uploadImageWithProgress(uploader, imagePath)
	if err != nil {
		return err
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	switch format {
	case "", "yaml":
		output, err := yaml.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	case "json":
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		return fmt.Errorf("unsupported format %q, supported formats: yaml, json", format)
	}
	return nil
}
