package manifest

import (
	"fmt"
	"slices"

	"github.com/osbuild/image-builder/pkg/customizations/firmware"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/customizations/kernelmodules"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

type InitramfsTreeCustomizations struct {
	PackageSet       rpmmd.PackageSet
	BaseRPMOptions   osbuild.RPMStageOptions
	Directories      []*fsnode.Directory
	Symlinks         []*fsnode.Symlink
	Files            []*fsnode.File
	EnabledServices  []string
	DisabledServices []string
	MaskedServices   []string
	KernelModules    *kernelmodules.KernelModules
	Firmware         *firmware.Firmware
}

// InitramfsTree builds the early userspace filesystem for an initramfs.
type InitramfsTree struct {
	Base
	platform       platform.Platform
	repos          []rpmmd.RepoConfig
	depsolveResult *depsolvednf.DepsolveResult
	Customizations InitramfsTreeCustomizations
}

func NewInitramfsTree(build Build, platform platform.Platform, repos []rpmmd.RepoConfig) *InitramfsTree {
	const name = "initramfs-tree"
	p := &InitramfsTree{
		Base:     NewBase(name, build),
		platform: platform,
		repos:    filterRepos(repos, name),
	}
	build.addDependent(p)
	return p
}

func (p *InitramfsTree) Platform() platform.Platform {
	return p.platform
}

func (p *InitramfsTree) getBuildPackages(Distro) ([]string, error) {
	packages := []string{"rpm"}
	if p.Customizations.KernelModules != nil || p.Customizations.Firmware != nil {
		packages = append(packages, "coreutils")
	}
	if len(p.Customizations.EnabledServices)+len(p.Customizations.DisabledServices)+len(p.Customizations.MaskedServices) > 0 {
		packages = append(packages, "systemd")
	}
	return packages, nil
}

func (p *InitramfsTree) getPackageSetChain(Distro) ([]rpmmd.PackageSet, error) {
	pkgSet := p.Customizations.PackageSet
	if len(pkgSet.Include) == 0 {
		return nil, nil
	}
	pkgSet.Repositories = slices.Concat(p.repos, pkgSet.Repositories)
	return []rpmmd.PackageSet{pkgSet}, nil
}

func (p *InitramfsTree) serializeStart(inputs Inputs) error {
	if p.depsolveResult != nil {
		return fmt.Errorf("InitramfsTree: double call to serializeStart()")
	}
	p.depsolveResult = &inputs.Depsolved
	return nil
}

func (p *InitramfsTree) serializeEnd() {
	p.depsolveResult = nil
}

func (p *InitramfsTree) serialize() (osbuild.Pipeline, error) {
	if p.depsolveResult == nil {
		return osbuild.Pipeline{}, fmt.Errorf("InitramfsTree: serialization not started")
	}
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	options := p.Customizations.BaseRPMOptions.Clone()
	options.DisableDracut = true
	rpmStages, err := osbuild.GenRPMStagesFromTransactions(p.depsolveResult.Transactions, options)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	pipeline.AddStages(rpmStages...)
	if p.Customizations.KernelModules != nil {
		stage, err := p.kernelModulesStage()
		if err != nil {
			return osbuild.Pipeline{}, err
		}
		pipeline.AddStage(stage)
	}
	if p.Customizations.Firmware != nil {
		source, err := p.sourceTree(p.Customizations.Firmware.From, "firmware")
		if err != nil {
			return osbuild.Pipeline{}, err
		}
		pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
			Paths: []osbuild.MkdirStagePath{{Path: "tree:///usr/lib/firmware", Parents: true, ExistOk: true}},
		}))
		pipeline.AddStage(osbuild.NewCopyStageSimple(&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{{
				From: "input://firmware/usr/lib/firmware/",
				To:   "tree:///usr/lib/firmware/",
			}},
		}, osbuild.NewPipelineTreeInputs("firmware", source.Name())))
	}
	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{Path: "tree:///dev", ExistOk: true},
			{Path: "tree:///proc", ExistOk: true},
			{Path: "tree:///run", ExistOk: true},
			{Path: "tree:///sys", ExistOk: true},
		},
	}))
	pipeline.AddStages(osbuild.GenDirectoryNodesStages(p.Customizations.Directories)...)
	pipeline.AddStages(osbuild.GenSymlinkNodesStages(p.Customizations.Symlinks)...)
	pipeline.AddStages(osbuild.GenFileNodesStages(p.Customizations.Files)...)
	if len(p.Customizations.EnabledServices)+len(p.Customizations.DisabledServices)+len(p.Customizations.MaskedServices) > 0 {
		pipeline.AddStage(osbuild.NewSystemdStage(&osbuild.SystemdStageOptions{
			EnabledServices:  p.Customizations.EnabledServices,
			DisabledServices: p.Customizations.DisabledServices,
			MaskedServices:   p.Customizations.MaskedServices,
		}))
	}
	return pipeline, nil
}

func (p *InitramfsTree) kernelModulesStage() (*osbuild.Stage, error) {
	from := p.Customizations.KernelModules.From
	source, err := p.sourceTree(from, "kernel_modules")
	if err != nil {
		return nil, err
	}
	kernelSource, ok := source.(interface{ KernelVersion() string })
	if !ok {
		return nil, fmt.Errorf("kernel_modules source %q does not provide a kernel tree", from)
	}
	version := kernelSource.KernelVersion()
	if version == "" {
		return nil, fmt.Errorf("kernel_modules source %q has no resolved kernel version", from)
	}
	return osbuild.NewKernelModulesCopyStage(version, from), nil
}

func (p *InitramfsTree) sourceTree(from, customization string) (TreePipeline, error) {
	if from == "" {
		return nil, fmt.Errorf("%s.from must name a source pipeline", customization)
	}
	for _, source := range p.Manifest().pipelines {
		if source == p {
			break
		}
		if source.Name() != from {
			continue
		}
		tree, ok := source.(TreePipeline)
		if !ok {
			return nil, fmt.Errorf("%s source %q does not provide a directory tree", customization, from)
		}
		return tree, nil
	}
	return nil, fmt.Errorf("%s source %q must precede pipeline %q", customization, from, p.Name())
}

func (p *InitramfsTree) getInline() []string {
	var data []string
	for _, file := range p.Customizations.Files {
		if file.URI() == "" {
			data = append(data, string(file.Data()))
		}
	}
	return data
}

func (p *InitramfsTree) fileRefs() ([]string, error) {
	var refs []string
	for _, file := range p.Customizations.Files {
		if uri := file.URI(); uri != "" {
			refs = append(refs, uri)
		}
	}
	return refs, nil
}
