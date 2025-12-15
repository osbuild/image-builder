package main

import (
	"github.com/osbuild/images/pkg/imagefilter"
)

func listImages(repoDir string, extraRepos []string, output string, filterExprs []string) error {
	imageFilter, err := newImageFilterDefault(repoDir, extraRepos)
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
