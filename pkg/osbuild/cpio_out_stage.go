package osbuild

type CPIOArchiveFormat string

const (
	CPIOArchiveFormatBin   CPIOArchiveFormat = "bin"
	CPIOArchiveFormatOdc   CPIOArchiveFormat = "odc"
	CPIOArchiveFormatNewc  CPIOArchiveFormat = "newc"
	CPIOArchiveFormatCRC   CPIOArchiveFormat = "crc"
	CPIOArchiveFormatTar   CPIOArchiveFormat = "tar"
	CPIOArchiveFormatUstar CPIOArchiveFormat = "ustar"
)

type CPIORootNode string

const (
	CPIORootNodeInclude CPIORootNode = "include"
	CPIORootNodeOmit    CPIORootNode = "omit"
)

type CPIOOwner struct {
	User  string `json:"user"`
	Group string `json:"group,omitempty"`
}

type CPIOOutStageOptions struct {
	Filename string `json:"filename"`
	Append   bool   `json:"append,omitempty"`

	// Format defaults to newc.
	Format CPIOArchiveFormat `json:"format,omitempty"`

	// RootNode defaults to include.
	RootNode CPIORootNode `json:"root-node,omitempty"`

	// Reproducible defaults to true.
	Reproducible *bool      `json:"reproducible,omitempty"`
	Owner        *CPIOOwner `json:"owner,omitempty"`
}

func (CPIOOutStageOptions) isStageOptions() {}

// NewCPIOOutStage assembles the input pipeline's tree into a CPIO archive.
func NewCPIOOutStage(options *CPIOOutStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.cpio.out",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}
