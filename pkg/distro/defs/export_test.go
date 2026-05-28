package defs

import (
	"os"
)

type WhenCondition = whenCondition

func MockDataFS(path string) (restore func()) {
	saved := defaultDataFS
	defaultDataFS = os.DirFS(path)
	return func() {
		defaultDataFS = saved
	}
}

func LoaderForTest(path string) *Loader {
	return NewLoader(os.DirFS(path))
}

func ClearLoader(d *DistroYAML) {
	d.loader = nil
}
