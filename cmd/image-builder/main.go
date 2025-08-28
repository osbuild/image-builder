package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/osbuild/image-builder-cli/pkg/progress"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/distro/bootc"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
	"github.com/osbuild/image-builder-cli/internal/olog"
)

var (
	osStdout io.Writer = os.Stdout
	osStderr io.Writer = os.Stderr
)

// basenameFor returns the basename for directory and filenames
// for the given imageType. This can be user overriden via userBasename.
func basenameFor(img *imagefilter.Result, userBasename string) string {
	if userBasename != "" {
		// If the user provided a basename that already has the
		// image extension just strip that off. I.e. when
		// we get "foo.qcow2" for a qcow out basename is just
		// "foo". This is mostly for convenience for our users.
		//
		// This code assumes that all our ImgType filesnames have
		// $name.$ext.$extraExt (e.g. disk.qcow2 or disk.raw.xz)
		l := strings.SplitN(img.ImgType.Filename(), ".", 2)
		if len(l) > 1 && l[1] != "" {
			imgExt := fmt.Sprintf(".%s", l[1])
			userBasename = strings.TrimSuffix(userBasename, imgExt)
		}
		return userBasename
	}
	return fmt.Sprintf("%s-%s-%s", img.Distro.Name(), img.ImgType.Name(), img.Arch.Name())
}

func cmdListImages(cmd *cobra.Command, args []string) error {
	filter, err := cmd.Flags().GetStringArray("filter")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	dataDir, err := cmd.Flags().GetString("data-dir")
	if err != nil {
		return err
	}
	extraRepos, err := cmd.Flags().GetStringArray("extra-repo")
	if err != nil {
		return err
	}

	return listImages(dataDir, extraRepos, format, filter)
}

func ostreeImageOptions(cmd *cobra.Command) (*ostree.ImageOptions, error) {
	imageRef, err := cmd.Flags().GetString("ostree-ref")
	if err != nil {
		return nil, err
	}
	parentRef, err := cmd.Flags().GetString("ostree-parent")
	if err != nil {
		return nil, err
	}
	url, err := cmd.Flags().GetString("ostree-url")
	if err != nil {
		return nil, err
	}
	if imageRef == "" && parentRef == "" && url == "" {
		return nil, nil
	}

	// XXX: how to add RHSM?
	return &ostree.ImageOptions{
		ImageRef:  imageRef,
		ParentRef: parentRef,
		URL:       url,
	}, nil
}

type registrations struct {
	Redhat struct {
		Subscription *subscription.ImageOptions `json:"subscription,omitempty"`
	} `json:"redhat,omitempty"`
}

func subscriptionImageOptions(cmd *cobra.Command) (*subscription.ImageOptions, error) {
	regFilePath, err := cmd.Flags().GetString("registrations")
	if err != nil {
		return nil, err
	}
	if regFilePath == "" {
		return nil, nil
	}

	f, err := os.Open(regFilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open registrations file: %w", err)
	}
	defer f.Close()

	// XXX: support yaml eventually
	var regs registrations
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&regs); err != nil {
		return nil, fmt.Errorf("cannot parse registrations file: %w", err)
	}
	return regs.Redhat.Subscription, nil
}

type cmdManifestWrapperOptions struct {
	useBootstrapIfNeeded bool
}

func cmdManifestWrapper(pbar progress.ProgressBar, cmd *cobra.Command, args []string, w io.Writer, wd io.Writer, wrapperOpts *cmdManifestWrapperOptions) (*imagefilter.Result, error) {
	if wrapperOpts == nil {
		wrapperOpts = &cmdManifestWrapperOptions{}
	}
	dataDir, err := cmd.Flags().GetString("data-dir")
	if err != nil {
		return nil, err
	}
	extraRepos, err := cmd.Flags().GetStringArray("extra-repo")
	if err != nil {
		return nil, err
	}
	forceRepos, err := cmd.Flags().GetStringArray("force-repo")
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
	withSBOM, err := cmd.Flags().GetBool("with-sbom")
	if err != nil {
		return nil, err
	}
	ignoreWarnings, err := cmd.Flags().GetBool("ignore-warnings")
	if err != nil {
		return nil, err
	}
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return nil, err
	}
	ostreeImgOpts, err := ostreeImageOptions(cmd)
	if err != nil {
		return nil, err
	}
	useLibrepo, err := cmd.Flags().GetBool("use-librepo")
	if err != nil {
		return nil, err
	}
	var rpmDownloader osbuild.RpmDownloader
	if useLibrepo {
		rpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	blueprintPath, err := cmd.Flags().GetString("blueprint")
	if err != nil {
		return nil, err
	}
	var customSeed *int64
	if cmd.Flags().Changed("seed") {
		seedFlagVal, err := cmd.Flags().GetInt64("seed")
		if err != nil {
			return nil, err
		}
		customSeed = &seedFlagVal
	}
	subscription, err := subscriptionImageOptions(cmd)
	if err != nil {
		return nil, err
	}
	bootcRef, err := cmd.Flags().GetString("bootc-ref")
	if err != nil {
		return nil, err
	}
	if bootcRef != "" && distroStr != "" {
		return nil, fmt.Errorf("cannot use --distro with --bootc-ref")
	}
	// XXX: remove once https://github.com/osbuild/images/pull/1797
	// and https://github.com/osbuild/bootc-image-builder/pull/1014
	// are merged
	if bootcRef != "" {
		fmt.Fprintln(os.Stderr, "WARNING: bootc support is experimental")
	}

	// no error check here as this is (deliberately) not defined on
	// "manifest" (if "images" learn to set the output filename in
	// manifests we would change this
	outputFilename, _ := cmd.Flags().GetString("output-name")

	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return nil, err
	}
	if bootcRef == "" {
		distroStr, err = findDistro(distroStr, bp.Distro)
		if err != nil {
			return nil, err
		}
	} else {
		distroStr = "bootc-based"
	}

	imgTypeStr := args[0]
	pbar.SetPulseMsgf("Manifest generation step")
	pbar.SetMessagef("Building manifest for %s-%s", distroStr, imgTypeStr)

	var img *imagefilter.Result
	if bootcRef != "" {
		distro, err := bootc.NewBootcDistro(bootcRef)
		if err != nil {
			return nil, err
		}
		archi, err := distro.GetArch(archStr)
		if err != nil {
			return nil, err
		}
		imgType, err := archi.GetImageType(imgTypeStr)
		if err != nil {
			return nil, err
		}
		img = &imagefilter.Result{distro, archi, imgType}
		// XXX: hack to skip repo loading for the bootc image.
		// We need to add a SkipRepositories or similar to
		// manifestgen instead to make this clean
		forceRepos = []string{"https://example.com/not-used"}
	} else {
		repoOpts := &repoOptions{
			DataDir:    dataDir,
			ExtraRepos: extraRepos,
			ForceRepos: forceRepos,
		}
		img, err = getOneImage(distroStr, imgTypeStr, archStr, repoOpts)
		if err != nil {
			return nil, err
		}
	}
	if len(img.ImgType.Exports()) > 1 {
		return nil, fmt.Errorf("image %q has multiple exports: this is current unsupport: please report this as a bug", basenameFor(img, ""))
	}

	opts := &manifestOptions{
		OutputDir:      outputDir,
		OutputFilename: outputFilename,
		BlueprintPath:  blueprintPath,
		Ostree:         ostreeImgOpts,
		BootcRef:       bootcRef,
		RpmDownloader:  rpmDownloader,
		WithSBOM:       withSBOM,
		IgnoreWarnings: ignoreWarnings,
		CustomSeed:     customSeed,
		Subscription:   subscription,

		ForceRepos: forceRepos,
	}
	opts.UseBootstrapContainer = wrapperOpts.useBootstrapIfNeeded && (img.Arch.Name() != arch.Current().String())
	if opts.UseBootstrapContainer {
		fmt.Fprintf(os.Stderr, "WARNING: using experimental cross-architecture building to build %q\n", img.Arch.Name())
	}

	err = generateManifest(dataDir, extraRepos, img, w, wd, opts)
	return img, err
}

func cmdManifest(cmd *cobra.Command, args []string) error {
	pbar, err := progress.New("")
	if err != nil {
		return err
	}
	_, err = cmdManifestWrapper(pbar, cmd, args, osStdout, io.Discard, nil)
	return err
}

func progressFromCmd(cmd *cobra.Command) (progress.ProgressBar, error) {
	progressType, err := cmd.Flags().GetString("progress")
	if err != nil {
		return nil, err
	}
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return nil, err
	}
	if progressType == "auto" && verbose {
		progressType = "verbose"
	}

	return progress.New(progressType)
}

// listOutputdir will return a string with the output dir content.
// Any errors will also just appear as part of the string (as it is
// purely informational)
func listOutputdir(path string) string {
	ents, err := filepath.Glob(filepath.Join(path, "/*"))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return strings.Join(ents, ",")
}

func cmdBuild(cmd *cobra.Command, args []string) error {
	cacheDir, err := cmd.Flags().GetString("cache")
	if err != nil {
		return err
	}
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return err
	}
	outputBasename, err := cmd.Flags().GetString("output-name")
	if err != nil {
		return err
	}
	withManifest, err := cmd.Flags().GetBool("with-manifest")
	if err != nil {
		return err
	}
	withBuildlog, err := cmd.Flags().GetBool("with-buildlog")
	if err != nil {
		return err
	}
	// XXX: check env here, i.e. if user is root and osbuild is installed

	pbar, err := progressFromCmd(cmd)
	if err != nil {
		return err
	}
	pbar.Start()
	defer pbar.Stop()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		pbar.Stop()
	}()

	var mf bytes.Buffer
	opts := &cmdManifestWrapperOptions{
		useBootstrapIfNeeded: true,
	}

	// We discard any warnings from the depsolver until we figure out a better
	// idea (likely in manifestgen)
	res, err := cmdManifestWrapper(pbar, cmd, args, &mf, io.Discard, opts)
	if err != nil {
		return err
	}

	bootMode := res.ImgType.BootMode()
	uploader, err := uploaderFor(cmd, res.ImgType.Name(), res.ImgType.Arch().Name(), &bootMode)
	if errors.Is(err, ErrUploadTypeUnsupported) || errors.Is(err, ErrUploadConfigNotProvided) {
		err = nil
	}

	if err != nil {
		return err
	}

	if uploader != nil {
		pbar.SetPulseMsgf("Checking cloud access")
		if err := uploaderCheckWithProgress(pbar, uploader); err != nil {
			return err
		}
	}
	outputDir = basenameFor(res, outputDir)

	buildOpts := &buildOptions{
		OutputDir:      outputDir,
		OutputBasename: outputBasename,
		StoreDir:       cacheDir,
		WriteManifest:  withManifest,
		WriteBuildlog:  withBuildlog,
	}
	pbar.SetPulseMsgf("Image building step")
	imagePath, err := buildImage(pbar, res, mf.Bytes(), buildOpts)
	if err != nil {
		return err
	}
	pbar.Stop()

	fmt.Fprintf(osStdout, "Image build successful, results:\n%s\n", listOutputdir(outputDir))

	if uploader != nil {
		// XXX: integrate better into the progress, see bib
		if err := uploadImageWithProgress(uploader, imagePath); err != nil {
			return err
		}
	}

	return nil
}

func cmdDescribeImg(cmd *cobra.Command, args []string) error {
	// XXX: boilderplate identical to cmdManifest() above
	dataDir, err := cmd.Flags().GetString("data-dir")
	if err != nil {
		return err
	}
	distroStr, err := cmd.Flags().GetString("distro")
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

	distroStr, err = findDistro(distroStr, "")
	if err != nil {
		return err
	}

	imgTypeStr := args[0]
	res, err := getOneImage(distroStr, imgTypeStr, archStr, &repoOptions{DataDir: dataDir})
	if err != nil {
		return err
	}

	return describeImage(res, osStdout)
}

func run() error {
	// Initialize console logger (stderr, no prefix)
	log.SetFlags(0)

	rootCmd := &cobra.Command{
		Use:   "image-builder",
		Short: "Build operating system images from a given distro/image-type/blueprint",
		Long: `Build operating system images from a given distribution,
image-type and blueprint.

Image-builder builds operating system images for a range of predefined
operating systems like Fedora, CentOS and RHEL with easy customizations support.`,
		SilenceErrors: true,
		Version:       prettyVersion(),
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
	}
	rootCmd.SetVersionTemplate(prettyVersion())

	rootCmd.PersistentFlags().String("data-dir", "", `Override the default data directory for e.g. custom repositories/*.json data`)
	rootCmd.PersistentFlags().StringArray("extra-repo", nil, `Add an extra repository during build (will *not* be gpg checked and not be part of the final image)`)
	rootCmd.PersistentFlags().StringArray("force-repo", nil, `Override the base repositories during build (these will not be part of the final image)`)
	rootCmd.PersistentFlags().String("output-dir", "", `Put output into the specified directory`)
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, `Switch to verbose mode (more logging on stderr and verbose progress)`)
	rootCmd.SetOut(osStdout)
	rootCmd.SetErr(osStderr)

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
	rootCmd.AddCommand(listCmd)

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
	manifestCmd.Flags().Bool("use-librepo", true, `use librepo to download packages (disable if you use old versions of osbuild)`)
	manifestCmd.Flags().Bool("with-sbom", false, `export SPDX SBOM document`)
	manifestCmd.Flags().Bool("ignore-warnings", false, `ignore warnings during manifest generation`)
	manifestCmd.Flags().String("registrations", "", `filename of a registrations file with e.g. subscription details`)
	rootCmd.AddCommand(manifestCmd)

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
	uploadCmd.Flags().String("arch", "", "upload for the given architecture")
	rootCmd.AddCommand(uploadCmd)

	buildCmd := &cobra.Command{
		Use:          "build <image-type>",
		Short:        "Build the given image-type, e.g. qcow2 (tip: combine with --distro, --arch)",
		RunE:         cmdBuild,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}
	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	buildCmd.Flags().Bool("with-manifest", false, `export osbuild manifest`)
	buildCmd.Flags().Bool("with-buildlog", false, `export osbuild buildlog`)
	// XXX: add --rpmmd cache too and put under /var/cache/image-builder/dnf
	buildCmd.Flags().String("cache", "/var/cache/image-builder/store", `osbuild directory to cache intermediate build artifacts"`)
	// XXX: add "--verbose" here, similar to how bib is doing this
	// (see https://github.com/osbuild/bootc-image-builder/pull/790/commits/5cec7ffd8a526e2ca1e8ada0ea18f927695dfe43)
	buildCmd.Flags().String("progress", "auto", "type of progress bar to use (e.g. verbose,term)")
	buildCmd.Flags().String("output-name", "", "set specific output basename")
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().AddFlagSet(uploadCmd.Flags())
	// add after the rest of the uploadCmd flag set is added to avoid
	// that build gets a "--to" parameter
	uploadCmd.Flags().String("to", "", "upload to the given cloud")

	// XXX: add --format=json too?
	describeImgCmd := &cobra.Command{
		Use:          "describe <image-type>",
		Short:        "Describe the given image-type, e.g. qcow2 (tip: combine with --distro,--arch)",
		RunE:         cmdDescribeImg,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Hidden:       false,
		Aliases:      []string{"describe-image"},
	}
	describeImgCmd.Flags().String("arch", "", `use the different architecture`)
	describeImgCmd.Flags().String("distro", "", `build manifest for a different distroname (e.g. centos-9)`)

	rootCmd.AddCommand(describeImgCmd)

	verbose, err := rootCmd.PersistentFlags().GetBool("verbose")
	if err != nil {
		return err
	}
	if verbose {
		olog.SetDefault(log.New(os.Stderr, "", 0))
		// XXX: add once images has olog support
		//images_log.SetDefault(log.New(os.Stderr, "", 0))
	}

	return rootCmd.Execute()
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %s\n", err)
	}
}
