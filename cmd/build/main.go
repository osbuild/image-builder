// Standalone executable for building a test image.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/buildconfig"
	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bootc"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/generic"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifestgen"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rhsm/facts"
	"github.com/osbuild/images/pkg/rpmmd"
)

func u(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func run() error {
	// common args
	var outputDir, osbuildStore, rpmCacheRoot, repositories, archName string
	flag.StringVar(&outputDir, "output", ".", "artifact output directory")
	flag.StringVar(&osbuildStore, "store", ".osbuild", "osbuild store for intermediate pipeline trees")
	flag.StringVar(&rpmCacheRoot, "rpmmd", "/tmp/rpmmd", "rpm metadata cache directory")
	flag.StringVar(&repositories, "repositories", "test/data/repositories", "path to repository file or directory")
	flag.StringVar(&archName, "arch", "", "target architecture")

	// osbuild checkpoint arg
	var checkpoints cmdutil.MultiValue
	flag.Var(&checkpoints, "checkpoints", "comma-separated list of pipeline names to checkpoint (passed to osbuild --checkpoint)")

	// image selection args
	var distroName, imgTypeName, configFile string
	flag.StringVar(&distroName, "distro", "", "distribution (required)")
	flag.StringVar(&imgTypeName, "type", "", "image type name (required)")
	flag.StringVar(&configFile, "config", "", "build config file (required)")

	// bootc args
	var bootcRef, bootcBuildRef string
	var bootcRemote bool
	flag.StringVar(&bootcRef, "bootc-ref", "", "bootc container image ref (e.g., localhost/bootc-foundry/stream10-qcow2:latest)")
	flag.StringVar(&bootcBuildRef, "bootc-build-ref", "", "separate build container image ref")
	flag.BoolVar(&bootcRemote, "bootc-remote", false, "use org.osbuild.skopeo sources instead of containers-storage")

	flag.Parse()

	if imgTypeName == "" || configFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	if distroName == "" && bootcRef == "" {
		fmt.Fprintf(os.Stderr, "error: either -distro or -bootc-ref is required\n")
		flag.Usage()
		os.Exit(1)
	}
	if distroName != "" && bootcRef != "" {
		fmt.Fprintf(os.Stderr, "error: -distro and -bootc-ref are mutually exclusive\n")
		flag.Usage()
		os.Exit(1)
	}
	if bootcRef != "" && repositories != "test/data/repositories" {
		fmt.Fprintf(os.Stderr, "warning: -repositories is ignored when -bootc-ref is used\n")
	}

	// NOTE: Check the minimum osbuild version before doing anything else.
	// Building the manifest would fail, but we need to depsolve the packages
	// also with the minimum osbuild version. Although the depsolve may fail
	// with an error, it is for the best to fail with the version mismatch
	// error.
	if err := osbuild.CheckMinimumOSBuildVersion(); err != nil {
		return err
	}

	config, err := buildconfig.New(configFile, nil)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0777); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	var distribution distro.Distro
	if bootcRef != "" {
		bootcInfo, err := bootc.ResolveBootcInfo(bootcRef)
		if err != nil {
			return fmt.Errorf("failed to resolve bootc container info: %w", err)
		}

		bootcDistro, err := generic.NewBootc("bootc", bootcInfo)
		if err != nil {
			return fmt.Errorf("failed to create bootc distro: %w", err)
		}

		if bootcBuildRef != "" {
			buildInfo, err := bootc.ResolveBootcBuildInfo(bootcBuildRef)
			if err != nil {
				return fmt.Errorf("failed to resolve bootc build container info: %w", err)
			}
			if err := bootcDistro.SetBuildContainer(buildInfo); err != nil {
				return fmt.Errorf("failed to set build container: %w", err)
			}
		}

		distribution = bootcDistro

		if archName == "" {
			// NOTE: bootcInfo.Arch contains the Docker/OCI arch name (e.g. "amd64"),
			// which must be normalized to the standard name used by the images library
			// (e.g. "x86_64") to match the BootcDistro's internal arch map keys.
			a, err := arch.FromString(bootcInfo.Arch)
			if err != nil {
				return fmt.Errorf("unsupported container architecture %q: %w", bootcInfo.Arch, err)
			}
			archName = a.String()
		}
	} else {
		distroFac := distrofactory.NewDefault()
		distribution = distroFac.GetDistro(distroName)
		if distribution == nil {
			return fmt.Errorf("invalid or unsupported distribution: %q", distroName)
		}
		if archName == "" {
			archName = arch.Current().String()
		}
	}

	archi, err := distribution.GetArch(archName)
	if err != nil {
		return fmt.Errorf("invalid arch name %q for distro %q: %w", archName, distribution.Name(), err)
	}

	buildName := fmt.Sprintf("%s-%s-%s-%s", u(distribution.Name()), u(archName), u(imgTypeName), u(config.Name))
	buildDir := filepath.Join(outputDir, buildName)
	if err := os.MkdirAll(buildDir, 0777); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	imgType, err := archi.GetImageType(imgTypeName)
	if err != nil {
		return fmt.Errorf("invalid image type %q for distro %q and arch %q: %w", imgTypeName, distribution.Name(), archName, err)
	}

	// NOTE: we always put the repositories to be used into the allRepos slice, instead of passing the
	// RepoRegistry to the manifestgen. The reason is that the manifestgen API is too clunky to easily
	// extend the repos list with custom repositories.
	allRepos := []rpmmd.RepoConfig{}
	if bootcRef == "" {
		if st, err := os.Stat(repositories); err == nil && !st.IsDir() {
			// anything that is not a dir is tried to be loaded as a file
			// to allow "-repositories <arbitrarily-named-file>.json"
			repoConfig, err := rpmmd.LoadRepositoriesFromFile(repositories)
			if err != nil {
				return fmt.Errorf("failed to load repositories from %q: %w", repositories, err)
			}
			allRepos = repoConfig[archName]
		} else {
			reporeg, err := reporegistry.New([]string{repositories}, nil)
			if err != nil {
				return fmt.Errorf("failed to load repositories from %q: %w", repositories, err)
			}
			allRepos, err = reporeg.ReposByImageTypeName(distribution.Name(), archName, imgTypeName)
			if err != nil {
				return fmt.Errorf(
					"failed to get repositories for %s/%s/%s: %w", distribution.Name(), archName, imgTypeName, err)
			}
		}
	}
	seedArg, err := cmdutil.SeedArgFor(config, distribution.Name(), archName)
	if err != nil {
		return err
	}

	// Extend the repositories with the custom repositories from the build config
	if len(config.CustomRepos) > 0 {
		allRepos = append(allRepos, config.CustomRepos...)
	}

	fmt.Printf("Generating manifest for %s: ", config.Name)
	manifestOpts := manifestgen.Options{
		Cachedir:       filepath.Join(rpmCacheRoot, archName+distribution.Name()),
		WarningsOutput: os.Stderr,
		OverrideRepos:  allRepos,
		CustomSeed:     &seedArg,
	}
	if archName != arch.Current().String() {
		manifestOpts.UseBootstrapContainer = true
	}
	// add RHSM fact to detect changes
	config.Options.Facts = &facts.ImageOptions{
		APIType: facts.TEST_APITYPE,
	}
	if config.Blueprint == nil {
		config.Blueprint = &blueprint.Blueprint{}
	}

	if bootcRef != "" {
		if config.Options.Bootc == nil {
			config.Options.Bootc = &distro.BootcImageOptions{}
		}
		if bootcRemote {
			config.Options.Bootc.UseRemoteContainerSource = true
		}
	}

	mg, err := manifestgen.New(nil, &manifestOpts)
	if err != nil {
		return fmt.Errorf("[ERROR] manifest generator creation failed: %w", err)
	}
	mf, err := mg.Generate(config.Blueprint, imgType, &config.Options)
	if err != nil {
		return fmt.Errorf("[ERROR] manifest generation failed: %w", err)
	}
	fmt.Print("DONE\n")

	manifestPath := filepath.Join(buildDir, "manifest.json")
	// nolint:gosec
	if err := os.WriteFile(manifestPath, mf, 0644); err != nil {
		return fmt.Errorf("failed to write output file %q: %w", manifestPath, err)
	}

	fmt.Printf("Building manifest: %s\n", manifestPath)

	jobOutput := filepath.Join(outputDir, buildName)
	_, err = osbuild.RunOSBuild(mf, &osbuild.OSBuildOptions{
		StoreDir:    osbuildStore,
		OutputDir:   jobOutput,
		Exports:     imgType.Exports(),
		Checkpoints: checkpoints,
		JSONOutput:  false,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Jobs done. Results saved in\n%s\n", outputDir)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
