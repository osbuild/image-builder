package blueprintload

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/osbuild/blueprint/pkg/blueprint"
)

// XXX: move this helper into images, share with bib
func decodeToml(r io.Reader, what string) (*blueprint.Blueprint, error) {
	dec := toml.NewDecoder(r)

	var conf blueprint.Blueprint
	metadata, err := dec.Decode(&conf)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}
	if len(metadata.Undecoded()) > 0 {
		return nil, fmt.Errorf("cannot decode %q: unknown keys found: %v", what, metadata.Undecoded())
	}

	return &conf, nil
}

func decodeJson(r io.Reader, what string) (*blueprint.Blueprint, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var conf blueprint.Blueprint
	if err := dec.Decode(&conf); err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("multiple configuration objects or extra data found in %q", what)
	}
	return &conf, nil
}

func Load(path string) (*blueprint.Blueprint, error) {
	var fp io.ReadCloser
	var err error

	switch path {
	case "":
		return &blueprint.Blueprint{}, nil
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
