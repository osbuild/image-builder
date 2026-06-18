package manifest

import "github.com/osbuild/image-builder/pkg/osbuild"

func (bpt *BootcRootFS) Serialize() (osbuild.Pipeline, error) {
	return bpt.serialize()
}

func (bpt *BootcRootFS) SerializeStart(inputs Inputs) error {
	return bpt.serializeStart(inputs)
}
