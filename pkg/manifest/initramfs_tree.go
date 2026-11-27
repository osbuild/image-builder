package manifest

import (
	"fmt"
	"slices"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
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
