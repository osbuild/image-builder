package manifest

import "github.com/osbuild/image-builder/pkg/osbuild"

func (it *ISOOSTree) Serialize() (osbuild.Pipeline, error) {
	return it.serialize()
}
