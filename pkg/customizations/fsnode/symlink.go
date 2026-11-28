package fsnode

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/osbuild/image-builder/internal/common"
)

type Symlink struct {
	Path   string `json:"path" yaml:"path"`
	Target string `json:"target" yaml:"target"`
}

func NewSymlink(path, target string) (*Symlink, error) {
	link := &Symlink{Path: path, Target: target}
	if err := link.validate(); err != nil {
		return nil, err
	}
	return link, nil
}

func (s *Symlink) validate() error {
	if _, err := newBaseFsNode(s.Path, nil, nil, nil); err != nil {
		return err
	}
	if s.Target == "" {
		return fmt.Errorf("invalid symlink target %q", s.Target)
	}
	return nil
}

func (s *Symlink) UnmarshalJSON(data []byte) error {
	type symlink Symlink
	var value symlink
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*s = Symlink(value)
	return s.validate()
}

func (s *Symlink) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(s, unmarshal)
}
