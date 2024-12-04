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
	archStr, err := cmd.Flags().GetString("arch")
	if err != nil {
		return err
	}
	if archStr == "" {
		archStr = arch.Current().String()
	}
	distroStr, err := cmd.Flags().GetString("distro")
	if err != nil {
		return err
	}

	var blueprintPath string
	imgTypeStr := args[0]
	if len(args) > 1 {
		blueprintPath = args[1]
	}
	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return err
	}
	distroStr, err = findDistro(distroStr, bp.Distro)
	if err != nil {
		return err
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

	return rootCmd.Execute()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(osStderr, "error: %s\n", err)
		os.Exit(1)
	}
}
