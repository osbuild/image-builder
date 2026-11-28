package osbuild

type KernelModulesCopyStageOptions struct {
	KernelVersion string `json:"kernel_version"`
}

func (KernelModulesCopyStageOptions) isStageOptions() {}

// NewKernelModulesCopyStage copies a kernel's modules and metadata from a pipeline tree.
func NewKernelModulesCopyStage(kernelVersion, sourcePipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.kernel-modules.copy",
		Options: &KernelModulesCopyStageOptions{KernelVersion: kernelVersion},
		Inputs:  NewPipelineTreeInputs("tree", sourcePipeline),
	}
}
