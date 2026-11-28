package manifest

import (
	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

type CPIO struct {
	Base
	filename     string
	treePipeline TreePipeline
}

func NewCPIO(buildPipeline Build, treePipeline TreePipeline, name string) *CPIO {
	if name == "" {
		name = "cpio"
	}
	p := &CPIO{
		Base:         NewBase(name, buildPipeline),
		filename:     "image.cpio",
		treePipeline: treePipeline,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *CPIO) Filename() string {
	return p.filename
}

func (p *CPIO) SetFilename(filename string) {
	p.filename = filename
}

func (p *CPIO) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewCPIOOutStage(&osbuild.CPIOOutStageOptions{
		Filename: p.Filename(),
		Format:   osbuild.CPIOArchiveFormatNewc,
	}, p.treePipeline.Name()))
	return pipeline, nil
}

func (p *CPIO) getBuildPackages(Distro) ([]string, error) {
	return []string{"cpio"}, nil
}

func (p *CPIO) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-cpio"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
