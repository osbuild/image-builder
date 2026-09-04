package osbuild

type RPMMacrosStageOptions struct {
	Filename string    `json:"filename"`
	Macros   RPMMacros `json:"macros"`
}

type RPMMacros struct {
	InstallLangs []string `json:"_install_langs,omitempty"`
	DBPath       string   `json:"_dbpath,omitempty"`
}

func (RPMMacrosStageOptions) isStageOptions() {}

func NewRPMMacrosStage(options *RPMMacrosStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.rpm.macros",
		Options: options,
	}
}
