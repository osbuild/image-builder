package progress

import (
	"io"
)

type Reader struct {
	io.Reader
	pb    ProgressBar
	done  int
	total int
}

func NewProxyReader(r io.Reader, total int, pb ProgressBar) (*Reader, error) {
	err := pb.SetProgress(0, "Uploading", 0, total)
	if err != nil {
		return nil, err
	}
	return &Reader{
		Reader: r,
		pb:     pb,
		done:   0,
		total:  total,
	}, nil
}

func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.done += n
	if err != nil {
		return n, err
	}
	err = r.pb.SetProgress(0, "", r.done, r.total)
	return n, err
}

func (r *Reader) Close() error {
	if closer, ok := r.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
