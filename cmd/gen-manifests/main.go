// Standalone executable for generating all test manifests in parallel.
// Collects list of image types from the distro list.  Must be run from the
// root of the repository and reads test/data/repositories for repositories
// test/config-list.json to match image types with configuration files.
// Collects errors and failures and prints them after all jobs are finished.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/buildconfig"
	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/bootc"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/generic"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/flatpak"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/manifestgen/manifestmock"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rhsm/facts"
	"github.com/osbuild/images/pkg/rpmmd"
	testrepos "github.com/osbuild/images/test/data/repositories"
)

// panicError is a custom error type to wrap a panic and its stack trace.
type panicError struct {
	panic any
	stack []byte
}

func (e *panicError) Error() string {
	return fmt.Sprintf("panic: %v", e.panic)
}

type buildRequest struct {
	Distro       string                   `json:"distro,omitempty"`
	Arch         string                   `json:"arch,omitempty"`
	ImageType    string                   `json:"image-type,omitempty"`
	Repositories []rpmmd.RepoConfig       `json:"repositories,omitempty"`
	Config       *buildconfig.BuildConfig `json:"config"`
}

type BuildDependency struct {
	Config    string `json:"config"`
	ImageType string `json:"image-type"`
}

type skipDistro struct {
	Name   string    `json:"name"`
	Reason string    `json:"reason"`
	Date   time.Time `json:"date"`
}

// BuildConfigs is a nested map representing the configs to use for each
// distro/arch/image-type. If any component is empty, it maps to all values.
type BuildConfigs struct {
	confMap  map[string]map[string]map[string][]*buildconfig.BuildConfig
	skipList map[*buildconfig.BuildConfig][]skipDistro
}

func newBuildConfigs() *BuildConfigs {
	return &BuildConfigs{
		confMap:  make(map[string]map[string]map[string][]*buildconfig.BuildConfig),
		skipList: make(map[*buildconfig.BuildConfig][]skipDistro),
	}
}

func (bc BuildConfigs) Insert(distro, arch, imageType string, cfg *buildconfig.BuildConfig) {
	distroCfgs := bc.confMap[distro]
	if distroCfgs == nil {
		distroCfgs = make(map[string]map[string][]*buildconfig.BuildConfig)
	}

	distroArchCfgs := distroCfgs[arch]
	if distroArchCfgs == nil {
		distroArchCfgs = make(map[string][]*buildconfig.BuildConfig)
	}

	distroArchItCfgs := distroArchCfgs[imageType]
	if distroArchItCfgs == nil {
		distroArchItCfgs = make([]*buildconfig.BuildConfig, 0)
	}

	distroArchItCfgs = append(distroArchItCfgs, cfg)
	distroArchCfgs[imageType] = distroArchItCfgs
	distroCfgs[arch] = distroArchCfgs
	bc.confMap[distro] = distroCfgs
}

func (bc BuildConfigs) needsSkipping(distro string, cfg *buildconfig.BuildConfig) (bool, string) {
	for _, s := range bc.skipList[cfg] {
		if s.Name == distro {
			if time.Since(s.Date) > 90*24*time.Hour {
				err := fmt.Errorf("distro %q is temporarily skipped for more than 90 days (added %q)", s.Name, s.Date)
				panic(err)
			}
			return true, s.Reason
		}
	}

	return false, ""
}

func (bc BuildConfigs) Get(distro, arch, imageType string) []*buildconfig.BuildConfig {

	configs := make([]*buildconfig.BuildConfig, 0)
	for distroName, distroCfgs := range bc.confMap {
		distroGlob := glob.MustCompile(distroName)
		if distroGlob.Match(distro) {
			for archName, distroArchCfgs := range distroCfgs {
				archGlob := glob.MustCompile(archName)
				if archGlob.Match(arch) {
					for itName, distroArchItCfgs := range distroArchCfgs {
						itGlob := glob.MustCompile(itName)
						if itGlob.Match(imageType) {
							configs = append(configs, distroArchItCfgs...)
						}
					}
				}
			}
		}
	}
	return configs
}

func loadConfigList(configPath string, opts *buildconfig.Options) *BuildConfigs {

	type configFilters struct {
		ImageTypes  []string     `json:"image-types"`
		Distros     []string     `json:"distros"`
		SkipDistros []skipDistro `json:"skip-distros"`
		Arches      []string     `json:"arches"`
	}

	type configItem struct {
		Path    string        `json:"path"`
		Filters configFilters `json:"filters"`
	}

	type configList []configItem

	fp, err := os.Open(configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open config list %q: %s", configPath, err.Error()))
	}
	defer fp.Close()

	var cfgList configList
	if err := json.NewDecoder(fp).Decode(&cfgList); err != nil {
		panic(fmt.Sprintf("failed to unmarshal config list %q: %s", configPath, err.Error()))
	}

	emptyFallback := func(list []string) []string {
		if len(list) == 0 {
			// empty list means everything so let's add the * to catch
			// everything with glob
			return []string{"*"}
		}
		return list
	}

	// load each config from its path
	cm := newBuildConfigs()
	for _, cfgItem := range cfgList {
		// config paths can be relative to the location of the config list
		path := cfgItem.Path
		filters := cfgItem.Filters
		if !filepath.IsAbs(path) {
			cfgDir := filepath.Dir(configPath)
			path = filepath.Join(cfgDir, path)
		}
		config, err := buildconfig.New(path, opts)
		if err != nil {
			panic(err)
		}
		for _, d := range emptyFallback(filters.Distros) {
			if len(filters.SkipDistros) > 0 {
				cm.skipList[config] = append(cm.skipList[config], filters.SkipDistros...)
			}
			for _, a := range emptyFallback(filters.Arches) {
				for _, it := range emptyFallback(filters.ImageTypes) {
					cm.Insert(d, a, it, config)
				}
			}
		}
	}

	return cm
}

// loadImgConfig loads a single image config from a file and returns a
// BuildConfigs map with the config mapped to all distros, arches, and image
// types.
func loadImgConfig(configPath string, opts *buildconfig.Options) *BuildConfigs {
	cm := newBuildConfigs()
	config, err := buildconfig.New(configPath, opts)
	if err != nil {
		panic(err)
	}
	cm.Insert("*", "*", "*", config)
	return cm
}

type manifestJob func(chan string) error

func makeManifestJob(
	bc *buildconfig.BuildConfig,
	imgType distro.ImageType,
	distribution distro.Distro,
	repos []rpmmd.RepoConfig,
	archName string,
	cacheRoot string,
	path string,
	content map[string]bool,
	metadata bool,
	tmpdirRoot string,
	bootcRemote bool,
	bootcInstallerRef string,
) manifestJob {
	name := bc.Name
	distroName := distribution.Name()
	filename := fmt.Sprintf("%s-%s-%s-%s.json", u(distroName), u(archName), u(imgType.Name()), u(name))
	cacheDir := filepath.Join(cacheRoot, archName+distribution.Name())

	// ensure that each distro/arch has a unique seed, we do not include the image type name here to
	// avoid checksum changes when just the name changes (but no content changes)
	seedArg, err := cmdutil.SeedArgFor(bc, distribution.Name(), archName)
	if err != nil {
		panic(err)
	}

	options := bc.Options
	if bootcRemote || bootcInstallerRef != "" {
		if options.Bootc == nil {
			options.Bootc = &distro.BootcImageOptions{}
		} else {
			bootcOpts := *options.Bootc
			options.Bootc = &bootcOpts
		}
		if bootcRemote {
			options.Bootc.UseRemoteContainerSource = true
		}
		if bootcInstallerRef != "" {
			options.Bootc.InstallerPayloadRef = bootcInstallerRef
		}
	}

	var bp blueprint.Blueprint
	if bc.Blueprint != nil {
		bp = *bc.Blueprint
	}
	if experimentalflags.Bool("gen-manifest-mock-bpfile-uris") && bp.Customizations != nil {
		for i, fc := range bp.Customizations.Files {
			if fc.URI != "" {
				newBpFileUrl := filepath.Join(tmpdirRoot, "fake-bp-files-with-urls", fmt.Sprintf("%x", sha256.Sum256([]byte(fc.URI))))
				if err := os.MkdirAll(filepath.Dir(newBpFileUrl), 0755); err != nil {
					panic(err)
				}
				if err := os.WriteFile(newBpFileUrl, []byte(fc.URI), 0600); err != nil {
					panic(err)
				}
				bp.Customizations.Files[i].URI = "file://" + newBpFileUrl
			}
		}
	}

	// add RHSM fact to detect changes
	options.Facts = &facts.ImageOptions{
		APIType: facts.TEST_APITYPE,
	}

	// Extend the repositories with the custom repositories from the build config
	allRepos := slices.Clone(repos)
	if len(bc.CustomRepos) > 0 {
		allRepos = append(allRepos, bc.CustomRepos...)
	}

	job := func(msgq chan string) (err error) {
		defer func() {
			msg := fmt.Sprintf("Finished job %s", filename)
			if err != nil {
				msg += " [failed]"
			}
			msgq <- msg
		}()
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("[%s] failed: %w", filename, &panicError{
					panic: r,
					stack: debug.Stack(),
				})
			}
		}()
		msgq <- fmt.Sprintf("Starting job %s", filename)

		manifest, _, err := imgType.Manifest(&bp, options, allRepos, &seedArg)
		if err != nil {
			err = fmt.Errorf("[%s] failed: %s", filename, err)
			return
		}

		var depsolvedSets map[string]depsolvednf.DepsolveResult
		if content["packages"] {
			solver := depsolvednf.NewSolver(distribution.ModulePlatformID(), distribution.Releasever(), archName, distribution.Name(), cacheDir)
			depsolvedSets, err = solver.DepsolveAll(common.Must(manifest.GetPackageSetChains()))
			if err != nil {
				err = fmt.Errorf("[%s] depsolve failed: %s", filename, err.Error())
				return
			}
			for plName, depsolved := range depsolvedSets {
				if len(depsolved.Transactions.AllPackages()) == 0 {
					err = fmt.Errorf("[%s] no packages in the depsolve result for %v", filename, plName)
					return
				}
			}
		} else {
			depsolvedSets, err = manifestmock.Depsolve(common.Must(manifest.GetPackageSetChains()), archName, bc, false)
			if err != nil {
				err = fmt.Errorf("[%s] manifestmock depsolve failed: %s", filename, err.Error())
				return
			}
		}

		var containerSpecs map[string][]container.Spec
		if content["containers"] {
			containerSpecs, err = container.NewResolver(archName).ResolveAll(manifest.GetContainerSourceSpecs())
			if err != nil {
				return fmt.Errorf("[%s] container resolution failed: %s", filename, err.Error())
			}
		} else {
			containerSpecs = manifestmock.ResolveContainers(manifest.GetContainerSourceSpecs())
		}

		var commitSpecs map[string][]ostree.CommitSpec
		if content["commits"] {
			commitSpecs, err = ostree.ResolveAll(manifest.GetOSTreeSourceSpecs())
			if err != nil {
				return fmt.Errorf("[%s] ostree commit resolution failed: %s", filename, err.Error())
			}
		} else {
			commitSpecs = manifestmock.ResolveCommits(manifest.GetOSTreeSourceSpecs())
		}

		var flatpakSpecs map[string][]flatpak.Spec
		if content["flatpaks"] {
			flatpakSpecs, err = flatpak.ResolveAll(manifest.GetFlatpakSourceSpecs())
			if err != nil {
				return fmt.Errorf("[%s] flatpak resolution failed: %s", filename, err.Error())
			}
		} else {
			flatpakSpecs = manifestmock.ResolveFlatpaks(manifest.GetFlatpakSourceSpecs())
		}

		mf, err := manifest.Serialize(depsolvedSets, containerSpecs, commitSpecs, flatpakSpecs, nil)
		if err != nil {
			return fmt.Errorf("[%s] manifest serialization failed: %s", filename, err.Error())
		}

		request := buildRequest{
			Distro:       distribution.Name(),
			Arch:         archName,
			ImageType:    imgType.Name(),
			Repositories: allRepos,
			Config:       bc,
		}
		err = save(mf, depsolvedSets, containerSpecs, commitSpecs, flatpakSpecs, request, path, filename, metadata)
		return
	}
	return job
}

func save(ms manifest.OSBuildManifest, depsolved map[string]depsolvednf.DepsolveResult, containers map[string][]container.Spec, commits map[string][]ostree.CommitSpec, flatpaks map[string][]flatpak.Spec, cr buildRequest, path, filename string, metadata bool) error {
	var data any
	if metadata {
		rpmmds := make(map[string]rpmmd.PackageList)
		for plName, res := range depsolved {
			rpmmds[plName] = res.Transactions.AllPackages()
		}
		data = struct {
			BuidRequest   buildRequest                   `json:"build-request"`
			Manifest      manifest.OSBuildManifest       `json:"manifest"`
			RPMMD         map[string]rpmmd.PackageList   `json:"rpmmd"`
			Containers    map[string][]container.Spec    `json:"containers,omitempty"`
			OSTreeCommits map[string][]ostree.CommitSpec `json:"ostree-commits,omitempty"`
			Flatpaks      map[string][]flatpak.Spec      `json:"flatpaks,omitempty"`
			NoImageInfo   bool                           `json:"no-image-info"`
		}{
			cr, ms, rpmmds, containers, commits, flatpaks, true,
		}
	} else {
		data = ms
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %q: %s\n", filename, err.Error())
	}
	b = append(b, '\n') // add new line at end of file
	fpath := filepath.Join(path, filename)
	fp, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %s\n", fpath, err.Error())
	}
	defer fp.Close()
	if _, err := fp.Write(b); err != nil {
		return fmt.Errorf("failed to write output file %q: %s\n", fpath, err.Error())
	}
	return nil
}

func u(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func main() {
	// common args
	var outputDir, cacheRoot, configPath, configMapPath string
	var nWorkers int
	var metadata, skipNoconfig, skipNorepos, buildconfigAllowUnknown bool
	flag.StringVar(&outputDir, "output", "test/data/manifests/", "manifest store directory")
	flag.IntVar(&nWorkers, "workers", 16, "number of workers to run concurrently")
	flag.StringVar(&cacheRoot, "cache", "/tmp/rpmmd", "rpm metadata cache directory")
	flag.BoolVar(&metadata, "metadata", true, "store metadata in the file")
	flag.StringVar(&configPath, "config", "", "image config file to use for all images (overrides -config-list)")
	flag.StringVar(&configMapPath, "config-list", "test/config-list.json", "configuration file mapping image types to configs")
	flag.BoolVar(&skipNoconfig, "skip-noconfig", false, "skip distro-arch-image configurations that have no config (otherwise fail)")
	flag.BoolVar(&skipNorepos, "skip-norepos", false, "skip distro-arch-image configurations that have no repositories (otherwise fail)")
	flag.BoolVar(&buildconfigAllowUnknown, "buildconfig-allow-unknown", false, "allow unknown keys in buildconfig")

	// content args
	var packages, containers, commits, flatpaks, fakeBootc bool
	flag.BoolVar(&packages, "packages", true, "depsolve package sets")
	flag.BoolVar(&containers, "containers", true, "resolve container checksums")
	flag.BoolVar(&commits, "commits", false, "resolve ostree commit IDs")
	flag.BoolVar(&flatpaks, "flatpaks", false, "resolve flatpak checksums")
	flag.BoolVar(&fakeBootc, "fake-bootc", false, "create fake bootc containers based on test/bootc-fake-containers.yaml")

	// manifest selection args
	var arches, distros, imgTypes, bootcRefs cmdutil.MultiValue
	flag.Var(&arches, "arches", "comma-separated list of architectures (globs supported)")
	flag.Var(&distros, "distros", "comma-separated list of distributions (globs supported)")
	flag.Var(&imgTypes, "types", "comma-separated list of image types (globs supported)")
	flag.Var(&bootcRefs, "bootc-refs", "comma-separated list of bootc container refs to generate manifests for (format: ref or ref#build-ref)")

	// bootc options
	var bootcRemote bool
	flag.BoolVar(&bootcRemote, "bootc-remote", false, "generate bootc manifests with org.osbuild.skopeo sources instead of containers-storage")
	var bootcInstallerRef string
	flag.StringVar(&bootcInstallerRef, "bootc-installer-ref", "", "override the installer payload container ref for bootc-installer manifests")

	// dry-run
	var dryRun bool
	flag.BoolVar(&dryRun, "dry-run", false, "print what manifests would be generated")

	flag.Parse()

	testedRepoRegistry, err := testrepos.New()
	if err != nil {
		panic(fmt.Sprintf("failed to create repo registry with tested distros: %v", err))
	}

	distroFac := distrofactory.NewDefault()
	jobs := make([]manifestJob, 0)

	contentResolve := map[string]bool{
		"packages":   packages,
		"containers": containers,
		"commits":    commits,
		"flatpaks":   flatpaks,
	}

	var configs *BuildConfigs
	opts := &buildconfig.Options{AllowUnknownFields: buildconfigAllowUnknown}
	if configPath != "" {
		fmt.Fprintln(os.Stderr, "'-config' was provided, thus ignoring '-config-list' option")
		configs = loadImgConfig(configPath, opts)
	} else {
		configs = loadConfigList(configMapPath, opts)
	}

	if err := os.MkdirAll(outputDir, 0770); err != nil {
		panic(fmt.Sprintf("failed to create target directory: %s", err.Error()))
	}

	// temporary directory for mocking file embeds with URIs (and anything else
	// we might need to write temporarily)
	// We can't use os.MkdirTemp to get an uniquw tmp dir here because the path
	// of the CURL source in the manifest would change every time we run this tool.
	tmpdirRoot := filepath.Join(os.TempDir(), "gen-manifests-tmpdir")
	defer os.RemoveAll(tmpdirRoot)

	fmt.Fprintln(os.Stderr, "Collecting jobs")

	distros, invalidDistros := distros.ResolveArgValues(testedRepoRegistry.ListDistros())
	if len(invalidDistros) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid distro names: [%s]\n", strings.Join(invalidDistros, ","))
	}
	for _, distroName := range distros {
		distribution := distroFac.GetDistro(distroName)
		if distribution == nil {
			fmt.Fprintf(os.Stderr, "WARNING: invalid distro name %q\n", distroName)
			continue
		}

		distroArches, invalidArches := arches.ResolveArgValues(distribution.ListArches())
		if len(invalidArches) > 0 {
			fmt.Fprintf(os.Stderr, "WARNING: invalid arch names [%s] for distro %q\n", strings.Join(invalidArches, ","), distroName)
		}
		for _, archName := range distroArches {
			arch, err := distribution.GetArch(archName)
			if err != nil {
				// resolveArgValues should prevent this
				panic(fmt.Sprintf("invalid arch name %q for distro %q: %s\n", archName, distroName, err.Error()))
			}

			daImgTypes, invalidImageTypes := imgTypes.ResolveArgValues(arch.ListImageTypes())
			if len(invalidImageTypes) > 0 {
				fmt.Fprintf(os.Stderr, "WARNING: invalid image type names [%s] for distro %q and arch %q\n", strings.Join(invalidImageTypes, ","), distroName, archName)
			}
			for _, imgTypeName := range daImgTypes {
				imgType, err := arch.GetImageType(imgTypeName)
				if err != nil {
					// resolveArgValues should prevent this
					panic(fmt.Sprintf("invalid image type %q for distro %q and arch %q: %s\n", imgTypeName, distroName, archName, err.Error()))
				}

				// get repositories
				repos, err := testedRepoRegistry.ReposByImageTypeName(distroName, archName, imgTypeName)
				if err != nil {
					panic(fmt.Sprintf("failed to get repositories for %s/%s: %v", distroName, archName, err))
				}
				if len(repos) == 0 {
					fmt.Printf("no repositories defined for %s/%s/%s\n", distroName, archName, imgTypeName)
					if skipNorepos {
						fmt.Fprintln(os.Stderr, "Skipping")
						continue
					}
					panic("no repositories found, pass --skip-norepos to skip")
				}

				imgTypeConfigs := configs.Get(distroName, archName, imgTypeName)
				if len(imgTypeConfigs) == 0 {
					if skipNoconfig {
						continue
					}
					panic(fmt.Sprintf("no configs defined for image type %q for %s", imgTypeName, distribution.Name()))
				}

				for _, itConfig := range imgTypeConfigs {
					if needsSkipping, reason := configs.needsSkipping(distribution.Name(), itConfig); needsSkipping {
						fmt.Printf("Skipping %s for %s/%s (reason: %v)\n", itConfig.Name, imgTypeName, distribution.Name(), reason)
						continue
					}

					if dryRun {
						fmt.Printf("%s,%s,%s,%s\n", distribution.Name(), archName, imgType.Name(), itConfig.Name)
					} else {
						job := makeManifestJob(itConfig, imgType, distribution, repos, archName, cacheRoot, outputDir, contentResolve, metadata, tmpdirRoot, false, "")
						jobs = append(jobs, job)
					}
				}
			}
		}
	}

	for _, bootcRefTuple := range bootcRefs {
		l := strings.SplitN(bootcRefTuple, "#", 2)
		bootcRef := l[0]

		bootcInfo, err := bootc.ResolveBootcInfo(bootcRef)
		if err != nil {
			panic(err)
		}
		// consider making this configurable but for now we just need
		// diffable manifests
		if bootcInfo.DefaultRootFs == "" {
			bootcInfo.DefaultRootFs = "ext4"
		}

		distribution, err := generic.NewBootc("bootc", bootcInfo)
		if err != nil {
			panic(err)
		}

		var buildBootcRef string
		if len(l) > 1 {
			buildBootcRef = l[1]
			buildBootcInfo, err := bootc.ResolveBootcBuildInfo(buildBootcRef)
			if err != nil {
				panic(err)
			}
			if err := distribution.SetBuildContainer(buildBootcInfo); err != nil {
				panic(err)
			}
		}

		for _, archName := range arches {
			archi, err := distribution.GetArch(archName)
			if err != nil {
				panic(err)
			}
			for _, imgTypeName := range imgTypes {
				imgType, err := archi.GetImageType(imgTypeName)
				if err != nil {
					panic(err)
				}
				// XXX: copied from loop above
				imgTypeConfigs := configs.Get(distribution.Name(), archName, imgTypeName)
				if len(imgTypeConfigs) == 0 {
					if skipNoconfig {
						continue
					}
					panic(fmt.Sprintf("no configs defined for image type %q for %s/%s", imgTypeName, distribution.Name(), archi.Name()))
				}
				for _, itConfig := range imgTypeConfigs {
					if needsSkipping, reason := configs.needsSkipping(distribution.Name(), itConfig); needsSkipping {
						fmt.Printf("Skipping %s for %s/%s (reason: %v)\n", itConfig.Name, imgTypeName, distribution.Name(), reason)
						continue
					}

					if dryRun {
						fmt.Printf("%s,%s,%s,%s\n", distribution.Name(), archName, imgType.Name(), itConfig.Name)
					} else {
						var repos []rpmmd.RepoConfig
						job := makeManifestJob(itConfig, imgType, distribution, repos, archName, cacheRoot, outputDir, contentResolve, metadata, tmpdirRoot, bootcRemote, bootcInstallerRef)
						jobs = append(jobs, job)
					}
				}
			}
		}
	}
	if fakeBootc {
		p := "test/bootc-fake-containers.yaml"
		f, err := os.Open(p)
		if err != nil {
			panic(fmt.Errorf("cannot find %q", p))
		}
		defer f.Close()
		type fakeBootcContainerYAML struct {
			Arch          arch.Arch   `yaml:"arch"`
			Info          osinfo.Info `yaml:"info"`
			DefaultFs     string      `yaml:"default_fs"`
			ContainerSize uint64      `yaml:"container_size"`
			ImageRef      string      `yaml:"image_ref"`
			ImageTypes    []string    `yaml:"image_types"`

			BuildContainerRef  string      `yaml:"build_container_ref"`
			BuildContainerInfo osinfo.Info `yaml:"build_container_info"`
		}
		type fakeContainersYAML struct {
			Containers []fakeBootcContainerYAML
		}
		var fakeContainers fakeContainersYAML
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		if err := dec.Decode(&fakeContainers); err != nil {
			panic(err)
		}
		for _, fakeBootcCnt := range fakeContainers.Containers {
			fakeBootcInfo := &bootc.Info{
				Imgref:        fakeBootcCnt.ImageRef,
				OSInfo:        &fakeBootcCnt.Info,
				Arch:          fakeBootcCnt.Arch.String(),
				DefaultRootFs: fakeBootcCnt.DefaultFs,
				Size:          fakeBootcCnt.ContainerSize,
			}
			distribution, err := generic.NewBootc("bootc", fakeBootcInfo)
			if err != nil {
				panic(err)
			}

			arches, _ := arches.ResolveArgValues(distribution.ListArches())
			for _, archName := range arches {
				archi, err := distribution.GetArch(archName)
				if err != nil {
					panic(err)
				}
				imgTypes, _ := imgTypes.ResolveArgValues(archi.ListImageTypes())
				for _, imgTypeName := range imgTypes {
					if !slices.Contains(fakeBootcCnt.ImageTypes, imgTypeName) {
						continue
					}

					if fakeBootcCnt.BuildContainerRef != "" {
						buildContainerInfo := &bootc.Info{
							Imgref: fakeBootcCnt.BuildContainerRef,
							OSInfo: &fakeBootcCnt.BuildContainerInfo,
							Arch:   fakeBootcInfo.Arch,
						}
						if err := distribution.SetBuildContainer(buildContainerInfo); err != nil {
							panic(err)
						}
					}

					imgType, err := archi.GetImageType(imgTypeName)
					if err != nil {
						panic(err)
					}
					// XXX: copied from loop above
					imgTypeConfigs := configs.Get(distribution.Name(), archName, imgTypeName)
					if len(imgTypeConfigs) == 0 {
						if skipNoconfig {
							continue
						}
						panic(fmt.Sprintf("no configs defined for image type %q for %s/%s", imgTypeName, distribution.Name(), archi.Name()))
					}
					for _, itConfig := range imgTypeConfigs {
						if needsSkipping, reason := configs.needsSkipping(distribution.Name(), itConfig); needsSkipping {
							fmt.Printf("Skipping %s for %s/%s (reason: %v)\n", itConfig.Name, imgTypeName, distribution.Name(), reason)
							continue
						}

						var repos []rpmmd.RepoConfig
						job := makeManifestJob(itConfig, imgType, distribution, repos, archName, cacheRoot, outputDir, contentResolve, metadata, tmpdirRoot, bootcRemote, bootcInstallerRef)
						jobs = append(jobs, job)
					}
				}
			}
		}
	}

	nJobs := len(jobs)
	fmt.Fprintf(os.Stderr, "Collected %d jobs\n", nJobs)

	// nolint:gosec
	wq := newWorkerQueue(uint32(nWorkers), uint32(nJobs))
	wq.start()
	fmt.Fprintf(os.Stderr, "Initialised %d workers\n", nWorkers)
	fmt.Fprintf(os.Stderr, "Submitting %d jobs... ", nJobs)
	for _, j := range jobs {
		wq.submitJob(j)
	}
	fmt.Fprintln(os.Stderr, "done")
	errs := wq.wait()
	exit := 0
	if nErrs := len(errs); nErrs > 0 {
		fmt.Fprintf(os.Stderr, "Encountered %d errors:\n", nErrs)
		var err1 *panicError
		for idx, err := range errs {
			fmt.Fprintf(os.Stderr, "%3d: %s\n", idx, err.Error())
			if err1 == nil {
				errors.As(err, &err1)
			}
		}

		if err1 != nil {
			fmt.Fprintf(os.Stderr, "\nStack trace of the first error:\n%s\n", err1.stack)
		}
		exit = 1
	}
	fmt.Fprintf(os.Stderr, "RPM metadata cache kept in %s\n", cacheRoot)
	os.Exit(exit)
}
