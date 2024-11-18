package main

import (
	"io"

	"github.com/osbuild/images/pkg/imagefilter"
)

func listImages(out io.Writer, output string, filterExprs []string) error {
	imageFilter, err := newImageFilterDefault()
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
	if err := fmter.Output(out, filteredResult); err != nil {
		return err
	}

	return nil
}
