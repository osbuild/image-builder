package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"

	"github.com/osbuild/image-builder-cli/pkg/progress"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/platform"
)

// ErrMissingUploadConfig is returned when the upload configuration is missing
var ErrMissingUploadConfig = errors.New("missing upload configuration")

// ErrUploadConfigNotProvided is returned when all the upload configuration is missing
var ErrUploadConfigNotProvided = errors.New("missing all upload configuration")

// ErrUploadTypeUnsupported is returned when the upload type is not supported
var ErrUploadTypeUnsupported = errors.New("unsupported type")

var awscloudNewUploader = awscloud.NewUploader

func uploadImageWithProgress(uploader cloud.Uploader, imagePath string) error {
	f, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// setup basic progress
	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat upload: %v", err)
	}
	pbar := pb.New64(st.Size())
	pbar.Set(pb.Bytes, true)
	pbar.SetWriter(osStdout)
	r := pbar.NewProxyReader(f)
	pbar.Start()
	defer pbar.Finish()

	return uploader.UploadAndRegister(r, osStderr)
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

func uploaderFor(cmd *cobra.Command, typeOrCloud string, targetArch string, bootMode *platform.BootMode) (cloud.Uploader, error) {
	switch typeOrCloud {
	case "ami", "aws":
		return uploaderForCmdAWS(cmd, targetArch, bootMode)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUploadTypeUnsupported, typeOrCloud)
	}

}

func uploaderForCmdAWS(cmd *cobra.Command, targetArch string, bootMode *platform.BootMode) (cloud.Uploader, error) {
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
	}

	return awscloudNewUploader(region, bucketName, amiName, opts)
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
	nameNoEx := strings.SplitN(baseName, ".", -1)[0]
	frags := strings.Split(nameNoEx, "-")
	maybeArch := frags[len(frags)-1]
	if a, err := arch.FromString(maybeArch); err == nil {
		return a.String()
	}
	return ""
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

	uploader, err := uploaderFor(cmd, uploadTo, targetArch, nil)
	if err != nil {
		return err
	}

	return uploadImageWithProgress(uploader, imagePath)
}
