package manifest

import "fmt"

type Compression string

const (
	CompressionXZ   Compression = "xz"
	CompressionZstd Compression = "zstd"
	CompressionGzip Compression = "gzip"
	CompressionNone Compression = "none"
)

type CompressionPipelineFunc func(Build, FilePipeline) FilePipeline

var CompressionPipelines = map[Compression]CompressionPipelineFunc{
	CompressionXZ:   func(b Build, p FilePipeline) FilePipeline { return NewXZ(b, p) },
	CompressionZstd: func(b Build, p FilePipeline) FilePipeline { return NewZstd(b, p) },
	CompressionGzip: func(b Build, p FilePipeline) FilePipeline { return NewGzip(b, p) },
	CompressionNone: func(_ Build, p FilePipeline) FilePipeline { return p },
}

func (c *Compression) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	v := Compression(s)
	if v == "" {
		v = CompressionNone
	}

	if _, ok := CompressionPipelines[v]; !ok {
		return fmt.Errorf("unsupported compression type %q", s)
	}

	*c = v
	return nil
}
