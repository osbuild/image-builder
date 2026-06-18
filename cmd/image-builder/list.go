package main

import (
	"github.com/osbuild/image-builder/pkg/imagefilter"
)

func listImages(repoDir string, extraRepos []string, forceDefsDir string, output string, filterExprs []string) error {
	imageFilter, err := newImageFilterDefault(repoDir, extraRepos, forceDefsDir)
	if err != nil {
		return err
	}

	filteredResult, err := imageFilter.Filter(filterExprs...)
	if err != nil {
		return err
	}

	fmter, err := imagefilter.NewResultsFormatter(imagefilter.OutputFormat(output))
	if err != nil {
		return err
	}
	if err := fmter.Output(osStdout, filteredResult); err != nil {
		return err
	}

	return nil
}
