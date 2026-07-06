package main

import (
	"fmt"
	"os"

	"github.com/osbuild/image-builder/internal/bibimg"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
)

func setupBibRootCmd() (*cobra.Command, error) {
	version, err := bibVersionFromBuildInfo()
	if err != nil {
		return nil, err
	}

	rootCmd := &cobra.Command{
		Use:               "bootc-image-builder",
		Long:              "Build operating system images from bootc containers",
		PersistentPreRunE: bibRootPreRunE,
		SilenceErrors:     true,
		Version:           version,
	}
	rootCmd.SetVersionTemplate(version)

	rootCmd.PersistentFlags().StringVar(&rootLogLevel, "log-level", "", "logging level (debug, info, error); default error")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, `Switch to verbose mode`)

	buildCmd := setupBibBuildCmd()
	if err != nil {
		return nil, err
	}
	buildCmd.SetVersionTemplate(version)
	buildCmd.Version = rootCmd.Version
	rootCmd.AddCommand(buildCmd)

	manifestCmd, err := setupBibManifestCmd()
	if err != nil {
		return nil, err
	}
	manifestCmd.Version = rootCmd.Version
	manifestCmd.SetVersionTemplate(version)
	rootCmd.AddCommand(manifestCmd)

	// add manifest flags to the build subcommand
	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	// flag rules
	for _, dname := range []string{"output", "store", "rpmmd"} {
		if err := buildCmd.MarkFlagDirname(dname); err != nil {
			return nil, err
		}
	}
	if err := buildCmd.MarkFlagFilename("config"); err != nil {
		return nil, err
	}

	versionCmd := setupBibVersionCmd()
	rootCmd.AddCommand(versionCmd)

	// If no subcommand is given, assume the user wants to use the build subcommand
	// See https://github.com/spf13/cobra/issues/823#issuecomment-870027246
	// which cannot be used verbatim because the arguments for "build" like
	// "quay.io" will create an "err != nil". Ideally we could check err
	// for something like cobra.UnknownCommandError but cobra just gives
	// us an error string
	cmd, _, err := rootCmd.Find(os.Args[1:])
	injectBuildArg := func() {
		args := append([]string{buildCmd.Name()}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}
	// command not known, i.e. happens for "bib quay.io/centos/..."
	if err != nil && !slices.Contains([]string{"help", "completion"}, os.Args[1]) {
		injectBuildArg()
	}
	// command appears valid, e.g. "bib --local quay.io/centos" but this
	// is the parser just assuming "quay.io" is an argument for "--local" :(
	if err == nil && cmd.Use == rootCmd.Use && cmd.Flags().Parse(os.Args[1:]) != pflag.ErrHelp {
		injectBuildArg()
	}

	return rootCmd, nil
}

func setupBibBuildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:                   "build IMAGE_NAME",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  bibCmdBuild,
		SilenceUsage:          true,
	}
	buildCmd.Flags().String("aws-ami-name", "", "name for the AMI in AWS (only for type=ami)")
	buildCmd.Flags().String("aws-bucket", "", "target S3 bucket name for intermediate storage when creating AMI (only for type=ami)")
	buildCmd.Flags().String("aws-region", "", "target region for AWS uploads (only for type=ami)")
	buildCmd.Flags().String("chown", "", "chown the ouput directory to match the specified UID:GID")
	buildCmd.Flags().String("output", ".", "artifact output directory")
	buildCmd.Flags().String("store", "/store", "osbuild store for intermediate pipeline trees")
	//TODO: add json progress for higher level tools like "podman bootc"
	buildCmd.Flags().String("progress", "auto", "type of progress bar to use (e.g. verbose,term)")

	buildCmd.MarkFlagsRequiredTogether("aws-region", "aws-bucket", "aws-ami-name")

	return buildCmd
}

func setupBibManifestCmd() (*cobra.Command, error) {
	manifestCmd := &cobra.Command{
		Use:                   "manifest",
		Short:                 "Only create the manifest but don't build the image.",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  bibCmdManifest,
		SilenceUsage:          true,
	}

	manifestCmd.Flags().Bool("tls-verify", false, "DEPRECATED: require HTTPS and verify certificates when contacting registries")
	if err := manifestCmd.Flags().MarkHidden("tls-verify"); err != nil {
		return nil, fmt.Errorf("cannot hide 'tls-verify' :%w", err)
	}
	manifestCmd.Flags().String("rpmmd", "/rpmmd", "rpm metadata cache directory")
	manifestCmd.Flags().String("target-arch", "", "build for the given target architecture (experimental)")
	manifestCmd.Flags().String("build-container", "", "Use a custom container for the image build")
	// XXX: add --bootc-installer-payload-ref as alias to make it
	// cmdline compatible with ibcli(?)
	manifestCmd.Flags().String("installer-payload-ref", "", "bootc installer payload ref")
	manifestCmd.Flags().StringArray("type", []string{"qcow2"}, fmt.Sprintf("image types to build [%s]", bibimg.Available()))
	manifestCmd.Flags().Bool("local", true, "DEPRECATED: --local is now the default behavior, make sure to pull the container image before running bootc-image-builder")
	if err := manifestCmd.Flags().MarkHidden("local"); err != nil {
		return nil, fmt.Errorf("cannot hide 'local' :%w", err)
	}
	manifestCmd.Flags().String("rootfs", "", "Root filesystem type. If not given, the default configured in the source container image is used.")
	manifestCmd.Flags().Bool("use-librepo", true, "switch to librepo for pkg download, needs new enough osbuild")
	// --config is only useful for developers who run bib outside
	// of a container to generate a manifest. so hide it by
	// default from users.
	manifestCmd.Flags().String("config", "", "build config file; /config.json will be used if present")
	if err := manifestCmd.Flags().MarkHidden("config"); err != nil {
		return nil, fmt.Errorf("cannot hide 'config' :%w", err)
	}

	return manifestCmd, nil
}

func setupBibVersionCmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:          "version",
		Short:        "Show the version and quit",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			root.SetArgs([]string{"--version"})
			return root.Execute()
		},
	}

	return versionCmd
}
