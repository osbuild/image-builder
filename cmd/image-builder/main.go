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

	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/bootc"
	"github.com/osbuild/image-builder/pkg/cloud"
	"github.com/osbuild/image-builder/pkg/customizations/subscription"
	"github.com/osbuild/image-builder/pkg/distro/generic"
	"github.com/osbuild/image-builder/pkg/imagefilter"
	"github.com/osbuild/image-builder/pkg/manifestgen"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/ostree"
	"github.com/osbuild/image-builder/pkg/progress"

	"github.com/osbuild/image-builder/internal/blueprintload"
	"github.com/osbuild/image-builder/pkg/setup"
)

var (
	osStdout         io.Writer = os.Stdout
	osStderr         io.Writer = os.Stderr
	bootcResolveInfo           = bootc.ResolveBootcInfo
)

// cacheDirForUid returns the cache directory for the given uid.
// When root (uid 0) it uses the system-wide /var/cache path.
// When non-root it follows the XDG Base Directory specification
// and falls back to ~/.cache.
func cacheDirForUid(uid int) string {
	if uid == 0 {
		return "/var/cache/image-builder/store"
	}
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "image-builder", "store")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/var/cache/image-builder/store"
	}
	return filepath.Join(home, ".cache", "image-builder", "store")
}

// defaultCacheDir returns the cache directory for the current user.
func defaultCacheDir() string {
	return cacheDirForUid(os.Getuid())
}

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
	arch := img.ImgType.Arch()
	distro := arch.Distro()
	return fmt.Sprintf("%s-%s-%s", distro.Name(), img.ImgType.Name(), arch.Name())
}

func cmdSystem(cmd *cobra.Command, args []string) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	switch format {
	case "", "yaml":
		fmt.Fprint(cmd.OutOrStdout(), prettySystemStatus())
	case "json":
		fmt.Fprint(cmd.OutOrStdout(), jsonSystemStatus())
	default:
		return fmt.Errorf("unsupported format %q, supported formats: yaml, json", format)
	}
	return nil
}

func cmdVersion(cmd *cobra.Command, args []string) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	switch format {
	case "", "yaml":
		fmt.Fprint(cmd.OutOrStdout(), prettyVersion())
	case "json":
		fmt.Fprint(cmd.OutOrStdout(), jsonVersion())
	default:
		return fmt.Errorf("unsupported format %q, supported formats: yaml, json", format)
	}
	return nil
}

func cmdBootcInspect(cmd *cobra.Command, args []string) error {
	ref, err := cmd.Flags().GetString("ref")
	if err != nil {
		return err
	}

	info, err := bootcResolveInfo(ref)
	if err != nil {
		return err
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	switch format {
	case "", "yaml":
		output, err := yaml.Marshal(&info)
		if err != nil {
			return err
		}

		fmt.Fprint(cmd.OutOrStdout(), string(output))
	case "json":
		output, err := json.Marshal(&info)
		if err != nil {
			return err
		}

		fmt.Fprint(cmd.OutOrStdout(), string(output))
	default:
		return fmt.Errorf("unsupported format %q, supported formats: yaml, json", format)
	}

	return nil
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
	repoDir, err := cmd.Flags().GetString("force-repo-dir")
	if err != nil {
		return err
	}
	extraRepos, err := cmd.Flags().GetStringArray("extra-repo")
	if err != nil {
		return err
	}
	forceDefsDir, err := cmd.Flags().GetString("force-defs-dir")
	if err != nil {
		return err
	}

	return listImages(repoDir, extraRepos, forceDefsDir, format, filter)
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

// used in tests
var manifestgenDepsolver manifestgen.DepsolveFunc
var manifestgenContainerResolver manifestgen.ContainerResolverFunc

func getImage(cmd *cobra.Command, args []string) (*imagefilter.Result, error) {
	repoDir, err := cmd.Flags().GetString("force-repo-dir")
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
	bootcDefaultFs, err := cmd.Flags().GetString("bootc-default-fs")
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
	bootcBuildRef, err := cmd.Flags().GetString("bootc-build-ref")
	if err != nil {
		return nil, err
	}
	forceDefsDir, err := cmd.Flags().GetString("force-defs-dir")
	if err != nil {
		return nil, err
	}
	imgTypeStr := args[0]

	var img *imagefilter.Result
	if bootcRef != "" {
		// The behavior of anaconda-iso without special mTLS setup is different
		// from bib so instead of introducing subtle incompatibilities just error
		// here
		if imgTypeStr == "anaconda-iso" {
			return nil, fmt.Errorf(`image type bootc "anaconda-iso" is not supported with image-builder, please consider switching to "bootc-installer" or use bootc-image-builder`)
		}
		bootcInfo, err := bootc.ResolveBootcInfo(bootcRef)
		if err != nil {
			return nil, err
		}

		// Log informational message about root filesystem configuration source.
		// disk.yaml always takes priority over --bootc-default-fs and bootc config.
		diskYamlRootFs := bootcInfo.OSInfo.GetDiskYamlRootFs()
		if diskYamlRootFs != "" {
			// XXX: hack, temporary progress bar, the file progress bar cannot be
			// constructed before we have the img. Use the verbose bar, as the terminal
			// bar would be overwritten by the next terminal message before anyone could
			// see it.
			pbar, err := progress.New("verbose", progress.ProgressConfig{})
			if err != nil {
				return nil, err
			}
			if bootcDefaultFs != "" {
				pbar.SetPulseMsgf("Using disk.yaml root filesystem (%s), ignoring --bootc-default-fs (%s)", diskYamlRootFs, bootcDefaultFs)
			} else if bootcInfo.DefaultRootFs != "" {
				pbar.SetPulseMsgf("Using disk.yaml root filesystem (%s), ignoring bootc config (%s)", diskYamlRootFs, bootcInfo.DefaultRootFs)
			} else {
				pbar.SetPulseMsgf("Using disk.yaml root filesystem (%s)", diskYamlRootFs)
			}
		} else if bootcDefaultFs != "" {
			bootcInfo.DefaultRootFs = bootcDefaultFs
		}

		distro, err := generic.NewBootc("bootc", bootcInfo)
		if err != nil {
			return nil, err
		}
		if bootcBuildRef != "" {
			buildBootcInfo, err := bootc.ResolveBootcBuildInfo(bootcBuildRef)
			if err != nil {
				return nil, err
			}
			if err := distro.SetBuildContainer(buildBootcInfo); err != nil {
				return nil, err
			}
		}
		archi, err := distro.GetArch(archStr)
		if err != nil {
			return nil, err
		}
		imgType, err := archi.GetImageType(imgTypeStr)
		if err != nil {
			return nil, err
		}
		img = &imagefilter.Result{ImgType: imgType, Repos: nil}
	} else {
		var bpDistroName string
		blueprintPath, err := cmd.Flags().GetString("blueprint")
		if err != nil {
			return nil, err
		}
		if blueprintPath != "" {
			bp, err := blueprintload.Load(blueprintPath)
			if err != nil {
				return nil, err
			}
			bpDistroName = bp.Distro
		}
		distroStr, err = findDistro(distroStr, bpDistroName)
		if err != nil {
			return nil, err
		}
		repoOpts := &repoOptions{
			RepoDir:      repoDir,
			ExtraRepos:   extraRepos,
			ForceRepos:   forceRepos,
			ForceDefsDir: forceDefsDir,
		}
		img, err = getOneImage(distroStr, imgTypeStr, archStr, repoOpts)
		if err != nil {
			return nil, err
		}
	}
	if len(img.ImgType.Exports()) > 1 {
		return nil, fmt.Errorf("image %q has multiple exports: this is current unsupport: please report this as a bug", basenameFor(img, ""))
	}
	return img, err
}

func cmdManifestWrapper(pbar progress.ProgressBar, cmd *cobra.Command, args []string, img *imagefilter.Result, w io.Writer, wd io.Writer, wrapperOpts *cmdManifestWrapperOptions) error {
	if wrapperOpts == nil {
		wrapperOpts = &cmdManifestWrapperOptions{}
	}
	repoDir, err := cmd.Flags().GetString("force-repo-dir")
	if err != nil {
		return err
	}
	rpmmdCacheDir, err := cmd.Flags().GetString("rpmmd-cache")
	if err != nil {
		return err
	}
	extraRepos, err := cmd.Flags().GetStringArray("extra-repo")
	if err != nil {
		return err
	}
	forceRepos, err := cmd.Flags().GetStringArray("force-repo")
	if err != nil {
		return err
	}
	distroStr, err := cmd.Flags().GetString("distro")
	if err != nil {
		return err
	}
	withSBOM, err := cmd.Flags().GetBool("with-sbom")
	if err != nil {
		return err
	}
	withRPMList, err := cmd.Flags().GetBool("with-rpmlist")
	if err != nil {
		return err
	}
	ignoreWarnings, err := cmd.Flags().GetBool("ignore-warnings")
	if err != nil {
		return err
	}
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return err
	}
	ostreeImgOpts, err := ostreeImageOptions(cmd)
	if err != nil {
		return err
	}
	useLibrepo, err := cmd.Flags().GetBool("use-librepo")
	if err != nil {
		return err
	}
	bootcRemote, err := cmd.Flags().GetBool("bootc-pull-container")
	if err != nil {
		return err
	}
	imageSize, err := cmd.Flags().GetUint64("image-size")
	if err != nil {
		return err
	}

	var preview *bool
	// Verify that the flag was actually passed. If it wasn't passed
	// we keep our nil value so that images used the distro-defined
	// value for preview. Otherwise use the provided value so the
	// distro value gets overridden.
	if cmd.Flags().Lookup("preview").Changed {
		value, err := cmd.Flags().GetBool("preview")
		if err != nil {
			return err
		}
		preview = &value
	}
	var rpmDownloader osbuild.RpmDownloader
	if useLibrepo {
		rpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	blueprintPath, err := cmd.Flags().GetString("blueprint")
	if err != nil {
		return err
	}
	var customSeed *int64
	if cmd.Flags().Changed("seed") {
		seedFlagVal, err := cmd.Flags().GetInt64("seed")
		if err != nil {
			return err
		}
		customSeed = &seedFlagVal
	}
	subscription, err := subscriptionImageOptions(cmd)
	if err != nil {
		return err
	}
	bootcRef, err := cmd.Flags().GetString("bootc-ref")
	if err != nil {
		return err
	}
	if bootcRef != "" && distroStr != "" {
		return fmt.Errorf("cannot use --distro with --bootc-ref")
	}
	bootcInstallerPayloadRef, err := cmd.Flags().GetString("bootc-installer-payload-ref")
	if err != nil {
		return err
	}
	bootcOmitDefaultKernelArgs, err := cmd.Flags().GetBool("bootc-no-default-kernel-args")
	if err != nil {
		return err
	}

	// no error check here as this is (deliberately) not defined on
	// "manifest" (if "images" learn to set the output filename in
	// manifests we would change this
	outputFilename, _ := cmd.Flags().GetString("output-name")

	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return err
	}
	if bootcRef == "" {
		distroStr, err = findDistro(distroStr, bp.Distro)
		if err != nil {
			return err
		}
	} else {
		distroStr = "bootc-based"
		// XXX: hack to skip repo loading for the bootc image.
		// We need to add a SkipRepositories or similar to
		// manifestgen instead to make this clean
		forceRepos = []string{"https://example.com/not-used"}
	}

	imgTypeStr := args[0]
	pbar.SetPulseMsgf("Manifest generation step")
	pbar.SetMessagef("Building manifest for %s-%s", distroStr, imgTypeStr)

	opts := &manifestOptions{
		ManifestgenOptions: manifestgen.Options{
			Cachedir:               rpmmdCacheDir,
			CustomSeed:             customSeed,
			RpmDownloader:          rpmDownloader,
			DepsolveWarningsOutput: wd,
			Depsolve:               manifestgenDepsolver,
			ContainerResolver:      manifestgenContainerResolver,
		},
		OutputDir:                  outputDir,
		OutputFilename:             outputFilename,
		BlueprintPath:              blueprintPath,
		Ostree:                     ostreeImgOpts,
		BootcRef:                   bootcRef,
		BootcInstallerPayloadRef:   bootcInstallerPayloadRef,
		BootcOmitDefaultKernelArgs: bootcOmitDefaultKernelArgs,
		BootcRemote:                bootcRemote,
		ImageSize:                  imageSize,
		WithSBOM:                   withSBOM,
		WithRPMList:                withRPMList,
		IgnoreWarnings:             ignoreWarnings,
		Subscription:               subscription,
		Preview:                    preview,

		ForceRepos: forceRepos,
	}
	opts.ManifestgenOptions.UseBootstrapContainer = wrapperOpts.useBootstrapIfNeeded && (img.ImgType.Arch().Name() != arch.Current().String())
	if opts.ManifestgenOptions.UseBootstrapContainer {
		fmt.Fprintf(os.Stderr, "WARNING: using experimental cross-architecture building to build %q\n", img.ImgType.Arch().Name())
	}
	return generateManifest(repoDir, extraRepos, img, w, opts)
}

func cmdManifest(cmd *cobra.Command, args []string) error {
	pbar, err := progress.New("", progress.ProgressConfig{})
	if err != nil {
		return err
	}
	img, err := getImage(cmd, args)
	if err != nil {
		return err
	}
	return cmdManifestWrapper(pbar, cmd, args, img, osStdout, io.Discard, nil)
}

func progressFromCmd(cmd *cobra.Command, conf progress.ProgressConfig) (progress.ProgressBar, error) {
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

	return progress.New(progressType, conf)
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
	withUploadResult, err := cmd.Flags().GetBool("with-upload-result")
	if err != nil {
		return err
	}
	withMetrics, err := cmd.Flags().GetBool("with-metrics")
	if err != nil {
		return err
	}
	runInVm, err := cmd.Flags().GetBool("in-vm")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	// Fail early if the cache directory is not writable, instead of
	// waiting for osbuild to fail after slow manifest generation.
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("cannot create cache directory %q: %w\nHint: use --cache to specify a writable path", cacheDir, err)
	}

	// Setup osbuild environment if running in a container
	if setup.IsContainer() {
		if err := setup.EnsureEnvironment(cacheDir, runInVm); err != nil {
			return fmt.Errorf("entrypoint setup failed: %w", err)
		}
	}

	if runInVm && !setup.IsContainer() {
		return fmt.Errorf("running in VM outside container is not supported yet")
	}

	img, err := getImage(cmd, args)
	if err != nil {
		return err
	}
	// Ensure the output directory exists before (file) progress starts.
	outputDir = basenameFor(img, outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("cannot create output base directory %s: %w", outputDir, err)
	}

	pbar, err := progressFromCmd(cmd, progress.ProgressConfig{
		FilePath: filepath.Join(outputDir, fmt.Sprintf("%s.progress", basenameFor(img, outputBasename))),
	})
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
	err = cmdManifestWrapper(pbar, cmd, args, img, &mf, io.Discard, opts)
	if err != nil {
		return err
	}

	bootMode := img.ImgType.BootMode()
	uploader, err := uploaderFor(cmd, img.ImgType.Name(), img.ImgType.Arch().Name(), &bootMode, "")
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

	buildOpts := &buildOptions{
		OutputDir:      outputDir,
		OutputBasename: outputBasename,
		StoreDir:       cacheDir,
		WriteManifest:  withManifest,
		WriteBuildlog:  withBuildlog,
		Metrics:        withMetrics,
		JSONOutput:     format == "json",
	}
	if runInVm {
		buildOpts.InVm = []string{"image"}
	}
	pbar.SetPulseMsgf("Image building step")
	imagePath, err := buildImage(pbar, img, mf.Bytes(), buildOpts)
	if err != nil {
		return err
	}
	pbar.Stop()

	fmt.Fprintf(osStdout, "Image build successful: %s\n", imagePath)

	// Default upload result to write out in case no uploader was specified
	uploadResult := &cloud.UploadResult{
		Provider: "LocalPath",
		ImageID:  imagePath,
	}
	if uploader != nil {
		// XXX: integrate better into the progress, see bib
		uploadResult, err = uploadImageWithProgress(uploader, imagePath)
		if err != nil {
			return err
		}
	}
	if withUploadResult {
		p := filepath.Join(outputDir, fmt.Sprintf("%s.upload-result", basenameFor(img, outputBasename)))
		data, err := json.Marshal(uploadResult)
		if err != nil {
			return err
		}
		// #nosec: G306
		if err := os.WriteFile(p, data, 0640); err != nil {
			return err
		}
	}

	return nil
}

func cmdDescribeImg(cmd *cobra.Command, args []string) error {
	// XXX: boilderplate identical to cmdManifest() above
	repoDir, err := cmd.Flags().GetString("force-repo-dir")
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
	forceDefsDir, err := cmd.Flags().GetString("force-defs-dir")
	if err != nil {
		return err
	}

	distroStr, err = findDistro(distroStr, "")
	if err != nil {
		return err
	}

	imgTypeStr := args[0]
	res, err := getOneImage(distroStr, imgTypeStr, archStr, &repoOptions{RepoDir: repoDir, ForceDefsDir: forceDefsDir})
	if err != nil {
		return err
	}

	return describeImage(res, osStdout)
}

func run() error {
	// Initialize console logger (stderr, no prefix)
	log.SetFlags(0)
	memProfileResetForProcessStart()

	rootCmd, err := setupRootCmd()
	if err != nil {
		return err
	}

	execErr := rootCmd.Execute()
	if flushErr := memProfileFlush(); flushErr != nil {
		return errors.Join(execErr, flushErr)
	}
	return execErr
}

func main() {
	var err error
	// we are a multi-call binary and behave differently when
	// called as "bootc-image-builder"
	if strings.HasSuffix(os.Args[0], "bootc-image-builder") {
		err = bibRun()
	} else {
		err = run()
	}
	if err != nil {
		log.Fatalf("error: %s\n", err)
	}
}
