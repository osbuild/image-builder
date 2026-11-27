package osbuild

type ConcatStageOptions struct {
	Filename string `json:"filename"`

	// Input file URLs in order of concatenation.
	Files []string `json:"files"`

	// Byte alignment of each input in the output. Zero uses the osbuild default of 1 e.g
	// no alignment.
	Alignment uint64 `json:"alignment,omitempty"`
}

func (ConcatStageOptions) isStageOptions() {}

// ConcatStageInputs maps the names used in input URLs to osbuild inputs.
type ConcatStageInputs map[string]Input

func (ConcatStageInputs) isStageInputs() {}

// NewConcatStage concatenates the input files into a single output file.
func NewConcatStage(options *ConcatStageOptions, inputs ConcatStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.concat",
		Inputs:  inputs,
		Options: options,
	}
}
