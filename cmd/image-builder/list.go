package main

import (
	"github.com/osbuild/images/pkg/imagefilter"
)

func listImages(output string, filterExprs []string, opts *cmdlineOpts) error {
	imageFilter, err := newImageFilterDefault(opts.dataDir)
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
	if err := fmter.Output(opts.out, filteredResult); err != nil {
		return err
	}

	return nil
}
