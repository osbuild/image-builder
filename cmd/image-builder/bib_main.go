package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	repos "github.com/osbuild/image-builder/data/repositories"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/bib/blueprintload"
	"github.com/osbuild/image-builder/pkg/bootc"
	"github.com/osbuild/image-builder/pkg/cloud"
	"github.com/osbuild/image-builder/pkg/cloud/awscloud"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/distro/generic"
	"github.com/osbuild/image-builder/pkg/experimentalflags"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/manifestgen"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/reporegistry"
	"github.com/osbuild/image-builder/pkg/rpmmd"

	"github.com/osbuild/image-builder/internal/bibimg"
	"github.com/osbuild/image-builder/pkg/progress"
	"github.com/osbuild/image-builder/pkg/setup"
)

var (
	osGetuid = os.Getuid
	osGetgid = os.Getgid
)

func saveManifest(ms manifest.OSBuildManifest, fpath string) error {
	b, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %q: %s", fpath, err.Error())
	}
	b = append(b, '\n')                 // add new line at end of file
	return os.WriteFile(fpath, b, 0644) // #nosec: G306
}

// bibManifestFromCobra generate an osbuild manifest from a cobra commandline.
//
// It takes an unstarted progres bar and will start it at the right
// point (it cannot be started yet to avoid the "podman pull" progress
// and our progress fighting). The caller is responsible for stopping
// the progress bar (this function cannot know what else needs to happen
// after manifest generation).
//
// This code is very similar to main.go:cmdManifestWrapper (which is
// sad), but consolidate is hard because:
//  1. We need to support anaconda-iso here which means we need to provide a custom
//     depsolve function to extract the mTLS config from the container image, this is
//     something we consider legacy so ibcli:main.go does not have it
//  2. The cobra options handling is different (but that could be consolidated)
//  3. Blueprint validation is a warning by default for bib but an error for ibcli (could also be
//     consolidated)
func bibManifestFromCobra(cmd *cobra.Command, args []string, pbar progress.ProgressBar) ([]byte, *mTLSConfig, error) {
	cntArch := arch.Current()

	imgref := args[0]
	userConfigFile, _ := cmd.Flags().GetString("config")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	rootFs, _ := cmd.Flags().GetString("rootfs")
	buildImgref, _ := cmd.Flags().GetString("build-container")
	installerPayloadRef, _ := cmd.Flags().GetString("installer-payload-ref")
	useLibrepo, _ := cmd.Flags().GetBool("use-librepo")

	// If --local was given, warn in the case of --local or --local=true (true is the default), error in the case of --local=false
	if cmd.Flags().Changed("local") {
		localStorage, _ := cmd.Flags().GetBool("local")
		if localStorage {
			fmt.Fprintf(os.Stderr, "WARNING: --local is now the default behavior, you can remove it from the command line\n")
		} else {
			return nil, nil, fmt.Errorf(`--local=false is no longer supported, remove it and make sure to pull the container before running bib:
	sudo podman pull %s`, imgref)
		}
	}

	if targetArch != "" {
		target, err := arch.FromString(targetArch)
		if err != nil {
			return nil, nil, err
		}
		if target != arch.Current() {
			// TODO: detect if binfmt_misc for target arch is
			// available, e.g. by mounting the binfmt_misc fs into
			// the container and inspects the files or by
			// including tiny statically linked target-arch
			// binaries inside our bib container
			fmt.Fprintf(os.Stderr, "WARNING: target-arch is experimental and needs an installed 'qemu-user' package\n")
			if slices.Contains(imgTypes, "iso") {
				return nil, nil, fmt.Errorf("cannot build iso for different target arches yet")
			}
			cntArch = target
		}
	}
	// TODO: add "target-variant", see https://github.com/osbuild/bootc-image-builder/pull/139/files#r1467591868

	if err := setup.ValidateHasContainerStorageMounted(); err != nil {
		return nil, nil, fmt.Errorf("could not access container storage, did you forget -v /var/lib/containers/storage:/var/lib/containers/storage? (%w)", err)
	}

	imageTypes, err := bibimg.New(imgTypes...)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot detect build types %v: %w", imgTypes, err)
	}
	config, err := blueprintload.LoadWithFallback(userConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read config: %w", err)
	}

	if err := setup.ValidateHasContainerTags(imgref); err != nil {
		return nil, nil, err
	}

	pbar.SetPulseMsgf("Manifest generation step")
	pbar.Start()

	// Note that we only need to pass a single imgType here into the manifest generation because:
	// 1. the bootc disk manifests contains exports for all supported image types
	// 2. the bootc legacy types (iso, anaconda-iso) always do a single build
	imgTypeStr := imageTypes[0]

	// The anaconda-iso code is different enough for a separate function
	if imgTypeStr == "anaconda-iso" || imgTypeStr == "iso" {
		return manifestFromCobraForLegacyISO(imgref, buildImgref, imgTypeStr, rootFs, rpmCacheRoot, config, useLibrepo, cntArch)
	}

	bootcInfo, err := bootc.ResolveBootcInfo(imgref)
	if err != nil {
		return nil, nil, err
	}
	if rootFs != "" {
		bootcInfo.DefaultRootFs = rootFs
	}
	distri, err := generic.NewBootc("bootc", bootcInfo)
	if err != nil {
		return nil, nil, err
	}
	if buildImgref != "" {
		buildBootcInfo, err := bootc.ResolveBootcInfo(buildImgref)
		if err != nil {
			return nil, nil, err
		}
		if err := distri.SetBuildContainer(buildBootcInfo); err != nil {
			return nil, nil, err
		}
	}
	archi, err := distri.GetArch(cntArch.String())
	if err != nil {
		return nil, nil, err
	}
	imgType, err := archi.GetImageType(imgTypeStr)
	if err != nil {
		return nil, nil, err
	}

	repos, err := reporegistry.New(nil, []fs.FS{repos.FS})
	if err != nil {
		return nil, nil, err
	}
	var depsolveResult map[string]depsolvednf.DepsolveResult
	var rpmDownloader osbuild.RpmDownloader
	if useLibrepo {
		rpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	var mTLS *mTLSConfig
	mg, err := manifestgen.New(repos, &manifestgen.Options{
		Cachedir: rpmCacheRoot,
		// XXX: hack to skip repo loading for the bootc image.
		// We need to add a SkipRepositories or similar to
		// manifestgen instead to make this clean
		OverrideRepos: []rpmmd.RepoConfig{
			{
				BaseURLs: []string{"https://example.com/not-used"},
			},
		},
		RpmDownloader: rpmDownloader,
		Depsolve: func(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
			depsolveResult, err = manifestgen.DefaultDepsolve(solver, cacheDir, depsolveWarningsOutput, packageSets, d, arch)
			// extracting needs to happen while container is mounted
			depsolvedRepos := make(map[string][]rpmmd.RepoConfig)
			for k, v := range depsolveResult {
				depsolvedRepos[k] = v.Repos
			}
			mTLS, err = extractTLSKeys(depsolvedRepos)
			if err != nil {
				return nil, err
			}
			return depsolveResult, err
		},
		// this turns (blueprint validation) warnings into
		// warnings as they are visible to the user
		WarningsOutput: os.Stderr,
	})
	if err != nil {
		return nil, nil, err
	}
	imgOpts := &distro.ImageOptions{
		Bootc: &distro.BootcImageOptions{
			InstallerPayloadRef: installerPayloadRef,
		},
	}
	manifest, err := mg.Generate(config, imgType, imgOpts)
	if err != nil {
		return nil, nil, err
	}

	return manifest, mTLS, nil
}

func bibCmdManifest(cmd *cobra.Command, args []string) error {
	pbar, err := progress.New("", progress.ProgressConfig{})
	if err != nil {
		// this should never happen
		return fmt.Errorf("cannot create progress bar: %w", err)
	}
	defer pbar.Stop()

	mf, _, err := bibManifestFromCobra(cmd, args, pbar)
	if err != nil {
		return fmt.Errorf("cannot generate manifest: %w", err)
	}
	fmt.Println(string(mf))
	return nil
}

func handleAWSFlags(cmd *cobra.Command) (cloud.Uploader, error) {
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	region, _ := cmd.Flags().GetString("aws-region")
	if region == "" {
		return nil, nil
	}
	bucketName, _ := cmd.Flags().GetString("aws-bucket")
	imageName, _ := cmd.Flags().GetString("aws-ami-name")
	targetArchStr, _ := cmd.Flags().GetString("target-arch")

	if !slices.Contains(imgTypes, "ami") {
		return nil, fmt.Errorf("aws flags set for non-ami image type (type is set to %s)", strings.Join(imgTypes, ","))
	}

	targetArch := arch.Current()
	if targetArchStr != "" {
		var err error
		targetArch, err = arch.FromString(targetArchStr)
		if err != nil {
			return nil, err
		}
	}
	uploaderOpts := &awscloud.UploaderOptions{
		TargetArch: targetArch,
	}
	uploader, err := awscloudNewUploader(region, bucketName, imageName, uploaderOpts)
	if err != nil {
		return nil, err
	}
	status := io.Discard
	if logrus.GetLevel() >= logrus.InfoLevel {
		status = os.Stderr
	}
	// check as many permission prerequisites as possible before starting
	if err := uploader.Check(status); err != nil {
		return nil, err
	}
	return uploader, nil
}

// This is very similar to main.go:cmdBuild (which is sad), the differences that makes
// merging them very hard are:
//  1. We need to support anaconda-iso here which means we need to support writing a custom
//     mTLS configuration (and cleaning up afterwards)
//  2. The cobra options are different
//  3. Multiple image types can be build in a single go (--type qcow2 --type raw) which is
//     not supported by ibcli
//  4. The produced artifacts are not renamed, they are exactly as they come out of the
//     imgType.Export(), e.g. "bootiso.iso". ibcli will rename to $distro-$arch-$imgtype
//     intead (but we cannot change the output of bib becaue e.g. podman desktop depends
//     on it)
func bibCmdBuild(cmd *cobra.Command, args []string) error {
	chown, _ := cmd.Flags().GetString("chown")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	osbuildStore, _ := cmd.Flags().GetString("store")
	outputDir, _ := cmd.Flags().GetString("output")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	progressType, _ := cmd.Flags().GetString("progress")

	logrus.Debug("Validating environment")
	if err := setup.Validate(targetArch, false); err != nil {
		return fmt.Errorf("cannot validate the setup: %w", err)
	}
	logrus.Debug("Ensuring environment setup")
	switch setup.IsContainer() {
	case false:
		fmt.Fprintf(os.Stderr, "WARNING: running outside a container, this is an unsupported configuration\n")
	case true:
		if err := setup.EnsureEnvironment(osbuildStore, false); err != nil {
			return fmt.Errorf("cannot ensure the environment: %w", err)
		}
	}

	if err := os.MkdirAll(outputDir, 0o777); err != nil {
		return fmt.Errorf("cannot setup build dir: %w", err)
	}

	uploader, err := handleAWSFlags(cmd)
	if err != nil {
		return fmt.Errorf("cannot handle AWS setup: %w", err)
	}

	canChown, err := canChownInPath(outputDir)
	if err != nil {
		return fmt.Errorf("cannot ensure ownership: %w", err)
	}
	if !canChown && chown != "" {
		return fmt.Errorf("chowning is not allowed in output directory")
	}

	pbar, err := progress.New(progressType, progress.ProgressConfig{})
	if err != nil {
		return fmt.Errorf("cannto create progress bar: %w", err)
	}
	defer pbar.Stop()

	manifest_fname := fmt.Sprintf("manifest-%s.json", strings.Join(imgTypes, "-"))
	pbar.SetMessagef("Generating manifest %s", manifest_fname)
	mf, mTLS, err := bibManifestFromCobra(cmd, args, pbar)
	if err != nil {
		return fmt.Errorf("cannot build manifest: %w", err)
	}
	pbar.SetMessagef("Done generating manifest")

	// collect pipeline exports for each image type
	imageTypes, err := bibimg.New(imgTypes...)
	if err != nil {
		return err
	}
	exports := imageTypes.Exports()
	manifestPath := filepath.Join(outputDir, manifest_fname)
	if err := saveManifest(mf, manifestPath); err != nil {
		return fmt.Errorf("cannot save manifest: %w", err)
	}

	pbar.SetPulseMsgf("Disk image building step")
	pbar.SetMessagef("Building %s", manifest_fname)

	var osbuildEnv []string
	if !canChown {
		// set export options for osbuild
		osbuildEnv = []string{"OSBUILD_EXPORT_FORCE_NO_PRESERVE_OWNER=1"}
	}

	if mTLS != nil {
		envVars, cleanup, err := prepareOsbuildMTLSConfig(mTLS)
		if err != nil {
			return fmt.Errorf("failed to prepare osbuild TLS keys: %w", err)
		}

		defer cleanup()

		osbuildEnv = append(osbuildEnv, envVars...)
	}

	if experimentalflags.Bool("debug-qemu-user") {
		osbuildEnv = append(osbuildEnv, "OBSBUILD_EXPERIMENAL=debug-qemu-user")
	}
	osbuildOpts := progress.OSBuildOptions{
		StoreDir:  osbuildStore,
		OutputDir: outputDir,
		ExtraEnv:  osbuildEnv,
	}
	if err = progress.RunOSBuild(pbar, mf, exports, &osbuildOpts); err != nil {
		return fmt.Errorf("cannot run osbuild: %w", err)
	}

	pbar.SetMessagef("Build complete!")
	if uploader != nil {
		// XXX: pass our own progress.ProgressBar here
		// *for now* just stop our own progress and let the uploadAMI
		// progress take over - but we really need to fix this in a
		// followup
		pbar.Stop()
		for idx, imgType := range imgTypes {
			switch imgType {
			case "ami":
				diskpath := filepath.Join(outputDir, exports[idx], "disk.raw")
				if err := bibUpload(uploader, diskpath, cmd.Flags()); err != nil {
					return fmt.Errorf("cannot upload AMI: %w", err)
				}
			default:
				continue
			}
		}
	} else {
		pbar.SetMessagef("Results saved in %s", outputDir)
	}

	if err := chownR(outputDir, chown); err != nil {
		return fmt.Errorf("cannot setup owner for %q: %w", outputDir, err)
	}

	return nil
}

var rootLogLevel string

func bibRootPreRunE(cmd *cobra.Command, _ []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	progress, _ := cmd.Flags().GetString("progress")
	switch {
	case rootLogLevel != "":
		level, err := logrus.ParseLevel(rootLogLevel)
		if err != nil {
			return err
		}
		logrus.SetLevel(level)
	case verbose:
		logrus.SetLevel(logrus.InfoLevel)
	default:
		logrus.SetLevel(logrus.ErrorLevel)
	}
	if verbose && progress == "auto" {
		if err := cmd.Flags().Set("progress", "verbose"); err != nil {
			return err
		}
	}

	return nil
}

// XXX: use prettyVersion()
func bibVersionFromBuildInfo() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("cannot read build info")
	}
	var buildTainted bool
	gitRev := "unknown"
	buildTime := "unknown"
	for _, bs := range info.Settings {
		switch bs.Key {
		case "vcs.revision":
			gitRev = bs.Value[:7]
		case "vcs.time":
			buildTime = bs.Value
		case "vcs.modified":
			bT, err := strconv.ParseBool(bs.Value)
			if err != nil {
				logrus.Errorf("Error parsing 'vcs.modified': %v", err)
				bT = true
			}
			buildTainted = bT
		}
	}

	return fmt.Sprintf(`build_revision: %s
build_time: %s
build_tainted: %v
`, gitRev, buildTime, buildTainted), nil
}

func bibRun() error {
	rootCmd, err := setupBibRootCmd()
	if err != nil {
		return err
	}

	return rootCmd.Execute()
}
