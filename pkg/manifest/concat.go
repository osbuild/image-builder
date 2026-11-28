package manifest

import (
	"fmt"
	"slices"
	"strings"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// Concat joins files from other pipelines into a single file.
type Concat struct {
	Base
	filename       string
	inputPipelines []FilePipeline

	// Alignment is the byte boundary for each input. Zero uses the stage default.
	Alignment uint64
}

func NewConcat(buildPipeline Build, inputPipelines []FilePipeline, name string) *Concat {
	if name == "" {
		name = "concat"
	}
	p := &Concat{
		Base:           NewBase(name, buildPipeline),
		filename:       "image.bin",
		inputPipelines: slices.Clone(inputPipelines),
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *Concat) Filename() string {
	return p.filename
}

func (p *Concat) SetFilename(filename string) {
	p.filename = filename
}

func (p *Concat) serialize() (osbuild.Pipeline, error) {
	if len(p.inputPipelines) == 0 {
		return osbuild.Pipeline{}, fmt.Errorf("concat requires at least one input pipeline")
	}
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	inputs := make(osbuild.ConcatStageInputs, len(p.inputPipelines))
	files := make([]string, 0, len(p.inputPipelines))
	for i, input := range p.inputPipelines {
		name := fmt.Sprintf("file.%d", i)
		filename := strings.TrimLeft(input.Filename(), "/")
		inputs[name] = osbuild.NewFilesInput(osbuild.NewFilesInputPipelineObjectRef(input.Name(), filename, nil))
		files = append(files, fmt.Sprintf("input://%s/%s", name, filename))
	}
	pipeline.AddStage(osbuild.NewConcatStage(&osbuild.ConcatStageOptions{
		Filename:  p.Filename(),
		Files:     files,
		Alignment: p.Alignment,
	}, inputs))
	return pipeline, nil
}

func (p *Concat) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
