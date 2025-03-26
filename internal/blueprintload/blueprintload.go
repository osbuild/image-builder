package blueprintload

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	// XXX: there should only be one importable blueprint, i.e.
	// importing the external and passing it to images, if
	// images needs its own it should not be public
	externalBlueprint "github.com/osbuild/blueprint/pkg/blueprint"
)

// XXX: move this helper into images, share with bib
func decodeToml(r io.Reader, what string) (*externalBlueprint.Blueprint, error) {
	dec := toml.NewDecoder(r)

	var conf externalBlueprint.Blueprint
	metadata, err := dec.Decode(&conf)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}
	if len(metadata.Undecoded()) > 0 {
		return nil, fmt.Errorf("cannot decode %q: unknown keys found: %v", what, metadata.Undecoded())
	}

	return &conf, nil
}

func decodeJson(r io.Reader, what string) (*externalBlueprint.Blueprint, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var conf externalBlueprint.Blueprint
	if err := dec.Decode(&conf); err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("multiple configuration objects or extra data found in %q", what)
	}
	return &conf, nil
}

func load(path string) (*externalBlueprint.Blueprint, error) {
	var fp io.ReadCloser
	var err error

	switch path {
	case "":
		return &externalBlueprint.Blueprint{}, nil
	case "-":
		fp = os.Stdin
	default:
		fp, err = os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fp.Close()
	}

	switch {
	case path == "-", filepath.Ext(path) == ".json":
		return decodeJson(fp, path)
	case filepath.Ext(path) == ".toml":
		return decodeToml(fp, path)
	default:
		return nil, fmt.Errorf("unsupported file extension for %q (please use .toml or .json)", path)
	}
}

func Load(path string) (*externalBlueprint.Blueprint, error) {
	externalBp, err := load(path)
	if err != nil {
		return nil, err
	}
	// XXX: make convert a method on "Blueprint"
	// XXX2: make Convert() take a pointer
	imagesBp := externalBlueprint.Convert(*externalBp)
	return &imagesBp, nil
}
