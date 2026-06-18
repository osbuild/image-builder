package manifest

import (
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func (br *BuildrootFromContainer) Dependents() []Pipeline {
	return br.dependents
}

func (rbc *RawBootcImage) Serialize() (osbuild.Pipeline, error) {
	return rbc.serialize()
}

func (rbc *RawBootcImage) SerializeStart(inputs Inputs) error {
	return rbc.serializeStart(inputs)
}
