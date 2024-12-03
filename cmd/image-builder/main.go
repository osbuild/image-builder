package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/imagefilter"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
)

var (
	osStdout io.Writer = os.Stdout
	osStderr io.Writer = os.Stderr
)

func cmdListImages(cmd *cobra.Command, args []string) error {
	filter, err := cmd.Flags().GetStringArray("filter")
	if err != nil {
		return err
	}
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	dataDir, err := cmd.Flags().GetString("datadir")
	if err != nil {
		return err
	}

	return listImages(dataDir, output, filter)
}

func cmdManifestWrapper(cmd *cobra.Command, args []string, w io.Writer, archChecker func(string) error) (*imagefilter.Result, error) {
	dataDir, err := cmd.Flags().GetString("datadir")
	if err != nil {
		return nil, err
	}
	archStr, err := cmd.Flags().GetString("arch")
	if err != nil {
		return nil, err
	}
	if archStr == "" {
		archStr = arch.Current().String()
	}
	distroStr, err := cmd.Flags().GetString("distro")
	if err != nil {
		return nil, err
	}

	var blueprintPath string
	imgTypeStr := args[0]
	if len(args) > 1 {
		blueprintPath = args[1]
	}
	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return nil, err
	}
	distroStr, err = findDistro(distroStr, bp.Distro)
	if err != nil {
		return nil, err
	}

	res, err := getOneImage(dataDir, distroStr, imgTypeStr, archStr)
	if err != nil {
		return nil, err
	}
	if archChecker != nil {
		if err := archChecker(res.Arch.Name()); err != nil {
			return nil, err
		}
	}

	err = generateManifest(dataDir, blueprintPath, res, w)
	return res, err
}

func cmdManifest(cmd *cobra.Command, args []string) error {
	_, err := cmdManifestWrapper(cmd, args, osStdout, nil)
	return err
}

func cmdBuild(cmd *cobra.Command, args []string) error {
	var mf bytes.Buffer

	// XXX: check env here, i.e. if user is root and osbuild is installed
	res, err := cmdManifestWrapper(cmd, args, &mf, func(archStr string) error {
		if archStr != arch.Current().String() {
			return fmt.Errorf("cannot build for arch %q from %q", archStr, arch.Current().String())
		}
		return nil
	})
	if err != nil {
		return err
	}

	return buildImage(res, mf.Bytes())
}

func run() error {
	// images logs a bunch of stuff to Debug/Info that is distracting
	// the user (at least by default, like what repos being loaded)
	logrus.SetLevel(logrus.WarnLevel)

	rootCmd := &cobra.Command{
		Use:   "image-builder",
		Short: "Build operating system images from a given distro/image-type/blueprint",
		Long: `Build operating system images from a given distribution,
image-type and blueprint.

Image-builder builds operating system images for a range of predefined
operating sytsems like centos and RHEL with easy customizations support.`,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().String("datadir", "", `Override the default data direcotry for e.g. custom repositories/*.json data`)
	rootCmd.SetOut(osStdout)
	rootCmd.SetErr(osStderr)

	listImagesCmd := &cobra.Command{
		Use:          "list-images",
		Short:        "List buildable images, use --filter to limit further",
		RunE:         cmdListImages,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
	}
	listImagesCmd.Flags().StringArray("filter", nil, `Filter distributions by a specific criteria (e.g. "type:rhel*")`)
	listImagesCmd.Flags().String("output", "", "Output in a specific format (text, json)")
	rootCmd.AddCommand(listImagesCmd)

	manifestCmd := &cobra.Command{
		Use:          "manifest <image-type> [blueprint]",
		Short:        "Build manifest for the given distro/image-type, e.g. centos-9 qcow2",
		RunE:         cmdManifest,
		SilenceUsage: true,
		Args:         cobra.RangeArgs(1, 2),
		Hidden:       true,
	}
	manifestCmd.Flags().String("arch", "", `build manifest for a different architecture`)
	manifestCmd.Flags().String("distro", "", `build manifest for a different distroname (e.g. centos-9)`)
	rootCmd.AddCommand(manifestCmd)

	buildCmd := &cobra.Command{
		Use:          "build <image-type> [blueprint]",
		Short:        "Build the given distro/image-type, e.g. centos-9 qcow2",
		RunE:         cmdBuild,
		SilenceUsage: true,
		Args:         cobra.RangeArgs(1, 2),
	}
	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	rootCmd.AddCommand(buildCmd)

	return rootCmd.Execute()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(osStderr, "error: %s\n", err)
		os.Exit(1)
	}
}
