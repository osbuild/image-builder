package manifest

import "github.com/osbuild/image-builder/pkg/osbuild"

func (it *ISOTree) Serialize() (osbuild.Pipeline, error) {
	return it.serialize()
}
