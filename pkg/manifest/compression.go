package manifest

type Compression string

const (
	CompressionXZ   Compression = "xz"
	CompressionZstd Compression = "zstd"
	CompressionGzip Compression = "gzip"
	CompressionNone Compression = "none"
)

// Keep ordering of compression types to keep manifest generation deterministic
var CompressionTypes = []Compression{CompressionGzip, CompressionXZ, CompressionZstd}

type CompressionPipelineFunc func(Build, FilePipeline) FilePipeline

var CompressionPipelines = map[Compression]CompressionPipelineFunc{
	CompressionXZ:   func(b Build, p FilePipeline) FilePipeline { return NewXZ(b, p) },
	CompressionZstd: func(b Build, p FilePipeline) FilePipeline { return NewZstd(b, p) },
	CompressionGzip: func(b Build, p FilePipeline) FilePipeline { return NewGzip(b, p) },
	CompressionNone: func(_ Build, p FilePipeline) FilePipeline { return p },
}

type CompressionConfig struct {
	Default Compression   `yaml:"default"`
	Allowed []Compression `yaml:"allowed"`
}
