package manifest

import "github.com/osbuild/images/pkg/osbuild"

func (bpt *BootcPXETree) Serialize() (osbuild.Pipeline, error) {
	return bpt.serialize()
}

func (bpt *BootcPXETree) SerializeStart(inputs Inputs) error {
	return bpt.serializeStart(inputs)
}
