package manifest

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var (
	DistroNames  = distroNames
	DISTRO_COUNT = _distro_count
)

func (p *OS) GetBuildPackages(d Distro) ([]string, error) {
	return p.getBuildPackages(d)
}

func (p *OS) GetPackageSetChain(d Distro) ([]rpmmd.PackageSet, error) {
	return p.getPackageSetChain(d)
}

func (p *OS) AddStagesForAllFilesAndInlineData(pipeline *osbuild.Pipeline, files []*fsnode.File) {
	p.addStagesForAllFilesAndInlineData(pipeline, files)
}

// NewTestOS is used in both internal and external package tests.
// TODO: make all tests external and define this only in the manifest_test
// package.
func NewTestOS() *OS {
	repos := []rpmmd.RepoConfig{}
	m := New()
	runner := &runner.Fedora{Version: 38}
	build := NewBuild(&m, runner, repos, nil)
	build.Checkpoint()

	// create an x86_64 platform with bios boot
	platform := &platform.Data{
		Arch:         arch.ARCH_X86_64,
		BIOSPlatform: "i386-pc",
	}

	os := NewOS(build, platform, repos)

	return os
}

func (p *OSTreeDeployment) AddStagesForAllFilesAndInlineData(pipeline *osbuild.Pipeline, files []*fsnode.File) {
	p.addStagesForAllFilesAndInlineData(pipeline, files, "ostree/ref")
}

func (p *Vagrant) GetMacAddress() string {
	return p.macAddress
}

func Serialize(p Pipeline) (osbuild.Pipeline, error) {
	return p.serialize()
}

func SerializeWith(p Pipeline, inputs Inputs) (osbuild.Pipeline, error) {
	err := p.serializeStart(inputs)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	return p.serialize()
}

var MakeKickstartSudoersPost = makeKickstartSudoersPost

func GetInline(p Pipeline) []string {
	return p.getInline()
}

func (p *OS) Serialize() (osbuild.Pipeline, error) {
	repo := rpmmd.RepoConfig{Id: "dummy-repo-id"}
	transaction := depsolvednf.TransactionList{
		{
			{
				Name:     "pkg1",
				Checksum: rpmmd.Checksum{Type: "sha256", Value: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
				RepoID:   repo.Id,
				Repo:     &repo,
			},
		},
		{
			{
				Name:     "pkg2",
				Checksum: rpmmd.Checksum{Type: "sha256", Value: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
				RepoID:   repo.Id,
				Repo:     &repo,
			},
		},
	}
	err := p.serializeStart(Inputs{
		Depsolved: depsolvednf.DepsolveResult{
			Transactions: transaction,
			Repos:        []rpmmd.RepoConfig{repo},
		},
	})
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	return p.serialize()
}
