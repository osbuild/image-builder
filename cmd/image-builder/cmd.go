package main

import (
	"fmt"
	"log"
	"os"

	"github.com/osbuild/image-builder/internal/olog"
	ilog "github.com/osbuild/image-builder/pkg/olog"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func setupRootCmd() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:   "image-builder",
		Short: "Build operating system images from a given distro/image-type/blueprint",
		Long: `Build operating system images from a given distribution,
image-type and blueprint.

Image-builder builds operating system images for a range of predefined
operating systems like Fedora, CentOS and RHEL with easy customizations support.`,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			// This is not the ideal way to implement this, but cobra does not
			// support lazy version strings and we need to call "osbuild" subprocess
			// to get its version.
			if versionFlag, err := cmd.Flags().GetBool("version"); err == nil && versionFlag {
				fmt.Fprintln(cmd.ErrOrStderr(), `Flag --version has been deprecated, use "image-builder version" instead`)
				fmt.Fprint(cmd.OutOrStdout(), prettyVersion())
			} else {
				_ = cmd.Help()
			}
		},
	}

	rootCmd.Flags().Bool("version", false, "Print version information and exit (deprecated: use \"image-builder version\" instead)")
	if err := rootCmd.Flags().MarkHidden("version"); err != nil {
		return nil, err
	}
	var forceRepoDir string
	rootCmd.PersistentFlags().StringVar(&forceRepoDir, "force-repo-dir", "", "Override the default repository search path for custom repository files")
	rootCmd.PersistentFlags().StringVar(&forceRepoDir, "force-data-dir", "", `Override the default data directory for e.g. custom repositories/*.json data`)
	if err := rootCmd.PersistentFlags().MarkDeprecated("force-data-dir", `Use --force-repo-dir instead`); err != nil {
		return nil, err
	}
	rootCmd.PersistentFlags().StringVar(&forceRepoDir, "data-dir", "", `Override the default data directory for e.g. custom repositories/*.json data`)
	if err := rootCmd.PersistentFlags().MarkDeprecated("data-dir", `Use --force-repo-dir instead`); err != nil {
		return nil, err
	}
	rootCmd.PersistentFlags().String("force-defs-dir", "", "Override the path to load YAML distro definitions from")
	if err := rootCmd.PersistentFlags().MarkHidden("force-defs-dir"); err != nil {
		return nil, err
	}
	rootCmd.PersistentFlags().StringArray("extra-repo", nil, `Add an extra repository during build (will *not* be gpg checked and not be part of the final image)`)
	rootCmd.PersistentFlags().StringArray("force-repo", nil, `Override the base repositories during build (these will not be part of the final image)`)
	rootCmd.PersistentFlags().String("output-dir", "", `Put output into the specified directory`)
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, `Switch to verbose mode (more logging on stderr and verbose progress)`)
	registerMemProfileFlags(rootCmd)
	rootCmd.PersistentPreRun = memProfilePersistentPreRun

	rootCmd.SetOut(osStdout)
	rootCmd.SetErr(osStderr)

	bootcCmd, err := setupBootcCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(bootcCmd)

	listCmd := setupListCmd()
	rootCmd.AddCommand(listCmd)

	versionCmd := setupVersionCmd()
	rootCmd.AddCommand(versionCmd)

	systemCmd := setupSystemCmd()
	rootCmd.AddCommand(systemCmd)

	manifestCmd, err := setupManifestCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(manifestCmd)

	uploadCmd := setupUploadCmd()
	rootCmd.AddCommand(uploadCmd)

	buildCmd, err := setupBuildCmd()
	if err != nil {
		return nil, err
	}
	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	// The build command can upload images to the appropriate cloud provider,
	// so it should support the upload options as well
	buildCmd.Flags().AddFlagSet(uploadCmd.Flags())
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().AddFlagSet(uploadCmd.Flags())
	// add after the rest of the uploadCmd flag set is added to avoid
	// that build gets a "--to" parameter
	uploadCmd.Flags().String("to", "", "upload to the given cloud")

	describeCmd := setupDescribeCmd()
	rootCmd.AddCommand(describeCmd)

	pkgSearchCmd := setupPkgSearchCmd()
	rootCmd.AddCommand(pkgSearchCmd)

	docCmd := setupDocCmd(rootCmd)
	rootCmd.AddCommand(docCmd)

	verbose, err := rootCmd.PersistentFlags().GetBool("verbose")
	if err != nil {
		return nil, err
	}
	if verbose {
		olog.SetDefault(log.New(os.Stderr, "", 0))
		ilog.SetDefault(log.New(os.Stderr, "", 0))
	}

	return rootCmd, nil
}

func setupBootcCmd() (*cobra.Command, error) {
	bootcCmd := &cobra.Command{
		Use:   "bootc",
		Short: "bootc-related commands",
		Args:  cobra.NoArgs,
	}

	bootcInspectCommand := &cobra.Command{
		Use:   "inspect",
		Short: "Show data gathered by `image-builder` for a container",
		RunE:  cmdBootcInspect,
		Args:  cobra.NoArgs,
	}
	bootcInspectCommand.Flags().String("ref", "", `bootc container ref`)
	if err := bootcInspectCommand.MarkFlagRequired("ref"); err != nil {
		return nil, err
	}
	bootcInspectCommand.Flags().String("format", "", "Output in a specific format (yaml, json)")
	bootcCmd.AddCommand(bootcInspectCommand)

	return bootcCmd, nil
}

func setupListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:          "list",
		Short:        "List buildable images, use --filter to limit further",
		RunE:         cmdListImages,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Aliases:      []string{"list-images"},
	}
	listCmd.Flags().StringArray("filter", nil, `Filter distributions by a specific criteria (e.g. "type:iot*")`)
	listCmd.Flags().String("format", "", "Output in a specific format (text, json)")

	return listCmd
}

func setupVersionCmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE:  cmdVersion,
		Args:  cobra.NoArgs,
	}
	versionCmd.Flags().String("format", "", "Output in a specific format (yaml, json)")

	return versionCmd
}

func setupSystemCmd() *cobra.Command {
	systemCmd := &cobra.Command{
		Use:   "system",
		Short: "Show system status information",
		RunE:  cmdSystem,
		Args:  cobra.NoArgs,
	}
	systemCmd.Flags().String("format", "", "Output in a specific format (yaml, json)")

	return systemCmd
}

func setupManifestCmd() (*cobra.Command, error) {
	manifestCmd := &cobra.Command{
		Use:          "manifest <image-type>",
		Short:        "Build manifest for the given image-type, e.g. qcow2 (tip: combine with --distro, --arch)",
		RunE:         cmdManifest,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Hidden:       true,
	}
	manifestCmd.Flags().String("blueprint", "", `filename of a blueprint to customize an image`)
	manifestCmd.Flags().Int64("seed", 0, `rng seed, some values are derived randomly, pinning the seed allows more reproducibility if you need it. must be an integer. only used when changed.`)
	manifestCmd.Flags().String("arch", "", `build manifest for a different architecture`)
	manifestCmd.Flags().String("distro", "", `build manifest for a different distroname (e.g. centos-9)`)
	manifestCmd.Flags().String("ostree-ref", "", `OSTREE reference`)
	manifestCmd.Flags().String("ostree-parent", "", `OSTREE parent`)
	manifestCmd.Flags().String("ostree-url", "", `OSTREE url`)
	manifestCmd.Flags().String("bootc-ref", "", `bootc container ref`)
	manifestCmd.Flags().String("bootc-build-ref", "", `bootc build container ref`)
	manifestCmd.Flags().String("bootc-installer-payload-ref", "", `bootc installer payload ref`)
	manifestCmd.Flags().String("bootc-default-fs", "", `default filesystem to use for the bootc install (e.g. ext4)`)
	manifestCmd.Flags().Bool("bootc-no-default-kernel-args", false, `don't use the default kernel arguments`)
	manifestCmd.Flags().Bool("bootc-pull-container", false, `pull bootc container from remote location instead of using it from local container storage`)
	manifestCmd.Flags().Uint64("image-size", 0, `override the default image size in bytes`)
	manifestCmd.Flags().Bool("use-librepo", true, `use librepo to download packages (disable if you use old versions of osbuild)`)
	if err := manifestCmd.Flags().MarkHidden("use-librepo"); err != nil {
		return nil, err
	}
	manifestCmd.Flags().Bool("with-sbom", false, `export SPDX SBOM document`)
	manifestCmd.Flags().Bool("with-rpmlist", false, `export RPM list as JSON`)
	if err := manifestCmd.Flags().MarkHidden("with-rpmlist"); err != nil {
		return nil, err
	}
	manifestCmd.Flags().Bool("ignore-warnings", false, `ignore warnings during manifest generation`)
	manifestCmd.Flags().String("registrations", "", `filename of a registrations file with e.g. subscription details`)
	manifestCmd.Flags().String("rpmmd-cache", "", `osbuild directory to cache rpm metadata`)
	manifestCmd.Flags().Bool("preview", true, `override distro default preview state if passed`)
	if err := manifestCmd.Flags().MarkHidden("preview"); err != nil {
		return nil, err
	}
	return manifestCmd, nil
}

func setupUploadCmd() *cobra.Command {
	uploadCmd := &cobra.Command{
		Use:          "upload <image-path>",
		Short:        "Upload the given image from <image-path>",
		RunE:         cmdUpload,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}
	uploadCmd.Flags().String("aws-ami-name", "", "name for the AMI in AWS (only for type=ami)")
	uploadCmd.Flags().String("aws-bucket", "", "target S3 bucket name for intermediate storage when creating AMI (only for type=ami)")
	uploadCmd.Flags().String("aws-region", "", "target region for AWS uploads (only for type=ami)")
	uploadCmd.Flags().String("aws-profile", "", "name of the AWS credentials profile (only for type=aws)")
	uploadCmd.Flags().StringArray("aws-tag", []string{}, "tag the AMI with this Key=Value (only for type=aws)")
	uploadCmd.Flags().String("aws-boot-mode", "", "boot mode for the AMI: legacy-bios, uefi, uefi-preferred (only for type=aws)")
	uploadCmd.Flags().String("libvirt-connection", "", "connection URI (only for type=libvirt)")
	uploadCmd.Flags().String("libvirt-pool", "", "pool name (only for type=libvirt)")
	uploadCmd.Flags().String("libvirt-volume", "", "volume name (only for type=libvirt)")
	uploadCmd.Flags().String("openstack-image", "", "name for the uploaded image (only for type=openstack)")
	uploadCmd.Flags().String("openstack-disk-format", "raw", "the disk format of a virtual machine image (only for type=openstack)")
	uploadCmd.Flags().String("openstack-container-format", "bare", "this indicates if the image contains metadata about the VM (only for type=openstack)")
	uploadCmd.Flags().String("ibmcloud-bucket", "", "target bucket name for storing the image (only for type=ibmcloud)")
	uploadCmd.Flags().String("ibmcloud-region", "", "target region for IBM Cloud uploads (only for type=ibmcloud)")
	uploadCmd.Flags().String("ibmcloud-image-name", "", "name for the uploaded image (only for type=ibmcloud)")
	uploadCmd.Flags().String("azure-client-id", "", "Azure client ID (only for type=azure)")
	uploadCmd.Flags().String("azure-client-secret", "", "Azure client secret (only for type=azure)")
	uploadCmd.Flags().String("azure-tenant", "", "Azure tenant ID (only for type=azure)")
	uploadCmd.Flags().String("azure-subscription", "", "Azure subscription ID (only for type=azure)")
	uploadCmd.Flags().String("azure-resource-group", "", "Azure resource group (only for type=azure)")
	uploadCmd.Flags().String("azure-image-name", "", "name for the uploaded image (only for type=azure)")
	uploadCmd.Flags().String("arch", "", "upload for the given architecture")
	uploadCmd.Flags().String("format", "", "output in a specific format (yaml, json)")

	return uploadCmd
}

func setupBuildCmd() (*cobra.Command, error) {
	buildCmd := &cobra.Command{
		Use:          "build <image-type>",
		Short:        "Build the given image-type, e.g. qcow2 (tip: combine with --distro, --arch)",
		RunE:         cmdBuild,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}
	buildCmd.Flags().Bool("with-manifest", false, `export osbuild manifest`)
	buildCmd.Flags().Bool("with-buildlog", false, `export osbuild buildlog`)
	buildCmd.Flags().String("cache", defaultCacheDir(), `osbuild directory to cache intermediate build artifacts"`)
	// XXX: add "--verbose" here, similar to how bib is doing this
	// (see https://github.com/osbuild/bootc-image-builder/pull/790/commits/5cec7ffd8a526e2ca1e8ada0ea18f927695dfe43)
	buildCmd.Flags().String("progress", "auto", "type of progress bar to use (e.g. verbose,term)")
	buildCmd.Flags().Bool("with-metrics", false, `print timing information at the end of the build`)
	buildCmd.Flags().String("output-name", "", "set specific output basename")
	buildCmd.Flags().Bool("in-vm", false, `run the osbuild pipeline in a virtual machine`)
	buildCmd.Flags().String("format", "", "Output in a specific format (json)")
	// hide this flag for now, this is only relevant for cockpit-image-builder
	buildCmd.Flags().Bool("with-upload-result", false, `export upload result`)
	if err := buildCmd.Flags().MarkHidden("with-upload-result"); err != nil {
		return nil, err
	}

	return buildCmd, nil
}

func setupDescribeCmd() *cobra.Command {
	// XXX: add --format=json too?
	describeCmd := &cobra.Command{
		Use:          "describe <image-type>",
		Short:        "Describe the given image-type, e.g. qcow2 (tip: combine with --distro,--arch)",
		RunE:         cmdDescribeImg,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Hidden:       false,
		Aliases:      []string{"describe-image"},
	}
	describeCmd.Flags().String("arch", "", `use the different architecture`)
	describeCmd.Flags().String("distro", "", `build manifest for a different distroname (e.g. centos-9)`)
	describeCmd.Flags().Bool("in-vm", false, `run container in a virtual machine`)

	return describeCmd
}

func setupPkgSearchCmd() *cobra.Command {
	pkgSearchCmd := &cobra.Command{
		Use:          "pkgsearch [pkg1 pkg2 ...]",
		Short:        "Search for packages available for the given distro (tip: combine with --distro, --arch, --type)",
		RunE:         cmdPkgSearch,
		SilenceUsage: true,
		Hidden:       true,
	}
	pkgSearchCmd.Flags().String("format", "", "Output in a specific format (json)")
	pkgSearchCmd.Flags().String("distro", "", "Search packages for a specific distro (e.g. centos-9)")
	pkgSearchCmd.Flags().String("arch", "", "Search packages for a specific architecture")
	pkgSearchCmd.Flags().String("type", "", "Narrow search to repos for a specific image type (e.g. qcow2)")
	pkgSearchCmd.Flags().String("rpmmd-cache", "", `osbuild directory to cache rpm metadata`)

	return pkgSearchCmd
}

func setupDocCmd(rootCmd *cobra.Command) *cobra.Command {
	docCmd := &cobra.Command{
		Use:    "doc <output-dir>",
		Short:  "Generate man pages for this command",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			header := &doc.GenManHeader{
				Section: "1",
			}
			return doc.GenManTree(rootCmd, header, args[0])
		},
	}
	return docCmd
}
