package main

import (
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/imagefilter"
)

func newImageFilterDefault() (*imagefilter.ImageFilter, error) {
	fac := distrofactory.NewDefault()
	repos, err := newRepoRegistry()
	if err != nil {
		return nil, err
	}
	return imagefilter.New(fac, repos)
}
