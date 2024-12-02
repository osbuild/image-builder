package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/osbuild/images/pkg/arch"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
	"github.com/osbuild/image-builder-cli/internal/manifestgen"
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

func cmdManifest(cmd *cobra.Command, args []string) error {
	dataDir, err := cmd.Flags().GetString("datadir")
	if err != nil {
		return err
	}
	blueprintPath, err := cmd.Flags().GetString("blueprint")
	if err != nil {
		return err
	}

	distroStr := args[0]
	imgTypeStr := args[1]
	var archStr string
	if len(args) > 2 {
		archStr = args[2]
	} else {
		archStr = arch.Current().String()
	}
	res, err := getOneImage(dataDir, distroStr, imgTypeStr, archStr)
	if err != nil {
		return err
	}
	repos, err := newRepoRegistry(dataDir)
	if err != nil {
		return err
	}
	// XXX: add --rpmmd/cachedir option like bib
	mg, err := manifestgen.New(repos, &manifestgen.Options{
		Output: osStdout,
	})
	if err != nil {
		return err
	}
	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return err
	}

	return mg.Generate(bp, res.Distro, res.ImgType, res.Arch, nil)
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
		Use:          "manifest <distro> <image-type> [<arch>]",
		Short:        "Build manifest for the given distro/image-type, e.g. centos-9 qcow2",
		RunE:         cmdManifest,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(2),
		Hidden:       true,
	}
	// XXX: share with build
	manifestCmd.Flags().String("blueprint", "", `pass a blueprint file`)
	rootCmd.AddCommand(manifestCmd)

	return rootCmd.Execute()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(osStderr, "error: %s\n", err)
		os.Exit(1)
	}
}
