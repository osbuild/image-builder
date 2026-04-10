package imagefilter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/osbuild/images/pkg/distrosort"
)

// OutputFormat contains the valid output formats for formatting results
type OutputFormat string

const (
	OutputFormatDefault   OutputFormat = ""
	OutputFormatText      OutputFormat = "text"
	OutputFormatJSON      OutputFormat = "json"
	OutputFormatTextShell OutputFormat = "shell"
	OutputFormatTextShort OutputFormat = "short"
)

// ResultFormatter will format the given result list to the given io.Writer
type ResultsFormatter interface {
	Output(io.Writer, []Result) error
}

var supportedFormatters = map[string]ResultsFormatter{
	string(OutputFormatDefault):   &textResultsFormatter{},
	string(OutputFormatText):      &textResultsFormatter{},
	string(OutputFormatJSON):      &jsonResultsFormatter{},
	string(OutputFormatTextShell): &shellResultsFormatter{},
	string(OutputFormatTextShort): &textShortResultsFormatter{},
}

// SupportedOutputFormats returns a list of supported output formats
func SupportedOutputFormats() []string {
	return slices.Sorted(maps.Keys(supportedFormatters))
}

// NewResultsFormatter will create a formatter based on the given format.
func NewResultsFormatter(format OutputFormat) (ResultsFormatter, error) {
	rs, ok := supportedFormatters[string(format)]
	if !ok {
		return nil, fmt.Errorf("unsupported formatter %q", format)
	}
	return rs, nil
}

type textResultsFormatter struct{}

func (*textResultsFormatter) Output(w io.Writer, all []Result) error {
	var errs []error

	for _, res := range all {
		// The should be copy/paste friendly, i.e. the "image-builder"
		// cmdline should support:
		//   image-builder manifest centos-9 type:qcow2 arch:s390
		//   image-builder build centos-9 type:qcow2 arch:x86_64
		arch := res.ImgType.Arch()
		distro := arch.Distro()
		if _, err := fmt.Fprintf(w, "%s type:%s arch:%s\n", distro.Name(), res.ImgType.Name(), arch.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

type shellResultsFormatter struct{}

func (*shellResultsFormatter) Output(w io.Writer, all []Result) error {
	var errs []error

	for _, res := range all {
		arch := res.ImgType.Arch()
		distro := arch.Distro()
		if _, err := fmt.Fprintf(w, "%s --distro %s --arch %s\n",
			res.ImgType.Name(),
			distro.Name(),
			arch.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

type textShortResultsFormatter struct{}

func (*textShortResultsFormatter) Output(w io.Writer, all []Result) error {
	var errs []error

	outputMap := make(map[string]map[string][]string)
	for _, res := range all {
		arch := res.ImgType.Arch()
		distro := arch.Distro()
		if _, ok := outputMap[distro.Name()]; !ok {
			outputMap[distro.Name()] = make(map[string][]string)
		}
		outputMap[distro.Name()][res.ImgType.Name()] = append(outputMap[distro.Name()][res.ImgType.Name()], arch.Name())
	}

	// Sort and prepare output
	var distros []string
	for distro := range outputMap {
		distros = append(distros, distro)
	}
	if err := distrosort.Names(distros); err != nil {
		return fmt.Errorf("cannot sort distro names %q: %w", distros, err)
	}

	for _, distro := range distros {
		var types []string
		for t := range outputMap[distro] {
			types = append(types, t)
		}
		slices.Sort(types)

		var typeArchPairs []string
		for _, t := range types {
			arches := outputMap[distro][t]
			slices.Sort(arches)
			typeArchPairs = append(typeArchPairs, fmt.Sprintf("%s: %s", t, strings.Join(arches, ", ")))
		}

		if _, err := fmt.Fprintf(w, "%s\n  %s\n", distro, strings.Join(typeArchPairs, "\n  ")); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

type jsonResultsFormatter struct{}

type distroResultJSON struct {
	Name string `json:"name"`
}

type archResultJSON struct {
	Name string `json:"name"`
}

type imgTypeResultJSON struct {
	Name string `json:"name"`
}

type filteredResultJSON struct {
	Distro  distroResultJSON  `json:"distro"`
	Arch    archResultJSON    `json:"arch"`
	ImgType imgTypeResultJSON `json:"image_type"`
}

func (*jsonResultsFormatter) Output(w io.Writer, all []Result) error {
	var out []filteredResultJSON

	for _, res := range all {
		arch := res.ImgType.Arch()
		distro := arch.Distro()
		out = append(out, filteredResultJSON{
			Distro: distroResultJSON{
				Name: distro.Name(),
			},
			Arch: archResultJSON{
				Name: arch.Name(),
			},
			ImgType: imgTypeResultJSON{
				Name: res.ImgType.Name(),
			},
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(out)
}
