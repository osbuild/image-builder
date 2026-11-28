package osbuild

type LnStageOptions struct {
	Paths []LnStagePath `json:"paths"`
}

type LnStagePath struct {
	Target   string `json:"target"`
	LinkName string `json:"link_name"`
	Symbolic bool   `json:"symbolic,omitempty"`
}

func (LnStageOptions) isStageOptions() {}

func NewLnStage(options *LnStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ln",
		Options: options,
	}
}
